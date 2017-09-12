package compiler

import (
	"strings"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	c, err := New()
	assert.NotNil(t, c)
	assert.NoError(t, err)
}

func TestTransform(t *testing.T) {
	c, err := New()
	if !assert.NoError(t, err) {
		return
	}

	t.Run("blank", func(t *testing.T) {
		src, _, err := c.Transform("", "test.js")
		assert.NoError(t, err)
		assert.Equal(t, `"use strict";`, src)
		// assert.Equal(t, 3, srcmap.Version)
		// assert.Equal(t, "test.js", srcmap.File)
		// assert.Equal(t, "", srcmap.Mappings)
	})
	t.Run("double-arrow", func(t *testing.T) {
		src, _, err := c.Transform("()=> true", "test.js")
		assert.NoError(t, err)
		assert.Equal(t, `"use strict";(function () {return true;});`, src)
		// assert.Equal(t, 3, srcmap.Version)
		// assert.Equal(t, "test.js", srcmap.File)
		// assert.Equal(t, "aAAA,qBAAK,IAAL", srcmap.Mappings)
	})
	t.Run("longer", func(t *testing.T) {
		src, _, err := c.Transform(strings.Join([]string{
			`function add(a, b) {`,
			`    return a + b;`,
			`};`,
			``,
			`let res = add(1, 2);`,
		}, "\n"), "test.js")
		assert.NoError(t, err)
		assert.Equal(t, strings.Join([]string{
			`"use strict";function add(a, b) {`,
			`    return a + b;`,
			`};`,
			``,
			`var res = add(1, 2);`,
		}, "\n"), src)
		// assert.Equal(t, 3, srcmap.Version)
		// assert.Equal(t, "test.js", srcmap.File)
		// assert.Equal(t, "aAAA,SAASA,GAAT,CAAaC,CAAb,EAAgBC,CAAhB,EAAmB;AACf,WAAOD,IAAIC,CAAX;AACH;;AAED,IAAIC,MAAMH,IAAI,CAAJ,EAAO,CAAP,CAAV", srcmap.Mappings)
	})
}

func TestCompile(t *testing.T) {
	c, err := New()
	if !assert.NoError(t, err) {
		return
	}
	t.Run("ES5", func(t *testing.T) {
		src := `1+(function() { return 2; })()`
		pgm, code, err := c.Compile(src, "script.js", "", "", true)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, src, code)
		v, err := goja.New().RunProgram(pgm)
		if assert.NoError(t, err) {
			assert.Equal(t, int64(3), v.Export())
		}

		t.Run("Wrap", func(t *testing.T) {
			pgm, code, err := c.Compile(src, "script.js", "(function(){return ", "})", true)
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, `(function(){return 1+(function() { return 2; })()})`, code)
			v, err := goja.New().RunProgram(pgm)
			if assert.NoError(t, err) {
				fn, ok := goja.AssertFunction(v)
				if assert.True(t, ok, "not a function") {
					v, err := fn(goja.Undefined())
					if assert.NoError(t, err) {
						assert.Equal(t, int64(3), v.Export())
					}
				}
			}
		})

		t.Run("Invalid", func(t *testing.T) {
			src := `1+(function() { return 2; )()`
			_, _, err := c.Compile(src, "script.js", "", "", true)
			assert.IsType(t, &goja.Exception{}, err)
			assert.EqualError(t, err, `SyntaxError: script.js: Unexpected token (1:26)
> 1 | 1+(function() { return 2; )()
    |                           ^ at <eval>:2:26853(114)`)
		})
	})
	t.Run("ES6", func(t *testing.T) {
		pgm, code, err := c.Compile(`1+(()=>2)()`, "script.js", "", "", true)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, `"use strict";1 + function () {return 2;}();`, code)
		v, err := goja.New().RunProgram(pgm)
		if assert.NoError(t, err) {
			assert.Equal(t, int64(3), v.Export())
		}

		t.Run("Wrap", func(t *testing.T) {
			pgm, code, err := c.Compile(`fn(1+(()=>2)())`, "script.js", "(function(fn){", "})", true)
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, `(function(fn){"use strict";fn(1 + function () {return 2;}());})`, code)
			rt := goja.New()
			v, err := rt.RunProgram(pgm)
			if assert.NoError(t, err) {
				fn, ok := goja.AssertFunction(v)
				if assert.True(t, ok, "not a function") {
					var out interface{}
					_, err := fn(goja.Undefined(), rt.ToValue(func(v goja.Value) {
						out = v.Export()
					}))
					assert.NoError(t, err)
					assert.Equal(t, int64(3), out)
				}
			}
		})

		t.Run("Invalid", func(t *testing.T) {
			_, _, err := c.Compile(`1+(=>2)()`, "script.js", "", "", true)
			assert.IsType(t, &goja.Exception{}, err)
			assert.EqualError(t, err, `SyntaxError: script.js: Unexpected token (1:3)
> 1 | 1+(=>2)()
    |    ^ at <eval>:2:26853(114)`)
		})
	})
}
