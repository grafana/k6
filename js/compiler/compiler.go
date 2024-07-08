// Package compiler implements additional functionality for k6 to compile js code.
// more specifically transpiling through babel in case that is needed.
package compiler

import (
	_ "embed" // we need this for embedding Babel
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-sourcemap/sourcemap"
	"github.com/grafana/sobek"
	"github.com/grafana/sobek/ast"
	"github.com/grafana/sobek/parser"
	"github.com/sirupsen/logrus"

	"go.k6.io/k6/lib"
)

//go:embed lib/babel.min.js
var babelSrc string

var (
	defaultOpts = map[string]interface{}{ //nolint:gochecknoglobals
		// "presets": []string{"latest"},
		"plugins": []interface{}{
			// es2015 https://github.com/babel/babel/blob/v6.26.0/packages/babel-preset-es2015/src/index.js
			// in Sobek
			// []interface{}{"transform-es2015-template-literals", map[string]interface{}{"loose": false, "spec": false}},
			// "transform-es2015-literals", // in Sobek
			// "transform-es2015-function-name", // in Sobek
			// []interface{}{"transform-es2015-arrow-functions", map[string]interface{}{"spec": false}}, // in Sobek
			// "transform-es2015-block-scoped-functions", // in Sobek
			// []interface{}{"transform-es2015-classes", map[string]interface{}{"loose": false}}, // in Sobek
			// "transform-es2015-object-super", // in Sobek
			// "transform-es2015-shorthand-properties", // in Sobek
			// "transform-es2015-duplicate-keys", // in Sobek
			// []interface{}{"transform-es2015-computed-properties", map[string]interface{}{"loose": false}}, // in Sobek
			// "transform-es2015-for-of", // in Sobek
			// "transform-es2015-sticky-regex", // in Sobek
			// "transform-es2015-unicode-regex", // in Sobek
			// "check-es2015-constants", // in Sobek
			// []interface{}{"transform-es2015-spread", map[string]interface{}{"loose": false}}, // in Sobek
			// "transform-es2015-parameters", // in Sobek
			// []interface{}{"transform-es2015-destructuring", map[string]interface{}{"loose": false}}, // in Sobek
			// "transform-es2015-block-scoping", // in Sobek
			// "transform-es2015-typeof-symbol", // in Sobek
			// all the other module plugins are just dropped
			[]interface{}{"transform-es2015-modules-commonjs", map[string]interface{}{"loose": false}},
			// "transform-regenerator", // Doesn't really work unless regeneratorRuntime is also added

			// es2016 https://github.com/babel/babel/blob/v6.26.0/packages/babel-preset-es2016/src/index.js
			// "transform-exponentiation-operator",

			// es2017 https://github.com/babel/babel/blob/v6.26.0/packages/babel-preset-es2017/src/index.js
			// "syntax-trailing-function-commas", // in Sobek
			// "transform-async-to-generator", // Doesn't really work unless regeneratorRuntime is also added
		},
		"ast":           false,
		"sourceMaps":    false,
		"babelrc":       false,
		"compact":       false,
		"retainLines":   true,
		"highlightCode": false,
	}

	maxSrcLenForBabelSourceMap     = 250 * 1024 //nolint:gochecknoglobals
	maxSrcLenForBabelSourceMapOnce sync.Once    //nolint:gochecknoglobals

	onceBabelCode      sync.Once      //nolint:gochecknoglobals
	globalBabelCode    *sobek.Program //nolint:gochecknoglobals
	errGlobalBabelCode error          //nolint:gochecknoglobals
	onceBabel          sync.Once      //nolint:gochecknoglobals
	globalBabel        *babel         //nolint:gochecknoglobals
)

const (
	maxSrcLenForBabelSourceMapVarName = "K6_DEBUG_SOURCEMAP_FILESIZE_LIMIT"
	sourceMapURLFromBabel             = "k6://internal-should-not-leak/file.map"
)

// A Compiler compiles JavaScript source code (ES5.1 or ES6) into a sobek.Program
type Compiler struct {
	logger  logrus.FieldLogger
	babel   *babel
	Options Options
}

// New returns a new Compiler
func New(logger logrus.FieldLogger) *Compiler {
	return &Compiler{logger: logger}
}

// initializeBabel initializes a separate (non-global) instance of babel specifically for this Compiler.
// An error is returned only if babel itself couldn't be parsed/run which should never be possible.
func (c *Compiler) initializeBabel() error {
	var err error
	if c.babel == nil {
		c.babel, err = newBabel()
	}
	return err
}

// Transform the given code into ES5
func (c *Compiler) Transform(src, filename string, inputSrcMap []byte) (code string, srcMap []byte, err error) {
	if c.babel == nil {
		onceBabel.Do(func() {
			globalBabel, err = newBabel()
		})
		c.babel = globalBabel
	}
	if err != nil {
		return "", nil, err
	}

	sourceMapEnabled := c.Options.SourceMapLoader != nil
	maxSrcLenForBabelSourceMapOnce.Do(func() {
		// TODO: drop this code and everything it's connected to when babel is dropped
		v := os.Getenv(maxSrcLenForBabelSourceMapVarName) //nolint:forbidigo
		if len(v) > 0 {
			i, err := strconv.Atoi(v)
			if err != nil {
				c.logger.Warnf("Tried to parse %q from %s as integer but couldn't %s\n",
					v, maxSrcLenForBabelSourceMapVarName, err)
				return
			}
			maxSrcLenForBabelSourceMap = i
		}
	})
	if sourceMapEnabled && len(src) > maxSrcLenForBabelSourceMap {
		sourceMapEnabled = false
		c.logger.Warnf("The source for `%s` needs to go through babel but is over %d bytes. "+
			"For performance reasons source map support will be disabled for this particular file.",
			filename, maxSrcLenForBabelSourceMap)
	}

	// check that babel will likely be able to parse the inputSrcMap
	if sourceMapEnabled && len(inputSrcMap) != 0 {
		if err = verifySourceMapForBabel(inputSrcMap); err != nil {
			sourceMapEnabled = false
			inputSrcMap = nil
			c.logger.WithError(err).Warnf(
				"The source for `%s` needs to be transpiled by Babel, but its source map will"+
					" not be accepted by Babel, so it was disabled", filename)
		}
	}
	return c.babel.transformImpl(c.logger, src, filename, sourceMapEnabled, inputSrcMap)
}

// Options are options to the compiler
type Options struct {
	CompatibilityMode lib.CompatibilityMode
	SourceMapLoader   func(string) ([]byte, error)
	Strict            bool
}

// parsingState is helper struct to keep the state of a compilation
type parsingState struct {
	// set when we couldn't load external source map so we can try parsing without loading it
	couldntLoadSourceMap bool
	// srcMap is the current full sourceMap that has been generated read so far
	srcMap            []byte
	srcMapError       error
	wrapped           bool // whether the original source is wrapped in a function to make it a commonjs module
	compatibilityMode lib.CompatibilityMode
	logger            logrus.FieldLogger

	loader func(string) ([]byte, error)

	compiler *Compiler
}

// Parse parses the provided source. It wraps as the same as CommonJS support.
// The returned program can be compiled directly by Sobek.
// Additionally, it returns the end code that has been parsed including any required transformations.
func (c *Compiler) Parse(
	src, filename string, wrap bool,
) (prg *ast.Program, finalCode string, err error) {
	state := &parsingState{
		loader:            c.Options.SourceMapLoader,
		wrapped:           wrap,
		compatibilityMode: c.Options.CompatibilityMode,
		logger:            c.logger,
		compiler:          c,
	}
	return state.parseImpl(src, filename, wrap)
}

// sourceMapLoader is to be used with Sobek's WithSourceMapLoader
// it not only gets the file from disk in the simple case, but also returns it if the map was generated from babel
// additioanlly it fixes off by one error in commonjs dependencies due to having to wrap them in a function.
func (ps *parsingState) sourceMapLoader(path string) ([]byte, error) {
	if path == sourceMapURLFromBabel {
		if ps.wrapped {
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
	if ps.wrapped {
		return ps.increaseMappingsByOne(ps.srcMap)
	}
	return ps.srcMap, nil
}

func (ps *parsingState) parseImpl(src, filename string, wrap bool) (*ast.Program, string, error) {
	code := src
	if wrap { // the lines in the sourcemap (if available) will be fixed by increaseMappingsByOne
		code = ps.wrap(code, filename)
		ps.wrapped = true
	}
	opts := parser.WithDisableSourceMaps
	if ps.loader != nil {
		opts = parser.WithSourceMapLoader(ps.sourceMapLoader)
	}
	prg, err := parser.ParseFile(nil, filename, code, 0, opts)

	if ps.couldntLoadSourceMap {
		ps.couldntLoadSourceMap = false // reset
		// we probably don't want to abort scripts which have source maps but they can't be found,
		// this also will be a breaking change, so if we couldn't we retry with it disabled
		ps.logger.WithError(ps.srcMapError).Warnf("Couldn't load source map for %s", filename)
		prg, err = parser.ParseFile(nil, filename, code, 0, parser.WithDisableSourceMaps)
	}

	if err == nil {
		return prg, code, nil
	}

	if ps.compatibilityMode == lib.CompatibilityModeExtended {
		code, ps.srcMap, err = ps.compiler.Transform(src, filename, ps.srcMap)
		if err != nil {
			return nil, code, err
		}
		ps.wrapped = false
		ps.compatibilityMode = lib.CompatibilityModeBase
		prg, code, err = ps.parseImpl(code, filename, wrap)
		if err == nil && strings.Contains(src, "module.exports") {
			ps.logger.Warningf(
				"During the compilation of %q, it has been detected that the file combines ECMAScript modules (ESM) "+
					"import/export syntax with commonJS module.exports. "+
					"Mixing these two module systems is non-standard and will not be supported anymore in future releases. "+
					"Please ensure to use solely one or the other syntax.",
				filename)
		}
		return prg, code, err
	}
	if ps.compatibilityMode == lib.CompatibilityModeExperimentalEnhanced {
		code, ps.srcMap, err = esbuildTransform(src, filename)
		if err != nil {
			return nil, "", err
		}
		if ps.loader != nil {
			// This hack is required for the source map to work
			code += "\n//# sourceMappingURL=" + sourceMapURLFromBabel
		}
		ps.wrapped = false
		ps.compatibilityMode = lib.CompatibilityModeBase
		return ps.parseImpl(code, filename, wrap)
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
			ps.logger.Warnf("while compiling %q, couldn't update its inline sourcemap which might lead "+
				"to some line numbers being off: %s", filename, err)
		} else {
			code = newCode
		}

		// if there is no sourcemap - bork only the first line of code, but leave the remaining ones.
	}
	return "(function(module, exports){" + conditionalNewLine + code + "\n})\n"
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

// transformImpl the given code into ES5, while synchronizing to ensure only a single
// bundle instance / Goja VM is in use at a time.
func (b *babel) transformImpl(
	logger logrus.FieldLogger, src, filename string, sourceMapsEnabled bool, inputSrcMap []byte,
) (string, []byte, error) {
	b.m.Lock()
	defer b.m.Unlock()
	opts := make(map[string]interface{})
	for k, v := range defaultOpts {
		opts[k] = v
	}
	if sourceMapsEnabled {
		// given that the source map should provide accurate lines(and columns), this option isn't needed
		// it also happens to make very long and awkward lines, especially around import/exports and definitely a lot
		// less readable overall. Hopefully it also has some performance improvement not trying to keep the same lines
		opts["retainLines"] = false
		opts["sourceMaps"] = true
		if inputSrcMap != nil {
			srcMap := new(map[string]interface{})
			if err := json.Unmarshal(inputSrcMap, &srcMap); err != nil {
				return "", nil, err
			}
			opts["inputSourceMap"] = srcMap
		}
	}
	opts["filename"] = filename

	startTime := time.Now()
	v, err := b.transform(b.this, b.vm.ToValue(src), b.vm.ToValue(opts))
	if err != nil {
		return "", nil, err
	}
	logger.WithField("t", time.Since(startTime)).Debug("Babel: Transformed")

	vO := v.ToObject(b.vm)
	var code string
	if err = b.vm.ExportTo(vO.Get("code"), &code); err != nil {
		return code, nil, err
	}
	if !sourceMapsEnabled {
		return code, nil, nil
	}

	// this is to make Sobek try to load a sourcemap.
	// it is a special url as it should never leak outside of this code
	// additionally the alternative support from babel is to embed *the whole* sourcemap at the end
	code += "\n//# sourceMappingURL=" + sourceMapURLFromBabel
	stringify, err := b.vm.RunString("(function(m) { return JSON.stringify(m)})")
	if err != nil {
		return code, nil, err
	}
	c, _ := sobek.AssertFunction(stringify)
	mapAsJSON, err := c(sobek.Undefined(), vO.Get("map"))
	if err != nil {
		return code, nil, err
	}
	return code, []byte(mapAsJSON.String()), nil
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
				err := co.initializeBabel()
				if err != nil {
					panic(err)
				}
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

func verifySourceMapForBabel(srcMap []byte) error {
	// this function exists to do what babel checks in sourcemap before we give it to it.
	m := make(map[string]json.RawMessage)
	err := json.Unmarshal(srcMap, &m)
	if err != nil {
		return fmt.Errorf("source map is not valid json: %w", err)
	}
	// there are no checks on it's value in babel
	// we technically only support v3 though
	if _, ok := m["version"]; !ok {
		return fmt.Errorf("source map missing required 'version' field")
	}

	// This actually gets checked by the go implementation so it's not really necessary
	if _, ok := m["mappings"]; !ok {
		return fmt.Errorf("source map missing required 'mappings' field")
	}
	// the go implementation checks the value even if it doesn't require it exists
	if _, ok := m["sources"]; !ok {
		return fmt.Errorf("source map missing required 'sources' field")
	}
	return nil
}

type babel struct {
	vm        *sobek.Runtime
	this      sobek.Value
	transform sobek.Callable
	m         sync.Mutex
}

func newBabel() (*babel, error) {
	onceBabelCode.Do(func() {
		globalBabelCode, errGlobalBabelCode = sobek.Compile("<internal/k6/compiler/lib/babel.min.js>", babelSrc, false)
	})
	if errGlobalBabelCode != nil {
		return nil, errGlobalBabelCode
	}
	vm := sobek.New()
	_, err := vm.RunProgram(globalBabelCode)
	if err != nil {
		return nil, err
	}

	this := vm.Get("Babel")
	bObj := this.ToObject(vm)
	result := &babel{vm: vm, this: this}
	if err = vm.ExportTo(bObj.Get("transform"), &result.transform); err != nil {
		return nil, err
	}

	return result, err
}
