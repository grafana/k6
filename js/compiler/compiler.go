// Package compiler implements additional functionality for k6 to compile js code.
// more specifically transpiling through babel in case that is needed.
package compiler

import (
	"encoding/json"
	"errors"

	"github.com/dop251/goja"
	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/parser"
	"github.com/go-sourcemap/sourcemap"
	"github.com/sirupsen/logrus"

	"go.k6.io/k6/lib"
)

// A Compiler compiles JavaScript source code (ES5.1 or ES6) into a goja.Program
type Compiler struct {
	logger  logrus.FieldLogger
	Options Options
}

// New returns a new Compiler
func New(logger logrus.FieldLogger) *Compiler {
	return &Compiler{logger: logger}
}

// Options are options to the compiler
type Options struct {
	CompatibilityMode lib.CompatibilityMode
	SourceMapLoader   func(string) ([]byte, error)
	Strict            bool
}

// parseState is helper struct to keep the state of a compilation
type parseState struct {
	// set when we couldn't load external source map so we can try parsing without loading it
	couldntLoadSourceMap bool
	// srcMap is the current full sourceMap that has been generated read so far
	srcMap      []byte
	srcMapError error
	wrapped     bool // whether the original source is wrapped in a function to make it a commonjs module

	loader func(string) ([]byte, error)
}

// Parse the program in the given CompatibilityMode, wrapping it between pre and post code
func (c *Compiler) Parse(src, filename string, wrap bool) (*ast.Program, bool, error) {
	return c.parseImpl(src, filename, wrap)
}

// Compile the program in the given CompatibilityMode, wrapping it between pre and post code
// TODO isESM will be used once goja support ESM modules natively
func (c *Compiler) Compile(src, filename string, isESM bool) (*goja.Program, string, error) {
	return c.compileImpl(src, filename, !isESM)
}

// sourceMapLoader is to be used with goja's WithSourceMapLoader
// it not only gets the file from disk in the simple case, but also returns it if the map was generated from babel
// additioanlly it fixes off by one error in commonjs dependencies due to having to wrap them in a function.
func (c *parseState) sourceMapLoader(path string) ([]byte, error) {
	c.srcMap, c.srcMapError = c.loader(path)
	if c.srcMapError != nil {
		c.couldntLoadSourceMap = true
		return nil, c.srcMapError
	}
	_, c.srcMapError = sourcemap.Parse(path, c.srcMap)
	if c.srcMapError != nil {
		c.couldntLoadSourceMap = true
		c.srcMap = nil
		return nil, c.srcMapError
	}
	if c.wrapped {
		return c.increaseMappingsByOne(c.srcMap)
	}
	return c.srcMap, nil
}

func (c *Compiler) parseImpl(src, filename string, wrap bool) (*ast.Program, bool, error) {
	code := src
	state := parseState{loader: c.Options.SourceMapLoader, wrapped: wrap}
	if state.wrapped { // the lines in the sourcemap (if available) will be fixed by increaseMappingsByOne
		code = "(function(module, exports){\n" + code + "\n})\n"
	}
	opts := parser.WithDisableSourceMaps
	if c.Options.SourceMapLoader != nil {
		opts = parser.WithSourceMapLoader(state.sourceMapLoader)
	}
	ast, err := parser.ParseFile(nil, filename, code, 0, opts, parser.IsModule)

	if state.couldntLoadSourceMap {
		state.couldntLoadSourceMap = false // reset
		// we probably don't want to abort scripts which have source maps but they can't be found,
		// this also will be a breaking change, so if we couldn't we retry with it disabled
		c.logger.WithError(state.srcMapError).Warnf("Couldn't load source map for %s", filename)
		ast, err = parser.ParseFile(nil, filename, code, 0, parser.WithDisableSourceMaps, parser.IsModule)
	}

	if err != nil {
		return nil, false, err
	}
	isModule := len(ast.ExportEntries) > 0 || len(ast.ImportEntries) > 0 || ast.HasTLA
	return ast, isModule, nil
}

func (c *Compiler) compileImpl(src, filename string, wrap bool) (*goja.Program, string, error) {
	code := src
	state := parseState{loader: c.Options.SourceMapLoader, wrapped: wrap}
	if wrap { // the lines in the sourcemap (if available) will be fixed by increaseMappingsByOne
		code = "(function(module, exports){\n" + code + "\n})\n"
	}
	opts := parser.WithDisableSourceMaps
	if c.Options.SourceMapLoader != nil {
		opts = parser.WithSourceMapLoader(state.sourceMapLoader)
	}
	ast, err := parser.ParseFile(nil, filename, code, 0, opts, parser.IsModule)

	if state.couldntLoadSourceMap {
		state.couldntLoadSourceMap = false // reset
		// we probably don't want to abort scripts which have source maps but they can't be found,
		// this also will be a breaking change, so if we couldn't we retry with it disabled
		c.logger.WithError(state.srcMapError).Warnf("Couldn't load source map for %s", filename)
		ast, err = parser.ParseFile(nil, filename, code, 0, parser.WithDisableSourceMaps, parser.IsModule)
	}
	if err != nil {
		return nil, code, err
	}
	pgm, err := goja.CompileAST(ast, c.Options.Strict)
	return pgm, code, err
}

// increaseMappingsByOne increases the lines in the sourcemap by line so that it fixes the case where we need to wrap a
// required file in a function to support/emulate commonjs
func (c *parseState) increaseMappingsByOne(sourceMap []byte) ([]byte, error) {
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
		c.couldntLoadSourceMap = true
		return nil, errors.New(`missing "mappings" in sourcemap`)
	}

	return json.Marshal(m)
}

// Pool is a pool of compilers so it can be used easier in parallel tests as they have their own babel.
type Pool struct {
	c chan *Compiler
}

// NewPool creates a Pool that will be using the provided logger and will preallocate (in parallel)
// the count of compilers each with their own babel.
func NewPool(logger logrus.FieldLogger, count int) *Pool {
	c := &Pool{
		c: make(chan *Compiler, count),
	}
	go func() {
		for i := 0; i < count; i++ {
			go func() {
				co := New(logger)
				c.Put(co)
			}()
		}
	}()

	return c
}

// Get a compiler from the pool.
func (c *Pool) Get() *Compiler {
	return <-c.c
}

// Put a compiler back in the pool.
func (c *Pool) Put(co *Compiler) {
	c.c <- co
}
