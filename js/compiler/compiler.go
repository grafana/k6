/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2017 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package compiler

import (
	_ "embed" // we need this for embedding Babel
	"sync"
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja/parser"
	"github.com/mitchellh/mapstructure"
	"github.com/sirupsen/logrus"

	"go.k6.io/k6/lib"
)

//go:embed lib/babel.min.js
var babelSrc string //nolint:gochecknoglobals

var (
	DefaultOpts = map[string]interface{}{
		// "presets": []string{"latest"},
		"plugins": []interface{}{
			// es2015 https://github.com/babel/babel/blob/v6.26.0/packages/babel-preset-es2015/src/index.js
			[]interface{}{"transform-es2015-template-literals", map[string]interface{}{"loose": false, "spec": false}},
			"transform-es2015-literals",
			"transform-es2015-function-name",
			[]interface{}{"transform-es2015-arrow-functions", map[string]interface{}{"spec": false}},
			// "transform-es2015-block-scoped-functions", // in goja
			[]interface{}{"transform-es2015-classes", map[string]interface{}{"loose": false}},
			"transform-es2015-object-super",
			// "transform-es2015-shorthand-properties", // in goja
			"transform-es2015-duplicate-keys",
			[]interface{}{"transform-es2015-computed-properties", map[string]interface{}{"loose": false}},
			// "transform-es2015-for-of", // in goja
			// "transform-es2015-sticky-regex", // in goja
			// "transform-es2015-unicode-regex", // in goja
			// "check-es2015-constants", // in goja
			[]interface{}{"transform-es2015-spread", map[string]interface{}{"loose": false}},
			"transform-es2015-parameters",
			[]interface{}{"transform-es2015-destructuring", map[string]interface{}{"loose": false}},
			// "transform-es2015-block-scoping", // in goja
			// "transform-es2015-typeof-symbol", // in goja
			// all the other module plugins are just dropped
			[]interface{}{"transform-es2015-modules-commonjs", map[string]interface{}{"loose": false}},
			// "transform-regenerator", // Doesn't really work unless regeneratorRuntime is also added

			// es2016 https://github.com/babel/babel/blob/v6.26.0/packages/babel-preset-es2016/src/index.js
			"transform-exponentiation-operator",

			// es2017 https://github.com/babel/babel/blob/v6.26.0/packages/babel-preset-es2017/src/index.js
			// "syntax-trailing-function-commas", // in goja
			// "transform-async-to-generator", // Doesn't really work unless regeneratorRuntime is also added
		},
		"ast":           false,
		"sourceMaps":    false,
		"babelrc":       false,
		"compact":       false,
		"retainLines":   true,
		"highlightCode": false,
	}

	onceBabelCode      sync.Once     // nolint:gochecknoglobals
	globalBabelCode    *goja.Program // nolint:gochecknoglobals
	globalBabelCodeErr error         // nolint:gochecknoglobals
	onceBabel          sync.Once     // nolint:gochecknoglobals
	globalBabel        *babel        // nolint:gochecknoglobals
)

// A Compiler compiles JavaScript source code (ES5.1 or ES6) into a goja.Program
type Compiler struct {
	logger logrus.FieldLogger
	babel  *babel
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
func (c *Compiler) Transform(src, filename string) (code string, srcmap *SourceMap, err error) {
	if c.babel == nil {
		onceBabel.Do(func() {
			globalBabel, err = newBabel()
		})
		c.babel = globalBabel
	}
	if err != nil {
		return
	}

	code, srcmap, err = c.babel.Transform(c.logger, src, filename)
	return
}

// Compile the program in the given CompatibilityMode, wrapping it between pre and post code
func (c *Compiler) Compile(src, filename, pre, post string,
	strict bool, compatMode lib.CompatibilityMode) (*goja.Program, string, error) {
	code := pre + src + post
	ast, err := parser.ParseFile(nil, filename, code, 0, parser.WithDisableSourceMaps)
	if err != nil {
		if compatMode == lib.CompatibilityModeExtended {
			code, _, err = c.Transform(src, filename)
			if err != nil {
				return nil, code, err
			}
			// the compatibility mode "decreases" here as we shouldn't transform twice
			return c.Compile(code, filename, pre, post, strict, lib.CompatibilityModeBase)
		}
		return nil, code, err
	}
	pgm, err := goja.CompileAST(ast, strict)
	// Parsing only checks the syntax, not whether what the syntax expresses
	// is actually supported (sometimes).
	//
	// For example, destructuring looks a lot like an object with shorthand
	// properties, but this is only noticeable once the code is compiled, not
	// while parsing. Even now code such as `let [x] = [2]` doesn't return an
	// error on the parsing stage but instead in the compilation in base mode.
	//
	// So, because of this, if there is an error during compilation, it still might
	// be worth it to transform the code and try again.
	if err != nil {
		if compatMode == lib.CompatibilityModeExtended {
			code, _, err = c.Transform(src, filename)
			if err != nil {
				return nil, code, err
			}
			// the compatibility mode "decreases" here as we shouldn't transform twice
			return c.Compile(code, filename, pre, post, strict, lib.CompatibilityModeBase)
		}
		return nil, code, err
	}
	return pgm, code, err
}

type babel struct {
	vm        *goja.Runtime
	this      goja.Value
	transform goja.Callable
	m         sync.Mutex
}

func newBabel() (*babel, error) {
	onceBabelCode.Do(func() {
		globalBabelCode, globalBabelCodeErr = goja.Compile("<internal/k6/compiler/lib/babel.min.js>", babelSrc, false)
	})
	if globalBabelCodeErr != nil {
		return nil, globalBabelCodeErr
	}
	vm := goja.New()
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

// Transform the given code into ES5, while synchronizing to ensure only a single
// bundle instance / Goja VM is in use at a time.
func (b *babel) Transform(logger logrus.FieldLogger, src, filename string) (string, *SourceMap, error) {
	b.m.Lock()
	defer b.m.Unlock()
	opts := make(map[string]interface{})
	for k, v := range DefaultOpts {
		opts[k] = v
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
	var rawMap map[string]interface{}
	if err = b.vm.ExportTo(vO.Get("map"), &rawMap); err != nil {
		return code, nil, err
	}
	var srcMap SourceMap
	if err = mapstructure.Decode(rawMap, &srcMap); err != nil {
		return code, &srcMap, err
	}
	return code, &srcMap, err
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
