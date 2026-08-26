package module_test

import (
	"testing"

	"github.com/grafana/xk6-faker/module"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/js/modulestest"
)

func Test_Default_Faker(t *testing.T) {
	t.Parallel()

	runtime := modulestest.NewRuntime(t)
	runtime.VU.InitEnvField.RuntimeOptions.Env = map[string]string{"XK6_FAKER_SEED": "11"}

	runtime.VU.InitEnvField.LookupEnv = func(key string) (string, bool) {
		val, ok := runtime.VU.InitEnvField.RuntimeOptions.Env[key]

		return val, ok
	}

	err := runtime.SetupModuleSystem(map[string]any{module.ImportPath: module.New()}, nil, nil)

	require.NoError(t, err)

	val, err := runtime.RunOnEventLoop(`
	let faker = require("` + module.ImportPath + `")
	faker.default.call("username")
	`)

	require.NoError(t, err)
	require.Equal(t, "Abshire5538", val.String())
}

func Test_New_Faker(t *testing.T) {
	t.Parallel()

	runtime := modulestest.NewRuntime(t)
	err := runtime.SetupModuleSystem(map[string]any{module.ImportPath: module.New()}, nil, nil)

	require.NoError(t, err)

	val, err := runtime.RunOnEventLoop(`
	let faker = require("` + module.ImportPath + `")
	new faker.Faker(11).call("username")
	`)

	require.NoError(t, err)
	require.Equal(t, "Abshire5538", val.String())
}
