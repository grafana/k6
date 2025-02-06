// Package compiler implements additional functionality for k6 to compile js code.
// more specifically wrapping code in CommonJS wrapper or transforming it through esbuild for typescript support.
// TODO this package name makes little sense now that it only parses and tranforms javascript.
// Although people do call such transformation - compilation, so maybe it is still fine
package compiler

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"

	"github.com/go-sourcemap/sourcemap"
	"github.com/grafana/sobek/ast"
	"github.com/grafana/sobek/parser"
	"github.com/sirupsen/logrus"

	"go.k6.io/k6/internal/usage"
	"go.k6.io/k6/lib"
)

// A Compiler compiles JavaScript or TypeScript source code into a sobek.Program
type Compiler struct {
	logger  logrus.FieldLogger
	Options Options
	usage   *usage.Usage
}

// New returns a new Compiler
func New(logger logrus.FieldLogger) *Compiler {
	return &Compiler{
		logger: logger,
		// TODO(@mstoykov): unfortunately otherwise we need to do much bigger rewrite around experimental extensions
		// and likely more extensions tests. This way tests don't need to care about this and the compiler code
		// doesn't need to check if it has usage set or not.
		usage: usage.New(),
	}
}

// WithUsage allows the compiler to be given [usage.Usage] to use
func (c *Compiler) WithUsage(u *usage.Usage) {
	c.usage = u
}

// Options are options to the compiler
type Options struct {
	CompatibilityMode lib.CompatibilityMode
	SourceMapLoader   func(string) ([]byte, error)
}

// parsingState is helper struct to keep the state of a parsing
type parsingState struct {
	// set when we couldn't load external source map so we can try parsing without loading it
	couldntLoadSourceMap bool
	// srcMap is the current full sourceMap that has been generated read so far
	srcMap            []byte
	srcMapError       error
	commonJSWrapped   bool // whether the original source is wrapped in a function to make it a CommonJS module
	compatibilityMode lib.CompatibilityMode
	compiler          *Compiler
	esm               bool

	loader func(string) ([]byte, error)
}

// Parse parses the provided source. It wraps as the same as CommonJS support.
// The returned program can be compiled directly by Sobek.
// Additionally, it returns the end code that has been parsed including any required transformations.
func (c *Compiler) Parse(
	src, filename string, commonJSWrap bool, esm bool,
) (prg *ast.Program, finalCode string, err error) {
	state := &parsingState{
		loader:            c.Options.SourceMapLoader,
		compatibilityMode: c.Options.CompatibilityMode,
		commonJSWrapped:   commonJSWrap,
		compiler:          c,
		esm:               esm,
	}
	return state.parseImpl(src, filename, commonJSWrap)
}

const (
	usageParsedFilesKey   = "usage/parsedFiles"
	usageParsedTSFilesKey = "usage/parsedTSFiles"
)

func (ps *parsingState) parseImpl(src, filename string, commonJSWrap bool) (*ast.Program, string, error) {
	if err := ps.compiler.usage.Uint64(usageParsedFilesKey, 1); err != nil {
		ps.compiler.logger.WithError(err).Warn("couldn't report usage for " + usageParsedFilesKey)
	}
	code := src
	if commonJSWrap { // the lines in the sourcemap (if available) will be fixed by increaseMappingsByOne
		code = ps.wrap(code, filename)
		ps.commonJSWrapped = true
	}
	var opts []parser.Option
	if ps.loader != nil {
		opts = append(opts, parser.WithSourceMapLoader(ps.sourceMapLoader))
	} else {
		opts = append(opts, parser.WithDisableSourceMaps)
	}

	if ps.esm {
		opts = append(opts, parser.IsModule)
	}

	prg, err := parser.ParseFile(nil, filename, code, 0, opts...)

	if ps.couldntLoadSourceMap {
		ps.couldntLoadSourceMap = false // reset
		// we probably don't want to abort scripts which have source maps but they can't be found,
		// this also will be a breaking change, so if we couldn't we retry with it disabled
		ps.compiler.logger.WithError(ps.srcMapError).Warnf("Couldn't load source map for %s", filename)
		ps.loader = nil
		return ps.parseImpl(src, filename, commonJSWrap)
	}

	if err == nil {
		return prg, code, nil
	}

	if strings.HasSuffix(filename, ".ts") {
		if err := ps.compiler.usage.Uint64(usageParsedTSFilesKey, 1); err != nil {
			ps.compiler.logger.WithError(err).Warn("couldn't report usage for " + usageParsedTSFilesKey)
		}
		code, ps.srcMap, err = StripTypes(src, filename)
		if err != nil {
			return nil, "", err
		}
		if ps.loader != nil {
			// This hack is required for the source map to work
			code += "\n//# sourceMappingURL=" + internalSourceMapURL
		}
		ps.commonJSWrapped = false
		ps.compatibilityMode = lib.CompatibilityModeBase
		return ps.parseImpl(code, filename, commonJSWrap)
	}
	return nil, "", err
}

func (ps *parsingState) wrap(code, filename string) string {
	conditionalNewLine := ""
	if index := strings.LastIndex(code, "//# sourceMappingURL="); index != -1 {
		// the lines in the sourcemap (if available) will be fixed by increaseMappingsByOne
		conditionalNewLine = "\n"
		newCode, err := ps.updateInlineSourceMap(code, index)
		if err != nil {
			ps.compiler.logger.Warnf("while parsing %q, couldn't update its inline sourcemap which might lead "+
				"to some line numbers being off: %s", filename, err)
		} else {
			code = newCode
		}

		// if there is no sourcemap - bork only the first line of code, but leave the remaining ones.
	}
	return "(function(module, exports){" + conditionalNewLine + code + "\n})\n"
}

const internalSourceMapURL = "k6://internal-should-not-leak/file.map"

// sourceMapLoader is to be used with Sobek's WithSourceMapLoader to add more than just loading files from disk.
// It additionally:
// - Loads a source-map if the it was generated from internal process.
// - It additioanlly fixes off-by-one error for CommonJS dependencies due to having to wrap them in a functions.
func (ps *parsingState) sourceMapLoader(path string) ([]byte, error) {
	if path == internalSourceMapURL {
		if ps.commonJSWrapped {
			return ps.increaseMappingsByOne(ps.srcMap)
		}
		return ps.srcMap, nil
	}
	ps.srcMap, ps.srcMapError = ps.loader(path)
	if ps.srcMapError != nil {
		ps.couldntLoadSourceMap = true
		return nil, ps.srcMapError
	}
	_, ps.srcMapError = sourcemap.Parse(path, ps.srcMap)
	if ps.srcMapError != nil {
		ps.couldntLoadSourceMap = true
		ps.srcMap = nil
		return nil, ps.srcMapError
	}
	if ps.commonJSWrapped {
		return ps.increaseMappingsByOne(ps.srcMap)
	}
	return ps.srcMap, nil
}

func (ps *parsingState) updateInlineSourceMap(code string, index int) (string, error) {
	nextnewline := strings.Index(code[index:], "\n")
	if nextnewline == -1 {
		nextnewline = len(code[index:])
	}
	mapurl := code[index : index+nextnewline]
	const base64EncodePrefix = "application/json;base64,"
	if startOfBase64EncodedSourceMap := strings.Index(mapurl, base64EncodePrefix); startOfBase64EncodedSourceMap != -1 {
		startOfBase64EncodedSourceMap += len(base64EncodePrefix)
		b, err := base64.StdEncoding.DecodeString(mapurl[startOfBase64EncodedSourceMap:])
		if err != nil {
			return code, err
		}
		b, err = ps.increaseMappingsByOne(b)
		if err != nil {
			return code, err
		}
		encoded := base64.StdEncoding.EncodeToString(b)
		code = code[:index] + "//# sourceMappingURL=data:application/json;base64," + encoded + code[index+nextnewline:]
	}
	return code, nil
}

// increaseMappingsByOne increases the lines in the sourcemap by line so that it fixes the case where we need to wrap a
// required file in a function to support/emulate commonjs
func (ps *parsingState) increaseMappingsByOne(sourceMap []byte) ([]byte, error) {
	var err error
	m := make(map[string]interface{})
	if err = json.Unmarshal(sourceMap, &m); err != nil {
		return nil, err
	}
	mappings, ok := m["mappings"]
	if !ok {
		// no mappings, no idea what this will do, but just return it as technically we can have sourcemap with sections
		// TODO implement incrementing of `offset` in the sections? to support that case as well
		// see https://sourcemaps.info/spec.html#h.n05z8dfyl3yh
		//
		// TODO (kind of alternatively) drop the newline in the "commonjs" wrapping and have only the first line wrong
		// and drop this whole function
		return sourceMap, nil
	}
	if str, ok := mappings.(string); ok {
		// ';' is the separator between lines so just adding 1 will make all mappings be for the line after which they were
		// originally
		m["mappings"] = ";" + str
	} else {
		// we have mappings but it's not a string - this is some kind of error
		// we still won't abort the test but just not load the sourcemap
		ps.couldntLoadSourceMap = true
		return nil, errors.New(`missing "mappings" in sourcemap`)
	}

	return json.Marshal(m)
}
