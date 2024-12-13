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
