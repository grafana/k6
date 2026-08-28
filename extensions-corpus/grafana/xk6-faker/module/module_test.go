package module

import (
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/require"
)

func Test_Default_Faker(t *testing.T) {
	t.Parallel()

	runtime := sobek.New()
	exports := New().NewModuleInstance(testVU{
		runtime: runtime, environment: map[string]string{"XK6_FAKER_SEED": "11"},
	}).Exports()
	require.NoError(t, runtime.Set("faker", exports.Default))

	val, err := runtime.RunString("faker.call('username')")

	require.NoError(t, err)
	require.Equal(t, "Abshire5538", val.String())
}

func Test_New_Faker(t *testing.T) {
	t.Parallel()

	runtime := sobek.New()
	exports := New().NewModuleInstance(testVU{runtime: runtime}).Exports()
	require.NoError(t, runtime.Set("Faker", exports.Named["Faker"]))

	val, err := runtime.RunString("new Faker(11).call('username')")

	require.NoError(t, err)
	require.Equal(t, "Abshire5538", val.String())
}
