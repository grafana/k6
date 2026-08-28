package faker

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/grafana/sobek"
	"github.com/grafana/xk6-faker/faker"
	"github.com/grafana/xk6-faker/module"
	"github.com/stretchr/testify/require"
)

//go:generate go run -tags codegen ./tools/codegen json ./functions.json
//go:embed functions.json
var functionsJSON []byte

func Test_functions_json(t *testing.T) {
	t.Parallel()

	var functions map[string]*gofakeit.Info

	require.NoError(t, json.Unmarshal(functionsJSON, &functions))
	require.Len(t, functions, len(faker.GetFuncLookups()))

	runtime := sobek.New()
	exports := module.New().NewModuleInstance(fakerTestVU{runtime: runtime}).Exports()
	require.NoError(t, runtime.Set("Faker", exports.Named["Faker"]))

	_, err := runtime.RunString("let faker = new Faker(11)")

	require.NoError(t, err)

	lookups := faker.GetFuncLookups()

	for name, info := range functions {
		require.Contains(t, lookups, name)

		val, err := runtime.RunString("typeof faker.zen." + name)
		require.NoError(t, err)
		require.Equal(t, "function", val.String())

		val, err = runtime.RunString(fmt.Sprintf("typeof faker.%s.%s", info.Category, name))
		require.NoError(t, err)
		require.Equal(t, "function", val.String())
	}
}

//go:generate go run -tags codegen ./tools/codegen it ./functions-test.js
//go:embed functions-test.js
var testJS string

func Test_functions_in_js(t *testing.T) {
	t.Parallel()

	runtime := sobek.New()
	exports := module.New().NewModuleInstance(fakerTestVU{runtime: runtime}).Exports()
	require.NoError(t, runtime.Set("__module", map[string]any{
		"default": exports.Default,
		"Faker":   exports.Named["Faker"],
	}))

	_, err := runtime.RunString(strings.Replace(testJS,
		`let mod = require("k6/x/faker");`, "let mod = __module;", 1))

	require.NoError(t, err)
}

type fakerTestVU struct{ runtime *sobek.Runtime }

func (vu fakerTestVU) Context() context.Context { return context.Background() }

func (vu fakerTestVU) Runtime() *sobek.Runtime { return vu.runtime }

//go:generate go run -tags codegen ./tools/codegen ts ./index.d.ts
//go:generate go run -tags codegen ./tools/codegen test ./smoke.test.js
