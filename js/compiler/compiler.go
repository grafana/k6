package compiler

import (
	"time"

	"github.com/GeertJohan/go.rice"
	"github.com/dop251/goja"
	"github.com/dop251/goja/parser"
	"github.com/mitchellh/mapstructure"
	log "github.com/sirupsen/logrus"
)

var (
	lib      = rice.MustFindBox("lib")
	babelSrc = lib.MustString("babel-standalone-bower/babel.min.js")

	DefaultOpts = map[string]interface{}{
		"presets":       []string{"latest"},
		"ast":           false,
		"sourceMaps":    false,
		"babelrc":       false,
		"compact":       false,
		"retainLines":   true,
		"highlightCode": false,
	}
)

// A Compiler uses Babel to compile ES6 code into something ES5-compatible.
type Compiler struct {
	vm *goja.Runtime

	// JS pointers.
	this      goja.Value
	transform goja.Callable
}

// Constructs a new compiler.
func New() (*Compiler, error) {
	c := &Compiler{vm: goja.New()}
	if _, err := c.vm.RunString(babelSrc); err != nil {
		return nil, err
	}

	c.this = c.vm.Get("Babel")
	thisObj := c.this.ToObject(c.vm)
	if err := c.vm.ExportTo(thisObj.Get("transform"), &c.transform); err != nil {
		return nil, err
	}

	return c, nil
}

// Transform the given code into ES5.
func (c *Compiler) Transform(src, filename string) (code string, srcmap SourceMap, err error) {
	opts := make(map[string]interface{})
	for k, v := range DefaultOpts {
		opts[k] = v
	}
	opts["filename"] = filename

	startTime := time.Now()
	v, err := c.transform(c.this, c.vm.ToValue(src), c.vm.ToValue(opts))
	if err != nil {
		return code, srcmap, err
	}
	log.WithField("t", time.Since(startTime)).Debug("Babel: Transformed")
	vO := v.ToObject(c.vm)

	if err := c.vm.ExportTo(vO.Get("code"), &code); err != nil {
		return code, srcmap, err
	}

	var rawmap map[string]interface{}
	if err := c.vm.ExportTo(vO.Get("map"), &rawmap); err != nil {
		return code, srcmap, err
	}
	if err := mapstructure.Decode(rawmap, &srcmap); err != nil {
		return code, srcmap, err
	}

	return code, srcmap, nil
}

// Compiles the program, first trying ES5, then ES6.
func (c *Compiler) Compile(src, filename string, pre, post string, strict bool) (*goja.Program, string, error) {
	return c.compile(src, filename, pre, post, strict, true)
}

func (c *Compiler) compile(src, filename string, pre, post string, strict, tryBabel bool) (*goja.Program, string, error) {
	code := pre + src + post
	ast, err := parser.ParseFile(nil, filename, code, 0)
	if err != nil {
		if tryBabel {
			code, _, err := c.Transform(src, filename)
			if err != nil {
				return nil, code, err
			}
			return c.compile(code, filename, pre, post, strict, false)
		}
		return nil, src, err
	}
	pgm, err := goja.CompileAST(ast, strict)
	return pgm, code, err
}
