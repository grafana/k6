package browser

import (
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/require"
)

func TestSobekEmptyString(t *testing.T) {
	t.Parallel()
	// SobekEmpty string should return true if the argument
	// is an empty string or not defined in the Sobek runtime.
	rt := sobek.New()
	require.NoError(t, rt.Set("sobekEmptyString", sobekEmptyString))
	for _, s := range []string{"() => true", "'() => false'"} { // not empty
		v, err := rt.RunString(`sobekEmptyString(` + s + `)`)
		require.NoError(t, err)
		require.Falsef(t, v.ToBoolean(), "got: true, want: false for %q", s)
	}
	for _, s := range []string{"", "  ", "null", "undefined"} { // empty
		v, err := rt.RunString(`sobekEmptyString(` + s + `)`)
		require.NoError(t, err)
		require.Truef(t, v.ToBoolean(), "got: false, want: true for %q", s)
	}
}

func TestPageFuncStringClosesObjectLiteralParens(t *testing.T) {
	t.Parallel()

	rt := sobek.New()
	v, err := rt.RunString(`() => ({ a: 1 })`)
	require.NoError(t, err)
	require.Equal(t, "() => ({ a: 1 }", v.String(), "document Sobek dropping the grouping paren")
	require.Equal(t, "() => ({ a: 1 })", pageFuncString(v))
}

func TestCloseUnbalancedParens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in, want string
	}{
		{in: "() => 0", want: "() => 0"},
		{in: "() => ({ a: 1 }", want: "() => ({ a: 1 })"},
		{in: "() => ({ window, document }", want: "() => ({ window, document })"},
		{in: "() => ([1, 2]", want: "() => ([1, 2])"},
		{in: `() => ("(")`, want: `() => ("(")`},
		{in: "function() { return 1 }", want: "function() { return 1 }"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, closeUnbalancedParens(tt.in))
		})
	}
}
