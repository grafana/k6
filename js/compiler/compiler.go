package compiler

import (
	"github.com/GeertJohan/go.rice"
	"github.com/dop251/goja"
	"github.com/mitchellh/mapstructure"
)

var (
	lib      = rice.MustFindBox("lib")
	babelSrc = lib.MustString("babel-standalone-bower/babel.min.js")

	DefaultOpts = map[string]interface{}{
		"presets":       []string{"latest"},
		"ast":           false,
		"sourceMaps":    true,
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

func (c *Compiler) Transform(src, filename string) (code string, srcmap SourceMap, err error) {
	opts := make(map[string]interface{})
	for k, v := range DefaultOpts {
		opts[k] = v
	}
	opts["filename"] = filename

	v, err := c.transform(c.this, c.vm.ToValue(src), c.vm.ToValue(opts))
	if err != nil {
		return code, srcmap, err
	}
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
