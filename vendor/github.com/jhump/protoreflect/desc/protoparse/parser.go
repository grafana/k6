package protoparse

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bufbuild/protocompile"
	ast2 "github.com/bufbuild/protocompile/ast"
	"github.com/bufbuild/protocompile/linker"
	"github.com/bufbuild/protocompile/options"
	"github.com/bufbuild/protocompile/parser"
	"github.com/bufbuild/protocompile/protoutil"
	"github.com/bufbuild/protocompile/reporter"
	"github.com/bufbuild/protocompile/sourceinfo"
	"github.com/bufbuild/protocompile/walk"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/internal"
	"github.com/jhump/protoreflect/desc/protoparse/ast"
)

// FileAccessor is an abstraction for opening proto source files. It takes the
// name of the file to open and returns either the input reader or an error.
type FileAccessor func(filename string) (io.ReadCloser, error)

// FileContentsFromMap returns a FileAccessor that uses the given map of file
// contents. This allows proto source files to be constructed in memory and
// easily supplied to a parser. The map keys are the paths to the proto source
// files, and the values are the actual proto source contents.
func FileContentsFromMap(files map[string]string) FileAccessor {
	return func(filename string) (io.ReadCloser, error) {
		contents, ok := files[filename]
		if !ok {
			// Try changing path separators since user-provided
			// map may use different separators.
			contents, ok = files[filepath.ToSlash(filename)]
			if !ok {
				return nil, os.ErrNotExist
			}
		}
		return ioutil.NopCloser(strings.NewReader(contents)), nil
	}
}

// Parser parses proto source into descriptors.
type Parser struct {
	// The paths used to search for dependencies that are referenced in import
	// statements in proto source files. If no import paths are provided then
	// "." (current directory) is assumed to be the only import path.
	//
	// This setting is only used during ParseFiles operations. Since calls to
	// ParseFilesButDoNotLink do not link, there is no need to load and parse
	// dependencies.
	ImportPaths []string

	// If true, the supplied file names/paths need not necessarily match how the
	// files are referenced in import statements. The parser will attempt to
	// match import statements to supplied paths, "guessing" the import paths
	// for the files. Note that this inference is not perfect and link errors
	// could result. It works best when all proto files are organized such that
	// a single import path can be inferred (e.g. all files under a single tree
	// with import statements all being relative to the root of this tree).
	InferImportPaths bool

	// LookupImport is a function that accepts a filename and
	// returns a file descriptor, which will be consulted when resolving imports.
	// This allows a compiled Go proto in another Go module to be referenced
	// in the proto(s) being parsed.
	//
	// In the event of a filename collision, Accessor is consulted first,
	// then LookupImport is consulted, and finally the well-known protos
	// are used.
	//
	// For example, in order to automatically look up compiled Go protos that
	// have been imported and be able to use them as imports, set this to
	// desc.LoadFileDescriptor.
	LookupImport func(string) (*desc.FileDescriptor, error)

	// LookupImportProto has the same functionality as LookupImport, however it returns
	// a FileDescriptorProto instead of a FileDescriptor.
	LookupImportProto func(string) (*descriptorpb.FileDescriptorProto, error)

	// Used to create a reader for a given filename, when loading proto source
	// file contents. If unset, os.Open is used. If ImportPaths is also empty
	// then relative paths are will be relative to the process's current working
	// directory.
	Accessor FileAccessor

	// If true, the resulting file descriptors will retain source code info,
	// that maps elements to their location in the source files as well as
	// includes comments found during parsing (and attributed to elements of
	// the source file).
	IncludeSourceCodeInfo bool

	// If true, the results from ParseFilesButDoNotLink will be passed through
	// some additional validations. But only constraints that do not require
	// linking can be checked. These include proto2 vs. proto3 language features,
	// looking for incorrect usage of reserved names or tags, and ensuring that
	// fields have unique tags and that enum values have unique numbers (unless
	// the enum allows aliases).
	ValidateUnlinkedFiles bool

	// If true, the results from ParseFilesButDoNotLink will have options
	// interpreted. Any uninterpretable options (including any custom options or
	// options that refer to message and enum types, which can only be
	// interpreted after linking) will be left in uninterpreted_options. Also,
	// the "default" pseudo-option for fields can only be interpreted for scalar
	// fields, excluding enums. (Interpreting default values for enum fields
	// requires resolving enum names, which requires linking.)
	InterpretOptionsInUnlinkedFiles bool

	// A custom reporter of syntax and link errors. If not specified, the
	// default reporter just returns the reported error, which causes parsing
	// to abort after encountering a single error.
	//
	// The reporter is not invoked for system or I/O errors, only for syntax and
	// link errors.
	ErrorReporter ErrorReporter

	// A custom reporter of warnings. If not specified, warning messages are ignored.
	WarningReporter WarningReporter
}

// ParseFiles parses the named files into descriptors. The returned slice has
// the same number of entries as the give filenames, in the same order. So the
// first returned descriptor corresponds to the first given name, and so on.
//
// All dependencies for all specified files (including transitive dependencies)
// must be accessible via the parser's Accessor or a link error will occur. The
// exception to this rule is that files can import standard Google-provided
// files -- e.g. google/protobuf/*.proto -- without needing to supply sources
// for these files. Like protoc, this parser has a built-in version of these
// files it can use if they aren't explicitly supplied.
//
// If the Parser has no ErrorReporter set and a syntax or link error occurs,
// parsing will abort with the first such error encountered. If there is an
// ErrorReporter configured and it returns non-nil, parsing will abort with the
// error it returns. If syntax or link errors are encountered but the configured
// ErrorReporter always returns nil, the parse fails with ErrInvalidSource.
func (p Parser) ParseFiles(filenames ...string) ([]*desc.FileDescriptor, error) {
	srcInfoMode := protocompile.SourceInfoNone
	if p.IncludeSourceCodeInfo {
		srcInfoMode = protocompile.SourceInfoExtraComments
	}
	rep := newReporter(p.ErrorReporter, p.WarningReporter)
	res, srcSpanAddr := p.getResolver(filenames)

	if p.InferImportPaths {
		// we must first compile everything to protos
		results, err := parseToProtosRecursive(res, filenames, reporter.NewHandler(rep), srcSpanAddr)
		if err != nil {
			return nil, err
		}
		// then we can infer import paths
		var rewritten map[string]string
		results, rewritten = fixupFilenames(results)
		if len(rewritten) > 0 {
			for i := range filenames {
				if replace, ok := rewritten[filenames[i]]; ok {
					filenames[i] = replace
				}
			}
		}
		resolverFromResults := protocompile.ResolverFunc(func(path string) (protocompile.SearchResult, error) {
			res, ok := results[path]
			if !ok {
				return protocompile.SearchResult{}, os.ErrNotExist
			}
			return protocompile.SearchResult{ParseResult: noCloneParseResult{res}}, nil
		})
		res = protocompile.CompositeResolver{resolverFromResults, res}
	}

	c := protocompile.Compiler{
		Resolver:       res,
		MaxParallelism: 1,
		SourceInfoMode: srcInfoMode,
		Reporter:       rep,
	}
	results, err := c.Compile(context.Background(), filenames...)
	if err != nil {
		return nil, err
	}

	fds := make([]protoreflect.FileDescriptor, len(results))
	alreadySeen := make(map[string]struct{}, len(results))
	for i, res := range results {
		removeDynamicExtensions(res, alreadySeen)
		fds[i] = res
	}
	return desc.WrapFiles(fds)
}

type noCloneParseResult struct {
	parser.Result
}

func (r noCloneParseResult) Clone() parser.Result {
	// protocompile will clone parser.Result to make sure it can't be shared
	// with other compilation operations (which would not be thread-safe).
	// However, this parse result cannot be shared with another compile
	// operation. That means the clone is unnecessary; so we skip it, to avoid
	// the associated performance costs.
	return r.Result
}

// ParseFilesButDoNotLink parses the named files into descriptor protos. The
// results are just protos, not fully-linked descriptors. It is possible that
// descriptors are invalid and still be returned in parsed form without error
// due to the fact that the linking step is skipped (and thus many validation
// steps omitted).
//
// There are a few side effects to not linking the descriptors:
//  1. No options will be interpreted. Options can refer to extensions or have
//     message and enum types. Without linking, these extension and type
//     references are not resolved, so the options may not be interpretable.
//     So all options will appear in UninterpretedOption fields of the various
//     descriptor options messages.
//  2. Type references will not be resolved. This means that the actual type
//     names in the descriptors may be unqualified and even relative to the
//     scope in which the type reference appears. This goes for fields that
//     have message and enum types. It also applies to methods and their
//     references to request and response message types.
//  3. Type references are not known. For non-scalar fields, until the type
//     name is resolved (during linking), it is not known whether the type
//     refers to a message or an enum. So all fields with such type references
//     will not have their Type set, only the TypeName.
//
// This method will still validate the syntax of parsed files. If the parser's
// ValidateUnlinkedFiles field is true, additional checks, beyond syntax will
// also be performed.
//
// If the Parser has no ErrorReporter set and a syntax error occurs, parsing
// will abort with the first such error encountered. If there is an
// ErrorReporter configured and it returns non-nil, parsing will abort with the
// error it returns. If syntax errors are encountered but the configured
// ErrorReporter always returns nil, the parse fails with ErrInvalidSource.
func (p Parser) ParseFilesButDoNotLink(filenames ...string) ([]*descriptorpb.FileDescriptorProto, error) {
	rep := newReporter(p.ErrorReporter, p.WarningReporter)
	p.ImportPaths = nil // not used for this "do not link" operation.
	res, _ := p.getResolver(filenames)
	results, err := parseToProtos(res, filenames, reporter.NewHandler(rep), p.ValidateUnlinkedFiles)
	if err != nil {
		return nil, err
	}

	if p.InferImportPaths {
		resultsMap := make(map[string]parser.Result, len(results))
		for _, res := range results {
			resultsMap[res.FileDescriptorProto().GetName()] = res
		}
		var rewritten map[string]string
		resultsMap, rewritten = fixupFilenames(resultsMap)
		if len(rewritten) > 0 {
			for i := range filenames {
				if replace, ok := rewritten[filenames[i]]; ok {
					filenames[i] = replace
				}
			}
		}
		for i := range filenames {
			results[i] = resultsMap[filenames[i]]
		}
	}

	protos := make([]*descriptorpb.FileDescriptorProto, len(results))
	for i, res := range results {
		protos[i] = res.FileDescriptorProto()
		var optsIndex sourceinfo.OptionIndex
		if p.InterpretOptionsInUnlinkedFiles {
			var err error
			optsIndex, err = options.InterpretUnlinkedOptions(res)
			if err != nil {
				return nil, err
			}
			removeDynamicExtensionsFromProto(protos[i])
		}
		if p.IncludeSourceCodeInfo {
			protos[i].SourceCodeInfo = sourceinfo.GenerateSourceInfo(res.AST(), optsIndex, sourceinfo.WithExtraComments())
		}
	}

	return protos, nil
}

// ParseToAST parses the named files into ASTs, or Abstract Syntax Trees. This
// is for consumers of proto files that don't care about compiling the files to
// descriptors, but care deeply about a non-lossy structured representation of
// the source (since descriptors are lossy). This includes formatting tools and
// possibly linters, too.
//
// If the requested filenames include standard imports (such as
// "google/protobuf/empty.proto") and no source is provided, the corresponding
// AST in the returned slice will be nil. These standard imports are only
// available for use as descriptors; no source is available unless it is
// provided by the configured Accessor.
//
// If the Parser has no ErrorReporter set and a syntax error occurs, parsing
// will abort with the first such error encountered. If there is an
// ErrorReporter configured and it returns non-nil, parsing will abort with the
// error it returns. If syntax errors are encountered but the configured
// ErrorReporter always returns nil, the parse fails with ErrInvalidSource.
func (p Parser) ParseToAST(filenames ...string) ([]*ast.FileNode, error) {
	rep := newReporter(p.ErrorReporter, p.WarningReporter)
	res, _ := p.getResolver(filenames)
	asts, _, err := parseToASTs(res, filenames, reporter.NewHandler(rep))
	if err != nil {
		return nil, err
	}
	results := make([]*ast.FileNode, len(asts))
	for i := range asts {
		if asts[i] == nil {
			// should not be possible but...
			return nil, fmt.Errorf("resolver did not produce source for %v", filenames[i])
		}
		results[i] = convertAST(asts[i])
	}
	return results, nil
}

func parseToAST(res protocompile.Resolver, filename string, rep *reporter.Handler) (*ast2.FileNode, parser.Result, error) {
	searchResult, err := res.FindFileByPath(filename)
	if err != nil {
		_ = rep.HandleError(err)
		return nil, nil, rep.Error()
	}
	switch {
	case searchResult.ParseResult != nil:
		return nil, searchResult.ParseResult, nil
	case searchResult.Proto != nil:
		return nil, parser.ResultWithoutAST(searchResult.Proto), nil
	case searchResult.Desc != nil:
		return nil, parser.ResultWithoutAST(protoutil.ProtoFromFileDescriptor(searchResult.Desc)), nil
	case searchResult.AST != nil:
		return searchResult.AST, nil, nil
	case searchResult.Source != nil:
		astRoot, err := parser.Parse(filename, searchResult.Source, rep)
		return astRoot, nil, err
	default:
		_ = rep.HandleError(fmt.Errorf("resolver did not produce a result for %v", filename))
		return nil, nil, rep.Error()
	}
}

func parseToASTs(res protocompile.Resolver, filenames []string, rep *reporter.Handler) ([]*ast2.FileNode, []parser.Result, error) {
	asts := make([]*ast2.FileNode, len(filenames))
	results := make([]parser.Result, len(filenames))
	for i := range filenames {
		asts[i], results[i], _ = parseToAST(res, filenames[i], rep)
		if rep.ReporterError() != nil {
			break
		}
	}
	return asts, results, rep.Error()
}

func parseToProtos(res protocompile.Resolver, filenames []string, rep *reporter.Handler, validate bool) ([]parser.Result, error) {
	asts, results, err := parseToASTs(res, filenames, rep)
	if err != nil {
		return nil, err
	}
	for i := range results {
		if results[i] != nil {
			continue
		}
		var err error
		results[i], err = parser.ResultFromAST(asts[i], validate, rep)
		if err != nil {
			return nil, err
		}
	}
	return results, nil
}

func parseToProtosRecursive(res protocompile.Resolver, filenames []string, rep *reporter.Handler, srcSpanAddr *ast2.SourceSpan) (map[string]parser.Result, error) {
	results := make(map[string]parser.Result, len(filenames))
	for _, filename := range filenames {
		if err := parseToProtoRecursive(res, filename, rep, srcSpanAddr, results); err != nil {
			return results, err
		}
	}
	return results, rep.Error()
}

func parseToProtoRecursive(res protocompile.Resolver, filename string, rep *reporter.Handler, srcSpanAddr *ast2.SourceSpan, results map[string]parser.Result) error {
	if _, ok := results[filename]; ok {
		// already processed this one
		return nil
	}
	results[filename] = nil // placeholder entry

	astRoot, parseResult, err := parseToAST(res, filename, rep)
	if err != nil {
		return err
	}
	if parseResult == nil {
		parseResult, err = parser.ResultFromAST(astRoot, true, rep)
		if err != nil {
			return err
		}
	}
	results[filename] = parseResult

	if astRoot != nil {
		// We have an AST, so we use it to recursively examine imports.
		for _, decl := range astRoot.Decls {
			imp, ok := decl.(*ast2.ImportNode)
			if !ok {
				continue
			}
			err := func() error {
				orig := *srcSpanAddr
				*srcSpanAddr = astRoot.NodeInfo(imp.Name)
				defer func() {
					*srcSpanAddr = orig
				}()

				return parseToProtoRecursive(res, imp.Name.AsString(), rep, srcSpanAddr, results)
			}()
			if err != nil {
				return err
			}
		}
		return nil
	}

	// Without an AST, we must recursively examine the proto. This makes it harder
	// (but not necessarily impossible) to get the source location of the import.
	fd := parseResult.FileDescriptorProto()
	for i, dep := range fd.Dependency {
		path := []int32{internal.File_dependencyTag, int32(i)}
		err := func() error {
			orig := *srcSpanAddr
			found := false
			for _, loc := range fd.GetSourceCodeInfo().GetLocation() {
				if pathsEqual(loc.Path, path) {
					start := SourcePos{
						Filename: dep,
						Line:     int(loc.Span[0]),
						Col:      int(loc.Span[1]),
					}
					var end SourcePos
					if len(loc.Span) > 3 {
						end = SourcePos{
							Filename: dep,
							Line:     int(loc.Span[2]),
							Col:      int(loc.Span[3]),
						}
					} else {
						end = SourcePos{
							Filename: dep,
							Line:     int(loc.Span[0]),
							Col:      int(loc.Span[2]),
						}
					}
					*srcSpanAddr = ast2.NewSourceSpan(start, end)
					found = true
					break
				}
			}
			if !found {
				*srcSpanAddr = ast2.UnknownSpan(dep)
			}
			defer func() {
				*srcSpanAddr = orig
			}()

			return parseToProtoRecursive(res, dep, rep, srcSpanAddr, results)
		}()
		if err != nil {
			return err
		}
	}
	return nil
}

func pathsEqual(a, b []int32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func newReporter(errRep ErrorReporter, warnRep WarningReporter) reporter.Reporter {
	if errRep != nil {
		delegate := errRep
		errRep = func(err ErrorWithPos) error {
			if _, ok := err.(ErrorWithSourcePos); !ok {
				err = toErrorWithSourcePos(err)
			}
			return delegate(err)
		}
	}
	if warnRep != nil {
		delegate := warnRep
		warnRep = func(err ErrorWithPos) {
			if _, ok := err.(ErrorWithSourcePos); !ok {
				err = toErrorWithSourcePos(err)
			}
			delegate(err)
		}
	}
	return reporter.NewReporter(errRep, warnRep)
}

func (p Parser) getResolver(filenames []string) (protocompile.Resolver, *ast2.SourceSpan) {
	var srcSpan ast2.SourceSpan
	accessor := p.Accessor
	if accessor == nil {
		accessor = func(name string) (io.ReadCloser, error) {
			return os.Open(name)
		}
	}
	sourceResolver := &protocompile.SourceResolver{
		Accessor: func(filename string) (io.ReadCloser, error) {
			in, err := accessor(filename)
			if err != nil {
				if !strings.Contains(err.Error(), filename) {
					// errors that don't include the filename that failed are no bueno
					err = errorWithFilename{filename: filename, underlying: err}
				}
				if srcSpan != nil {
					err = reporter.Error(srcSpan, err)
				}
			}
			return in, err
		},
		ImportPaths: p.ImportPaths,
	}
	var importResolver protocompile.CompositeResolver
	if p.LookupImport != nil {
		importResolver = append(importResolver, protocompile.ResolverFunc(func(path string) (protocompile.SearchResult, error) {
			fd, err := p.LookupImport(path)
			if err != nil {
				return protocompile.SearchResult{}, err
			}
			return protocompile.SearchResult{Desc: fd.UnwrapFile()}, nil
		}))
	}
	if p.LookupImportProto != nil {
		importResolver = append(importResolver, protocompile.ResolverFunc(func(path string) (protocompile.SearchResult, error) {
			fd, err := p.LookupImportProto(path)
			if err != nil {
				return protocompile.SearchResult{}, err
			}
			return protocompile.SearchResult{Proto: fd}, nil
		}))
	}
	backupResolver := protocompile.WithStandardImports(importResolver)
	return protocompile.CompositeResolver{
		sourceResolver,
		protocompile.ResolverFunc(func(path string) (protocompile.SearchResult, error) {
			return backupResolver.FindFileByPath(path)
		}),
	}, &srcSpan
}

func fixupFilenames(protos map[string]parser.Result) (revisedProtos map[string]parser.Result, rewrittenPaths map[string]string) {
	// In the event that the given filenames (keys in the supplied map) do not
	// match the actual paths used in 'import' statements in the files, we try
	// to revise names in the protos so that they will match and be linkable.
	revisedProtos = make(map[string]parser.Result, len(protos))
	rewrittenPaths = make(map[string]string, len(protos))

	protoPaths := map[string]struct{}{}
	// TODO: this is O(n^2) but could likely be O(n) with a clever data structure (prefix tree that is indexed backwards?)
	importCandidates := map[string]map[string]struct{}{}
	candidatesAvailable := map[string]struct{}{}
	for name := range protos {
		candidatesAvailable[name] = struct{}{}
		for _, f := range protos {
			for _, imp := range f.FileDescriptorProto().Dependency {
				if strings.HasSuffix(name, imp) || strings.HasSuffix(imp, name) {
					candidates := importCandidates[imp]
					if candidates == nil {
						candidates = map[string]struct{}{}
						importCandidates[imp] = candidates
					}
					candidates[name] = struct{}{}
				}
			}
		}
	}
	for imp, candidates := range importCandidates {
		// if we found multiple possible candidates, use the one that is an exact match
		// if it exists, and otherwise, guess that it's the shortest path (fewest elements)
		var best string
		for c := range candidates {
			if _, ok := candidatesAvailable[c]; !ok {
				// already used this candidate and re-written its filename accordingly
				continue
			}
			if c == imp {
				// exact match!
				best = c
				break
			}
			if best == "" {
				best = c
			} else {
				// NB: We can't actually tell which file is supposed to match
				// this import. So we prefer the longest name. On a tie, we
				// choose the lexically earliest match.
				minLen := strings.Count(best, string(filepath.Separator))
				cLen := strings.Count(c, string(filepath.Separator))
				if cLen > minLen || (cLen == minLen && c < best) {
					best = c
				}
			}
		}
		if best != "" {
			if len(best) > len(imp) {
				prefix := best[:len(best)-len(imp)]
				protoPaths[prefix] = struct{}{}
			}
			f := protos[best]
			f.FileDescriptorProto().Name = proto.String(imp)
			revisedProtos[imp] = f
			rewrittenPaths[best] = imp
			delete(candidatesAvailable, best)

			// If other candidates are actually references to the same file, remove them.
			for c := range candidates {
				if _, ok := candidatesAvailable[c]; !ok {
					// already used this candidate and re-written its filename accordingly
					continue
				}
				possibleDup := protos[c]
				prevName := possibleDup.FileDescriptorProto().Name
				possibleDup.FileDescriptorProto().Name = proto.String(imp)
				if !proto.Equal(f.FileDescriptorProto(), protos[c].FileDescriptorProto()) {
					// not equal: restore name and look at next one
					possibleDup.FileDescriptorProto().Name = prevName
					continue
				}
				// This file used a different name but was actually the same file. So
				// we prune it from the set.
				rewrittenPaths[c] = imp
				delete(candidatesAvailable, c)
				if len(c) > len(imp) {
					prefix := c[:len(c)-len(imp)]
					protoPaths[prefix] = struct{}{}
				}
			}
		}
	}

	if len(candidatesAvailable) == 0 {
		return revisedProtos, rewrittenPaths
	}

	if len(protoPaths) == 0 {
		for c := range candidatesAvailable {
			revisedProtos[c] = protos[c]
		}
		return revisedProtos, rewrittenPaths
	}

	// Any remaining candidates are entry-points (not imported by others), so
	// the best bet to "fixing" their file name is to see if they're in one of
	// the proto paths we found, and if so strip that prefix.
	protoPathStrs := make([]string, len(protoPaths))
	i := 0
	for p := range protoPaths {
		protoPathStrs[i] = p
		i++
	}
	sort.Strings(protoPathStrs)
	// we look at paths in reverse order, so we'll use a longer proto path if
	// there is more than one match
	for c := range candidatesAvailable {
		var imp string
		for i := len(protoPathStrs) - 1; i >= 0; i-- {
			p := protoPathStrs[i]
			if strings.HasPrefix(c, p) {
				imp = c[len(p):]
				break
			}
		}
		if imp != "" {
			f := protos[c]
			f.FileDescriptorProto().Name = proto.String(imp)
			f.FileNode()
			revisedProtos[imp] = f
			rewrittenPaths[c] = imp
		} else {
			revisedProtos[c] = protos[c]
		}
	}

	return revisedProtos, rewrittenPaths
}

func removeDynamicExtensions(fd protoreflect.FileDescriptor, alreadySeen map[string]struct{}) {
	if _, ok := alreadySeen[fd.Path()]; ok {
		// already processed
		return
	}
	alreadySeen[fd.Path()] = struct{}{}
	res, ok := fd.(linker.Result)
	if ok {
		removeDynamicExtensionsFromProto(res.FileDescriptorProto())
	}
	// also remove extensions from dependencies
	for i, length := 0, fd.Imports().Len(); i < length; i++ {
		removeDynamicExtensions(fd.Imports().Get(i).FileDescriptor, alreadySeen)
	}
}

func removeDynamicExtensionsFromProto(fd *descriptorpb.FileDescriptorProto) {
	// protocompile returns descriptors with dynamic extension fields for custom options.
	// But protoparse only used known custom options and everything else defined in the
	// sources would be stored as unrecognized fields. So to bridge the difference in
	// behavior, we need to remove custom options from the given file and add them back
	// via serializing-then-de-serializing them back into the options messages. That way,
	// statically known options will be properly typed and others will be unrecognized.
	//
	// This is best effort. So if an error occurs, we'll still return a result, but it
	// may include a dynamic extension.
	fd.Options = removeDynamicExtensionsFromOptions(fd.Options)
	_ = walk.DescriptorProtos(fd, func(_ protoreflect.FullName, msg proto.Message) error {
		switch msg := msg.(type) {
		case *descriptorpb.DescriptorProto:
			msg.Options = removeDynamicExtensionsFromOptions(msg.Options)
			for _, extr := range msg.ExtensionRange {
				extr.Options = removeDynamicExtensionsFromOptions(extr.Options)
			}
		case *descriptorpb.FieldDescriptorProto:
			msg.Options = removeDynamicExtensionsFromOptions(msg.Options)
		case *descriptorpb.OneofDescriptorProto:
			msg.Options = removeDynamicExtensionsFromOptions(msg.Options)
		case *descriptorpb.EnumDescriptorProto:
			msg.Options = removeDynamicExtensionsFromOptions(msg.Options)
		case *descriptorpb.EnumValueDescriptorProto:
			msg.Options = removeDynamicExtensionsFromOptions(msg.Options)
		case *descriptorpb.ServiceDescriptorProto:
			msg.Options = removeDynamicExtensionsFromOptions(msg.Options)
		case *descriptorpb.MethodDescriptorProto:
			msg.Options = removeDynamicExtensionsFromOptions(msg.Options)
		}
		return nil
	})
}

type ptrMsg[T any] interface {
	*T
	proto.Message
}

type fieldValue struct {
	fd  protoreflect.FieldDescriptor
	val protoreflect.Value
}

func removeDynamicExtensionsFromOptions[O ptrMsg[T], T any](opts O) O {
	if opts == nil {
		return nil
	}
	var dynamicExtensions []fieldValue
	opts.ProtoReflect().Range(func(fd protoreflect.FieldDescriptor, val protoreflect.Value) bool {
		if fd.IsExtension() {
			dynamicExtensions = append(dynamicExtensions, fieldValue{fd: fd, val: val})
		}
		return true
	})

	// serialize only these custom options
	optsWithOnlyDyn := opts.ProtoReflect().Type().New()
	for _, fv := range dynamicExtensions {
		optsWithOnlyDyn.Set(fv.fd, fv.val)
	}
	data, err := proto.MarshalOptions{AllowPartial: true}.Marshal(optsWithOnlyDyn.Interface())
	if err != nil {
		// oh, well... can't fix this one
		return opts
	}

	// and then replace values by clearing these custom options and deserializing
	optsClone := proto.Clone(opts).ProtoReflect()
	for _, fv := range dynamicExtensions {
		optsClone.Clear(fv.fd)
	}
	err = proto.UnmarshalOptions{AllowPartial: true, Merge: true}.Unmarshal(data, optsClone.Interface())
	if err != nil {
		// bummer, can't fix this one
		return opts
	}

	return optsClone.Interface().(O)
}
