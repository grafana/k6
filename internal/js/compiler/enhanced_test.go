package compiler

import (
	"testing"

	"github.com/grafana/sobek"
	"github.com/grafana/sobek/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/lib/testutils"
)

func Test_esbuildTransform_js(t *testing.T) {
	t.Parallel()

	code, srcMap, err := StripTypes(`export default function(name) { return "Hello, " + name }`, "script.js")

	require.NoError(t, err)
	require.NotNil(t, srcMap)
	require.NotEmpty(t, code)
}

func Test_esbuildTransform_ts(t *testing.T) {
	t.Parallel()

	script := `export function hello(name:string) : string { return "Hello, " + name}`

	code, srcMap, err := StripTypes(script, "script.ts")

	require.NoError(t, err)
	require.NotNil(t, srcMap)
	require.NotEmpty(t, code)
}

func TestCompile_experimental_enhanced(t *testing.T) {
	t.Parallel()

	t.Run("experimental_enhanced Invalid", func(t *testing.T) {
		t.Parallel()
		c := New(testutils.NewLogger(t))
		src := `1+(function() { return 2; )()`
		_, _, err := c.Parse(src, "script.ts", false, false)
		assert.IsType(t, &parser.Error{}, err)
		assert.Contains(t, err.Error(), `script.ts: Line 1:26 Unexpected ")"`)
	})
	t.Run("experimental_enhanced", func(t *testing.T) {
		t.Parallel()
		c := New(testutils.NewLogger(t))
		prg, code, err := c.Parse(`let t :string = "something"; require(t);`, "script.ts", false, false)
		require.NoError(t, err)
		assert.Equal(t, `let t = "something";
require(t);
`, code)
		pgm, err := sobek.CompileAST(prg, true)
		require.NoError(t, err)
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
		c.Options.SourceMapLoader = func(_ string) ([]byte, error) { return nil, nil }
		_, code, err := c.Parse(`let t :string = "something"; require(t);`, "script.ts", false, false)
		require.NoError(t, err)
		assert.Equal(t, `let t = "something";
require(t);

//# sourceMappingURL=k6://internal-should-not-leak/file.map`, code)
	})
}
