// Copyright 2020-2022 Buf Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package linker

import (
	"strings"
	"sync"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/bufbuild/protocompile/ast"
	"github.com/bufbuild/protocompile/internal"
	"github.com/bufbuild/protocompile/reporter"
	"github.com/bufbuild/protocompile/walk"
)

const unknownFilePath = "<unknown file>"

// Symbols is a symbol table that maps names for all program elements to their
// location in source. It also tracks extension tag numbers. This can be used
// to enforce uniqueness for symbol names and tag numbers across many files and
// many link operations.
//
// This type is thread-safe.
type Symbols struct {
	pkgTrie packageSymbols
}

type packageSymbols struct {
	mu       sync.RWMutex
	children map[protoreflect.FullName]*packageSymbols
	files    map[protoreflect.FileDescriptor]struct{}
	symbols  map[protoreflect.FullName]symbolEntry
	exts     map[extNumber]ast.SourcePos
}

type extNumber struct {
	extendee protoreflect.FullName
	tag      protoreflect.FieldNumber
}

type symbolEntry struct {
	pos         ast.SourcePos
	isEnumValue bool
	isPackage   bool
}

// Import populates the symbol table with all symbols/elements and extension
// tags present in the given file descriptor. If s is nil or if fd has already
// been imported into s, this returns immediately without doing anything. If any
// collisions in symbol names or extension tags are identified, an error will be
// returned and the symbol table will not be updated.
func (s *Symbols) Import(fd protoreflect.FileDescriptor, handler *reporter.Handler) error {
	if s == nil {
		return nil
	}

	if f, ok := fd.(*file); ok {
		// unwrap any file instance
		fd = f.FileDescriptor
	}

	var pkgPos ast.SourcePos
	if res, ok := fd.(*result); ok {
		pkgPos = packageNameStart(res)
	} else {
		pkgPos = sourcePositionForPackage(fd)
	}
	pkg, err := s.importPackages(pkgPos, fd.Package(), handler)
	if err != nil || pkg == nil {
		return err
	}

	pkg.mu.RLock()
	_, alreadyImported := pkg.files[fd]
	pkg.mu.RUnlock()

	if alreadyImported {
		return nil
	}

	for i := 0; i < fd.Imports().Len(); i++ {
		if err := s.Import(fd.Imports().Get(i).FileDescriptor, handler); err != nil {
			return err
		}
	}

	if res, ok := fd.(*result); ok && res.hasSource() {
		return s.importResultWithExtensions(pkg, res, handler)
	}

	return s.importFileWithExtensions(pkg, fd, handler)
}

func (s *Symbols) importFileWithExtensions(pkg *packageSymbols, fd protoreflect.FileDescriptor, handler *reporter.Handler) error {
	imported, err := pkg.importFile(fd, handler)
	if err != nil {
		return err
	}
	if !imported {
		// nothing else to do
		return nil
	}

	return walk.Descriptors(fd, func(d protoreflect.Descriptor) error {
		fld, ok := d.(protoreflect.FieldDescriptor)
		if !ok || !fld.IsExtension() {
			return nil
		}
		pos := sourcePositionForNumber(fld)
		extendee := fld.ContainingMessage()
		if err := s.AddExtension(packageFor(extendee), extendee.FullName(), fld.Number(), pos, handler); err != nil {
			return err
		}
		return nil
	})
}

func (s *packageSymbols) importFile(fd protoreflect.FileDescriptor, handler *reporter.Handler) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.files[fd]; ok {
		// have to double-check if it's already imported, in case
		// it was added after above read-locked check
		return false, nil
	}

	// first pass: check for conflicts
	if err := s.checkFileLocked(fd, handler); err != nil {
		return false, err
	}
	if err := handler.Error(); err != nil {
		return false, err
	}

	// second pass: commit all symbols
	s.commitFileLocked(fd)

	return true, nil
}

func (s *Symbols) importPackages(pkgPos ast.SourcePos, pkg protoreflect.FullName, handler *reporter.Handler) (*packageSymbols, error) {
	if pkg == "" {
		return &s.pkgTrie, nil
	}

	parts := strings.Split(string(pkg), ".")
	for i := 1; i < len(parts); i++ {
		parts[i] = parts[i-1] + "." + parts[i]
	}

	cur := &s.pkgTrie
	for _, p := range parts {
		var err error
		cur, err = cur.importPackage(pkgPos, protoreflect.FullName(p), handler)
		if err != nil {
			return nil, err
		}
		if cur == nil {
			return nil, nil
		}
	}

	return cur, nil
}

func (s *packageSymbols) importPackage(pkgPos ast.SourcePos, pkg protoreflect.FullName, handler *reporter.Handler) (*packageSymbols, error) {
	s.mu.RLock()
	existing, ok := s.symbols[pkg]
	var child *packageSymbols
	if ok && existing.isPackage {
		child = s.children[pkg]
	}
	s.mu.RUnlock()

	if ok && existing.isPackage {
		// package already exists
		return child, nil
	} else if ok {
		return nil, reportSymbolCollision(pkgPos, pkg, false, existing, handler)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	// have to double-check in case it was added while upgrading to write lock
	existing, ok = s.symbols[pkg]
	if ok && existing.isPackage {
		// package already exists
		return s.children[pkg], nil
	} else if ok {
		return nil, reportSymbolCollision(pkgPos, pkg, false, existing, handler)
	}
	if s.symbols == nil {
		s.symbols = map[protoreflect.FullName]symbolEntry{}
	}
	s.symbols[pkg] = symbolEntry{pos: pkgPos, isPackage: true}
	child = &packageSymbols{}
	if s.children == nil {
		s.children = map[protoreflect.FullName]*packageSymbols{}
	}
	s.children[pkg] = child
	return child, nil
}

func (s *Symbols) getPackage(pkg protoreflect.FullName) *packageSymbols {
	if pkg == "" {
		return &s.pkgTrie
	}

	parts := strings.Split(string(pkg), ".")
	for i := 1; i < len(parts); i++ {
		parts[i] = parts[i-1] + "." + parts[i]
	}

	cur := &s.pkgTrie
	for _, p := range parts {
		cur.mu.RLock()
		next := cur.children[protoreflect.FullName(p)]
		cur.mu.RUnlock()

		if next == nil {
			return nil
		}
		cur = next
	}

	return cur
}

func reportSymbolCollision(pos ast.SourcePos, fqn protoreflect.FullName, additionIsEnumVal bool, existing symbolEntry, handler *reporter.Handler) error {
	// because of weird scoping for enum values, provide more context in error message
	// if this conflict is with an enum value
	var isPkg, suffix string
	if additionIsEnumVal || existing.isEnumValue {
		suffix = "; protobuf uses C++ scoping rules for enum values, so they exist in the scope enclosing the enum"
	}
	if existing.isPackage {
		isPkg = " as a package"
	}
	orig := existing.pos
	conflict := pos
	if posLess(conflict, orig) {
		orig, conflict = conflict, orig
	}
	return handler.HandleErrorf(conflict, "symbol %q already defined%s at %v%s", fqn, isPkg, orig, suffix)
}

func posLess(a, b ast.SourcePos) bool {
	if a.Filename == b.Filename {
		if a.Line == b.Line {
			return a.Col < b.Col
		}
		return a.Line < b.Line
	}
	return false
}

func (s *packageSymbols) checkFileLocked(f protoreflect.FileDescriptor, handler *reporter.Handler) error {
	return walk.Descriptors(f, func(d protoreflect.Descriptor) error {
		pos := sourcePositionFor(d)
		if existing, ok := s.symbols[d.FullName()]; ok {
			_, isEnumVal := d.(protoreflect.EnumValueDescriptor)
			if err := reportSymbolCollision(pos, d.FullName(), isEnumVal, existing, handler); err != nil {
				return err
			}
		}
		return nil
	})
}

func sourcePositionForPackage(fd protoreflect.FileDescriptor) ast.SourcePos {
	loc := fd.SourceLocations().ByPath([]int32{internal.FilePackageTag})
	if isZeroLoc(loc) {
		return ast.UnknownPos(fd.Path())
	}
	return ast.SourcePos{
		Filename: fd.Path(),
		Line:     loc.StartLine,
		Col:      loc.StartColumn,
	}
}

func sourcePositionFor(d protoreflect.Descriptor) ast.SourcePos {
	file := d.ParentFile()
	if file == nil {
		return ast.UnknownPos(unknownFilePath)
	}
	path, ok := computePath(d)
	if !ok {
		return ast.UnknownPos(file.Path())
	}
	namePath := path
	switch d.(type) {
	case protoreflect.FieldDescriptor:
		namePath = append(namePath, internal.FieldNameTag)
	case protoreflect.MessageDescriptor:
		namePath = append(namePath, internal.MessageNameTag)
	case protoreflect.OneofDescriptor:
		namePath = append(namePath, internal.OneOfNameTag)
	case protoreflect.EnumDescriptor:
		namePath = append(namePath, internal.EnumNameTag)
	case protoreflect.EnumValueDescriptor:
		namePath = append(namePath, internal.EnumValNameTag)
	case protoreflect.ServiceDescriptor:
		namePath = append(namePath, internal.ServiceNameTag)
	case protoreflect.MethodDescriptor:
		namePath = append(namePath, internal.MethodNameTag)
	default:
		// NB: shouldn't really happen, but just in case fall back to path to
		// descriptor, sans name field
	}
	loc := file.SourceLocations().ByPath(namePath)
	if isZeroLoc(loc) {
		loc = file.SourceLocations().ByPath(path)
		if isZeroLoc(loc) {
			return ast.UnknownPos(file.Path())
		}
	}
	return ast.SourcePos{
		Filename: file.Path(),
		Line:     loc.StartLine,
		Col:      loc.StartColumn,
	}
}

func sourcePositionForNumber(fd protoreflect.FieldDescriptor) ast.SourcePos {
	file := fd.ParentFile()
	if file == nil {
		return ast.UnknownPos(unknownFilePath)
	}
	path, ok := computePath(fd)
	if !ok {
		return ast.UnknownPos(file.Path())
	}
	numberPath := path
	numberPath = append(numberPath, internal.FieldNumberTag)
	loc := file.SourceLocations().ByPath(numberPath)
	if isZeroLoc(loc) {
		loc = file.SourceLocations().ByPath(path)
		if isZeroLoc(loc) {
			return ast.UnknownPos(file.Path())
		}
	}
	return ast.SourcePos{
		Filename: file.Path(),
		Line:     loc.StartLine,
		Col:      loc.StartColumn,
	}
}

func isZeroLoc(loc protoreflect.SourceLocation) bool {
	return loc.Path == nil &&
		loc.StartLine == 0 &&
		loc.StartColumn == 0 &&
		loc.EndLine == 0 &&
		loc.EndColumn == 0
}

func (s *packageSymbols) commitFileLocked(f protoreflect.FileDescriptor) {
	if s.symbols == nil {
		s.symbols = map[protoreflect.FullName]symbolEntry{}
	}
	if s.exts == nil {
		s.exts = map[extNumber]ast.SourcePos{}
	}
	_ = walk.Descriptors(f, func(d protoreflect.Descriptor) error {
		pos := sourcePositionFor(d)
		name := d.FullName()
		_, isEnumValue := d.(protoreflect.EnumValueDescriptor)
		s.symbols[name] = symbolEntry{pos: pos, isEnumValue: isEnumValue}
		return nil
	})

	if s.files == nil {
		s.files = map[protoreflect.FileDescriptor]struct{}{}
	}
	s.files[f] = struct{}{}
}

func (s *Symbols) importResultWithExtensions(pkg *packageSymbols, r *result, handler *reporter.Handler) error {
	imported, err := pkg.importResult(r, handler)
	if err != nil {
		return err
	}
	if !imported {
		// nothing else to do
		return nil
	}

	return walk.Descriptors(r, func(d protoreflect.Descriptor) error {
		fd, ok := d.(*extTypeDescriptor)
		if !ok {
			return nil
		}
		file := r.FileNode()
		node := r.FieldNode(fd.FieldDescriptorProto())
		pos := file.NodeInfo(node.FieldTag()).Start()
		extendee := fd.ContainingMessage()
		if err := s.AddExtension(packageFor(extendee), extendee.FullName(), fd.Number(), pos, handler); err != nil {
			return err
		}

		return nil
	})
}

func (s *Symbols) importResult(r *result, handler *reporter.Handler) error {
	pkg, err := s.importPackages(packageNameStart(r), r.Package(), handler)
	if err != nil || pkg == nil {
		return err
	}
	_, err = pkg.importResult(r, handler)
	return err
}

func (s *packageSymbols) importResult(r *result, handler *reporter.Handler) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.files[r]; ok {
		// already imported
		return false, nil
	}

	// first pass: check for conflicts
	if err := s.checkResultLocked(r, handler); err != nil {
		return false, err
	}
	if err := handler.Error(); err != nil {
		return false, err
	}

	// second pass: commit all symbols
	s.commitResultLocked(r)

	return true, nil
}

func (s *packageSymbols) checkResultLocked(r *result, handler *reporter.Handler) error {
	resultSyms := map[protoreflect.FullName]symbolEntry{}
	return walk.DescriptorProtos(r.FileDescriptorProto(), func(fqn protoreflect.FullName, d proto.Message) error {
		_, isEnumVal := d.(*descriptorpb.EnumValueDescriptorProto)
		file := r.FileNode()
		node := r.Node(d)
		pos := nameStart(file, node)
		// check symbols already in this symbol table
		if existing, ok := s.symbols[fqn]; ok {
			if err := reportSymbolCollision(pos, fqn, isEnumVal, existing, handler); err != nil {
				return err
			}
		}

		// also check symbols from this result (that are not yet in symbol table)
		if existing, ok := resultSyms[fqn]; ok {
			if err := reportSymbolCollision(pos, fqn, isEnumVal, existing, handler); err != nil {
				return err
			}
		}
		resultSyms[fqn] = symbolEntry{
			pos:         pos,
			isEnumValue: isEnumVal,
		}

		return nil
	})
}

func packageNameStart(r *result) ast.SourcePos {
	if node, ok := r.FileNode().(*ast.FileNode); ok {
		for _, decl := range node.Decls {
			if pkgNode, ok := decl.(*ast.PackageNode); ok {
				return r.FileNode().NodeInfo(pkgNode.Name).Start()
			}
		}
	}
	return ast.UnknownPos(r.Path())
}

func nameStart(file ast.FileDeclNode, n ast.Node) ast.SourcePos {
	// TODO: maybe ast package needs a NamedNode interface to simplify this?
	switch n := n.(type) {
	case ast.FieldDeclNode:
		return file.NodeInfo(n.FieldName()).Start()
	case ast.MessageDeclNode:
		return file.NodeInfo(n.MessageName()).Start()
	case ast.OneOfDeclNode:
		return file.NodeInfo(n.OneOfName()).Start()
	case ast.EnumValueDeclNode:
		return file.NodeInfo(n.GetName()).Start()
	case *ast.EnumNode:
		return file.NodeInfo(n.Name).Start()
	case *ast.ServiceNode:
		return file.NodeInfo(n.Name).Start()
	case ast.RPCDeclNode:
		return file.NodeInfo(n.GetName()).Start()
	default:
		return file.NodeInfo(n).Start()
	}
}

func (s *packageSymbols) commitResultLocked(r *result) {
	if s.symbols == nil {
		s.symbols = map[protoreflect.FullName]symbolEntry{}
	}
	if s.exts == nil {
		s.exts = map[extNumber]ast.SourcePos{}
	}
	_ = walk.DescriptorProtos(r.FileDescriptorProto(), func(fqn protoreflect.FullName, d proto.Message) error {
		pos := nameStart(r.FileNode(), r.Node(d))
		_, isEnumValue := d.(protoreflect.EnumValueDescriptor)
		s.symbols[fqn] = symbolEntry{pos: pos, isEnumValue: isEnumValue}
		return nil
	})

	if s.files == nil {
		s.files = map[protoreflect.FileDescriptor]struct{}{}
	}
	s.files[r] = struct{}{}
}

func (s *Symbols) AddExtension(pkg, extendee protoreflect.FullName, tag protoreflect.FieldNumber, pos ast.SourcePos, handler *reporter.Handler) error {
	if pkg != "" {
		if !strings.HasPrefix(string(extendee), string(pkg)+".") {
			return handler.HandleErrorf(pos, "could not register extension: extendee %q does not match package %q", extendee, pkg)
		}
	}
	pkgSyms := s.getPackage(pkg)
	if pkgSyms == nil {
		// should never happen
		return handler.HandleErrorf(pos, "could not register extension: missing package symbols for %q", pkg)
	}
	return pkgSyms.addExtension(extendee, tag, pos, handler)
}

func (s *packageSymbols) addExtension(extendee protoreflect.FullName, tag protoreflect.FieldNumber, pos ast.SourcePos, handler *reporter.Handler) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.exts == nil {
		s.exts = map[extNumber]ast.SourcePos{}
	}

	extNum := extNumber{extendee: extendee, tag: tag}
	if existing, ok := s.exts[extNum]; ok {
		if err := handler.HandleErrorf(pos, "extension with tag %d for message %s already defined at %v", tag, extendee, existing); err != nil {
			return err
		}
	} else {
		s.exts[extNum] = pos
	}
	return nil
}
