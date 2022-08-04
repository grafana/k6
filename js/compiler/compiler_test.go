package compiler

import (
	"errors"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/dop251/goja"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/testutils"
)

func TestTransform(t *testing.T) {
	t.Parallel()
	t.Run("blank", func(t *testing.T) {
		t.Parallel()
		c := New(testutils.NewLogger(t))
		src, _, err := c.Transform("", "test.js", nil)
		assert.NoError(t, err)
		assert.Equal(t, `"use strict";`, src)
	})
	t.Run("double-arrow", func(t *testing.T) {
		t.Parallel()
		c := New(testutils.NewLogger(t))
		src, _, err := c.Transform("()=> true", "test.js", nil)
		assert.NoError(t, err)
		assert.Equal(t, `"use strict";() => true;`, src)
	})
	t.Run("longer", func(t *testing.T) {
		t.Parallel()
		c := New(testutils.NewLogger(t))
		src, _, err := c.Transform(strings.Join([]string{
			`function add(a, b) {`,
			`    return a + b;`,
			`};`,
			``,
			`let res = add(1, 2);`,
		}, "\n"), "test.js", nil)
		assert.NoError(t, err)
		assert.Equal(t, strings.Join([]string{
			`"use strict";function add(a, b) {`,
			`    return a + b;`,
			`};`,
			``,
			`let res = add(1, 2);`,
		}, "\n"), src)
	})

	t.Run("double-arrow with sourceMap", func(t *testing.T) {
		t.Parallel()
		c := New(testutils.NewLogger(t))
		c.Options.SourceMapLoader = func(string) ([]byte, error) { return nil, errors.New("shouldn't be called") }
		src, _, err := c.Transform("()=> true", "test.js", nil)
		assert.NoError(t, err)
		assert.Equal(t, `"use strict";

() => true;
//# sourceMappingURL=k6://internal-should-not-leak/file.map`, src)
	})
}

func TestCompile(t *testing.T) {
	t.Parallel()
	t.Run("ES5", func(t *testing.T) {
		t.Parallel()
		c := New(testutils.NewLogger(t))
		src := `1+(function() { return 2; })()`
		pgm, code, err := c.Compile(src, "script.js", true)
		require.NoError(t, err)
		assert.Equal(t, src, code)
		v, err := goja.New().RunProgram(pgm)
		if assert.NoError(t, err) {
			assert.Equal(t, int64(3), v.Export())
		}
	})

	t.Run("ES5 Wrap", func(t *testing.T) {
		t.Parallel()
		c := New(testutils.NewLogger(t))
		src := `exports.d=1+(function() { return 2; })()`
		pgm, code, err := c.Compile(src, "script.js", false)
		require.NoError(t, err)
		assert.Equal(t, "(function(module, exports){\nexports.d=1+(function() { return 2; })()\n})\n", code)
		rt := goja.New()
		v, err := rt.RunProgram(pgm)
		if assert.NoError(t, err) {
			fn, ok := goja.AssertFunction(v)
			if assert.True(t, ok, "not a function") {
				exp := make(map[string]goja.Value)
				_, err := fn(goja.Undefined(), goja.Undefined(), rt.ToValue(exp))
				if assert.NoError(t, err) {
					assert.Equal(t, int64(3), exp["d"].Export())
				}
			}
		}
	})

	t.Run("ES5 Invalid", func(t *testing.T) {
		t.Parallel()
		c := New(testutils.NewLogger(t))
		src := `1+(function() { return 2; )()`
		c.Options.CompatibilityMode = lib.CompatibilityModeExtended
		_, _, err := c.Compile(src, "script.js", false)
		assert.IsType(t, &goja.Exception{}, err)
		assert.Contains(t, err.Error(), `SyntaxError: script.js: Unexpected token (1:26)
> 1 | 1+(function() { return 2; )()`)
	})
	t.Run("ES6", func(t *testing.T) {
		t.Parallel()
		c := New(testutils.NewLogger(t))
		c.Options.CompatibilityMode = lib.CompatibilityModeExtended
		pgm, code, err := c.Compile(`import "something"`, "script.js", true)
		require.NoError(t, err)
		assert.Equal(t, `"use strict";require("something");`,
			code)
		rt := goja.New()
		var requireCalled bool
		require.NoError(t, rt.Set("require", func(s string) {
			assert.Equal(t, "something", s)
			requireCalled = true
		}))
		_, err = rt.RunProgram(pgm)
		require.NoError(t, err)
		require.True(t, requireCalled)
	})

	t.Run("Wrap", func(t *testing.T) {
		t.Parallel()
		c := New(testutils.NewLogger(t))
		c.Options.CompatibilityMode = lib.CompatibilityModeExtended
		pgm, code, err := c.Compile(`import "something";`, "script.js", false)
		require.NoError(t, err)
		assert.Equal(t, `(function(module, exports){
"use strict";require("something");
})
`, code)
		var requireCalled bool
		rt := goja.New()
		require.NoError(t, rt.Set("require", func(s string) {
			assert.Equal(t, "something", s)
			requireCalled = true
		}))
		v, err := rt.RunProgram(pgm)
		require.NoError(t, err)
		fn, ok := goja.AssertFunction(v)
		require.True(t, ok, "not a function")
		_, err = fn(goja.Undefined())
		assert.NoError(t, err)
		require.True(t, requireCalled)
	})

	t.Run("Invalid", func(t *testing.T) {
		t.Parallel()
		c := New(testutils.NewLogger(t))
		c.Options.CompatibilityMode = lib.CompatibilityModeExtended
		_, _, err := c.Compile(`1+(=>2)()`, "script.js", true)
		assert.IsType(t, &goja.Exception{}, err)
		assert.Contains(t, err.Error(), `SyntaxError: script.js: Unexpected token (1:3)
> 1 | 1+(=>2)()`)
	})
}

func TestCorruptSourceMap(t *testing.T) {
	t.Parallel()
	corruptSourceMap := []byte(`{"mappings": 12}`) // 12 is a number not a string

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	logger.Out = ioutil.Discard
	hook := testutils.SimpleLogrusHook{
		HookedLevels: []logrus.Level{logrus.InfoLevel, logrus.WarnLevel},
	}
	logger.AddHook(&hook)

	compiler := New(logger)
	compiler.Options = Options{
		Strict: true,
		SourceMapLoader: func(string) ([]byte, error) {
			return corruptSourceMap, nil
		},
	}
	_, _, err := compiler.Compile("var s = 5;\n//# sourceMappingURL=somefile", "somefile", false)
	require.NoError(t, err)
	entries := hook.Drain()
	require.Len(t, entries, 1)
	msg, err := entries[0].String() // we need this in order to get the field error
	require.NoError(t, err)

	require.Contains(t, msg, `Couldn't load source map for somefile`)
	require.Contains(t, msg, `json: cannot unmarshal number into Go struct field v3.mappings of type string`)
}

func TestCorruptSourceMapOnlyForBabel(t *testing.T) {
	t.Parallel()
	// this a valid source map for the go implementation but babel doesn't like it
	corruptSourceMap := []byte(`{"mappings": ";"}`)

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	logger.Out = ioutil.Discard
	hook := testutils.SimpleLogrusHook{
		HookedLevels: []logrus.Level{logrus.InfoLevel, logrus.WarnLevel},
	}
	logger.AddHook(&hook)

	compiler := New(logger)
	compiler.Options = Options{
		CompatibilityMode: lib.CompatibilityModeExtended,
		Strict:            true,
		SourceMapLoader: func(string) ([]byte, error) {
			return corruptSourceMap, nil
		},
	}
	_, _, err := compiler.Compile("import 'something';\n//# sourceMappingURL=somefile", "somefile", false)
	require.NoError(t, err)
	entries := hook.Drain()
	require.Len(t, entries, 1)
	msg, err := entries[0].String() // we need this in order to get the field error
	require.NoError(t, err)

	require.Contains(t, msg, `needs to be transpiled by Babel, but its source map will not be accepted by Babel`)
	require.Contains(t, msg, `source map missing required 'version' field`)
}

func TestMinimalSourceMap(t *testing.T) {
	t.Parallel()
	// this is the minimal sourcemap valid for both go and babel implementations
	corruptSourceMap := []byte(`{"version":3,"mappings":";","sources":[]}`)

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	logger.Out = ioutil.Discard
	hook := testutils.SimpleLogrusHook{
		HookedLevels: []logrus.Level{logrus.InfoLevel, logrus.WarnLevel},
	}
	logger.AddHook(&hook)

	compiler := New(logger)
	compiler.Options = Options{
		CompatibilityMode: lib.CompatibilityModeExtended,
		Strict:            true,
		SourceMapLoader: func(string) ([]byte, error) {
			return corruptSourceMap, nil
		},
	}
	_, _, err := compiler.Compile("class s {};\n//# sourceMappingURL=somefile", "somefile", false)
	require.NoError(t, err)
	require.Empty(t, hook.Drain())
}
