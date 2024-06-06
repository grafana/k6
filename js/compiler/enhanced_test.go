package compiler

import (
	"errors"
	"testing"

	"github.com/dop251/goja/parser"
	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/testutils"
)

func Test_esbuildTransform_js(t *testing.T) {
	t.Parallel()

	code, srcMap, err := esbuildTransform(`export default function(name) { return "Hello, " + name }`, "script.js")

	require.NoError(t, err)
	require.NotNil(t, srcMap)
	require.NotEmpty(t, code)
}

func Test_esbuildTransform_ts(t *testing.T) {
	t.Parallel()

	script := `export function hello(name:string) : string { return "Hello, " + name}`

	code, srcMap, err := esbuildTransform(script, "script.ts")

	require.NoError(t, err)
	require.NotNil(t, srcMap)
	require.NotEmpty(t, code)
}

func Test_esbuildTransform_error(t *testing.T) {
	t.Parallel()

	script := `export function hello(name:string) : string { return "Hello, " + name}`

	_, _, err := esbuildTransform(script, "script.js")

	require.Error(t, err)

	var perr *parser.Error

	require.True(t, errors.As(err, &perr))
	require.NotNil(t, perr.Position)
	require.Equal(t, "script.js", perr.Position.Filename)
	require.Equal(t, 1, perr.Position.Line)
	require.Equal(t, 26, perr.Position.Column)
	require.Equal(t, "Expected \")\" but found \":\"", perr.Message)
}

func TestCompile_experimental_enhanced(t *testing.T) {
	t.Parallel()

	t.Run("experimental_enhanced Invalid", func(t *testing.T) {
		t.Parallel()
		c := New(testutils.NewLogger(t))
		src := `1+(function() { return 2; )()`
		c.Options.CompatibilityMode = lib.CompatibilityModeExperimentalEnhanced
		_, _, err := c.Compile(src, "script.js", false)
		assert.IsType(t, &parser.Error{}, err)
		assert.Contains(t, err.Error(), `script.js: Line 1:26 Unexpected ")"`)
	})
	t.Run("experimental_enhanced", func(t *testing.T) {
		t.Parallel()
		c := New(testutils.NewLogger(t))
		c.Options.CompatibilityMode = lib.CompatibilityModeExperimentalEnhanced
		pgm, code, err := c.Compile(`import "something"`, "script.js", true)
		require.NoError(t, err)
		assert.Equal(t, `var import_something = require("something");
`, code)
		rt := sobek.New()
		var requireCalled bool
		require.NoError(t, rt.Set("require", func(s string) {
			assert.Equal(t, "something", s)
			requireCalled = true
		}))
		_, err = rt.RunProgram(pgm)
		require.NoError(t, err)
		require.True(t, requireCalled)
	})
	t.Run("experimental_enhanced sourcemap", func(t *testing.T) {
		t.Parallel()
		c := New(testutils.NewLogger(t))
		c.Options.CompatibilityMode = lib.CompatibilityModeExperimentalEnhanced
		c.Options.SourceMapLoader = func(_ string) ([]byte, error) { return nil, nil }
		_, code, err := c.Compile(`import "something"`, "script.js", true)
		require.NoError(t, err)
		assert.Equal(t, `var import_something = require("something");

//# sourceMappingURL=k6://internal-should-not-leak/file.map`, code)
	})
}
