package module

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/js/modulestest"
)

func Test_getseed(t *testing.T) {
	t.Parallel()

	require.Equal(t, int64(0), getseed(nil))

	vu := modulestest.NewRuntime(t).VU

	require.Equal(t, int64(0), getseed(vu))

	vu.InitEnvField.RuntimeOptions.Env = map[string]string{}
	vu.InitEnvField.LookupEnv = func(key string) (string, bool) {
		val, ok := vu.InitEnvField.RuntimeOptions.Env[key]

		return val, ok
	}

	require.Equal(t, int64(0), getseed(vu))

	vu.InitEnvField.RuntimeOptions.Env["XK6_FAKER_SEED"] = "foo"

	require.Equal(t, int64(0), getseed(vu))

	vu.InitEnvField.RuntimeOptions.Env["XK6_FAKER_SEED"] = "42"

	require.Equal(t, int64(42), getseed(vu))
}
