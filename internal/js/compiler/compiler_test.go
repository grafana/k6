package compiler

import (
	"io"
	"testing"

	"github.com/grafana/sobek"
	"github.com/grafana/sobek/parser"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/lib/testutils"
	"go.k6.io/k6/lib"
)

func TestCompile(t *testing.T) {
	t.Parallel()
	t.Run("ES5", func(t *testing.T) {
		t.Parallel()
		c := New(testutils.NewLogger(t))
		src := `1+(function() { return 2; })()`
		prg, code, err := c.Parse(src, "script.js", false, false)
		require.NoError(t, err)
		assert.Equal(t, src, code)
		pgm, err := sobek.CompileAST(prg, true)
		require.NoError(t, err)
		v, err := sobek.New().RunProgram(pgm)
		require.NoError(t, err)
		assert.Equal(t, int64(3), v.Export())
	})

	t.Run("ES5 Wrap", func(t *testing.T) {
		t.Parallel()
		c := New(testutils.NewLogger(t))
		src := `exports.d=1+(function() { return 2; })()`
		prg, code, err := c.Parse(src, "script.js", true, false)
		require.NoError(t, err)
		pgm, err := sobek.CompileAST(prg, true)
		require.NoError(t, err)
		assert.Equal(t, "(function(module, exports){exports.d=1+(function() { return 2; })()\n})\n", code)
		rt := sobek.New()
		v, err := rt.RunProgram(pgm)
		if assert.NoError(t, err) {
			fn, ok := sobek.AssertFunction(v)
			if assert.True(t, ok, "not a function") {
				exp := make(map[string]sobek.Value)
				_, err := fn(sobek.Undefined(), sobek.Undefined(), rt.ToValue(exp))
				if assert.NoError(t, err) {
					assert.Equal(t, int64(3), exp["d"].Export())
				}
			}
		}
	})

	t.Run("ES6", func(t *testing.T) {
		t.Parallel()
		c := New(testutils.NewLogger(t))
		c.Options.CompatibilityMode = lib.CompatibilityModeExtended
		prg, code, err := c.Parse(`import "something"`, "script.js", false, true)
		require.NoError(t, err)
		assert.Equal(t, `import "something"`, code)
		require.NoError(t, err)
		_, err = sobek.CompileAST(prg, true)
		require.NoError(t, err)
		// TODO running this is a bit more involved :(
	})

	t.Run("Wrap", func(t *testing.T) {
		// This only works with `require` as wrapping means the import/export won't be top level and that is forbidden
		t.Parallel()
		c := New(testutils.NewLogger(t))
		c.Options.CompatibilityMode = lib.CompatibilityModeExtended
		prg, code, err := c.Parse(`require("something");`, "script.js", true, false)
		require.NoError(t, err)
		assert.Equal(t, `(function(module, exports){require("something");
})
`, code)
		var requireCalled bool
		rt := sobek.New()
		require.NoError(t, rt.Set("require", func(s string) {
			assert.Equal(t, "something", s)
			requireCalled = true
		}))

		pgm, err := sobek.CompileAST(prg, true)
		require.NoError(t, err)
		v, err := rt.RunProgram(pgm)
		require.NoError(t, err)
		fn, ok := sobek.AssertFunction(v)
		require.True(t, ok, "not a function")
		_, err = fn(sobek.Undefined())
		assert.NoError(t, err)
		require.True(t, requireCalled)
	})

	t.Run("Invalid", func(t *testing.T) {
		t.Parallel()
		c := New(testutils.NewLogger(t))
		c.Options.CompatibilityMode = lib.CompatibilityModeExtended
		_, _, err := c.Parse(`1+(=>2)()`, "script.js", false, false)
		assert.IsType(t, parser.ErrorList{}, err)
		assert.Contains(t, err.Error(), `Line 1:4 Unexpected token =>`)
	})
}

func TestCorruptSourceMap(t *testing.T) {
	t.Parallel()
	corruptSourceMap := []byte(`{"mappings": 12}`) // 12 is a number not a string

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	logger.Out = io.Discard
	hook := testutils.SimpleLogrusHook{
		HookedLevels: []logrus.Level{logrus.InfoLevel, logrus.WarnLevel},
	}
	logger.AddHook(&hook)

	compiler := New(logger)
	compiler.Options = Options{
		SourceMapLoader: func(string) ([]byte, error) {
			return corruptSourceMap, nil
		},
	}
	_, _, err := compiler.Parse("var s = 5;\n//# sourceMappingURL=somefile", "somefile", false, false)
	require.NoError(t, err)
	entries := hook.Drain()
	require.Len(t, entries, 1)
	msg, err := entries[0].String() // we need this in order to get the field error
	require.NoError(t, err)

	require.Contains(t, msg, `Couldn't load source map for somefile`)
	// @mstoykov: this is split as message changed in go1.24
	require.Contains(t, msg, `json: cannot unmarshal number into Go struct field`)
	require.Contains(t, msg, `mappings of type string`)
}

func TestMinimalSourceMap(t *testing.T) {
	t.Parallel()
	corruptSourceMap := []byte(`{"version":3,"mappings":";","sources":[]}`)

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	logger.Out = io.Discard
	hook := testutils.SimpleLogrusHook{
		HookedLevels: []logrus.Level{logrus.InfoLevel, logrus.WarnLevel},
	}
	logger.AddHook(&hook)

	compiler := New(logger)
	compiler.Options = Options{
		CompatibilityMode: lib.CompatibilityModeExtended,
		SourceMapLoader: func(string) ([]byte, error) {
			return corruptSourceMap, nil
		},
	}
	_, _, err := compiler.Parse("class s {};\n//# sourceMappingURL=somefile", "somefile", false, false)
	require.NoError(t, err)
	require.Empty(t, hook.Drain())
}
