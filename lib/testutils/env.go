package testutils

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// SetEnv is a helper funcion for setting arbitrary environment variables and
// restoring the old ones at the end, usually by deferring the returned callback
// TODO: remove these hacks when we improve the configuration (hopefully
// completely, see https://github.com/loadimpact/k6/issues/883)... we shouldn't
// have to mess with the global environment at all...
func SetEnv(t *testing.T, newEnv []string) (restoreEnv func()) {
	actuallSetEnv := func(env []string, abortOnSetErr bool) {
		os.Clearenv()
		for _, e := range env {
			val := ""
			pair := strings.SplitN(e, "=", 2)
			if len(pair) > 1 {
				val = pair[1]
			}
			err := os.Setenv(pair[0], val)
			if abortOnSetErr {
				require.NoError(t, err)
			} else if err != nil {
				t.Logf(
					"Received a non-aborting but unexpected error '%s' when setting env.var '%s' to '%s'",
					err, pair[0], val,
				)
			}
		}
	}
	oldEnv := os.Environ()
	actuallSetEnv(newEnv, true)

	return func() {
		actuallSetEnv(oldEnv, false)
	}
}
