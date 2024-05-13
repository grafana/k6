package compiler

import (
	"errors"
	"testing"

	"github.com/dop251/goja/parser"
	"github.com/stretchr/testify/require"
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
