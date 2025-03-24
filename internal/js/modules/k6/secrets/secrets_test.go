package secrets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/secretsource/mock"
	"go.k6.io/k6/js/modulestest"
	"go.k6.io/k6/secretsource"
)

func testRuntimeWithSecrets(t testing.TB, secretSources map[string]secretsource.Source) *modulestest.Runtime {
	testRuntime := modulestest.NewRuntime(t)
	var err error
	testRuntime.VU.InitEnvField.SecretsManager, _, err = secretsource.NewManager(secretSources)
	require.NoError(t, err)

	m, ok := New().NewModuleInstance(testRuntime.VU).(*Secrets)
	require.True(t, ok)
	require.NoError(t, testRuntime.VU.RuntimeField.Set("secrets", m.Exports().Default))

	return testRuntime
}

func TestSecrets(t *testing.T) {
	t.Parallel()

	type secretsTest struct {
		secretsources map[string]secretsource.Source
		script        string
		expectedValue any
		expectedError string
	}

	cases := map[string]secretsTest{
		"simple": {
			secretsources: map[string]secretsource.Source{
				"default": mock.NewMockSecretSource(map[string]string{
					"secret": "value",
				}),
			},
			script:        "await secrets.get('secret')",
			expectedValue: "value",
		},
		"error": {
			secretsources: map[string]secretsource.Source{
				"default": mock.NewMockSecretSource(map[string]string{
					"secret": "value",
				}),
			},
			script:        "await secrets.get('not_secret')",
			expectedError: "no value",
		},
		"multiple": {
			secretsources: map[string]secretsource.Source{
				"default": mock.NewMockSecretSource(map[string]string{
					"secret": "value",
				}),
				"second": mock.NewMockSecretSource(map[string]string{
					"secret2": "value2",
				}),
			},
			script:        "await secrets.get('secret')",
			expectedValue: "value",
		},
		"multiple get default": {
			secretsources: map[string]secretsource.Source{
				"default": mock.NewMockSecretSource(map[string]string{
					"secret": "value",
				}),
				"second": mock.NewMockSecretSource(map[string]string{
					"secret2": "value2",
				}),
			},
			script:        "await secrets.source('default').get('secret')",
			expectedValue: "value",
		},
		"multiple get not default": {
			secretsources: map[string]secretsource.Source{
				"default": mock.NewMockSecretSource(map[string]string{
					"secret": "value",
				}),
				"second": mock.NewMockSecretSource(map[string]string{
					"secret2": "value2",
				}),
			},
			script:        "await secrets.source('second').get('secret2')",
			expectedValue: "value2",
		},
		"multiple get wrong": {
			secretsources: map[string]secretsource.Source{
				"default": mock.NewMockSecretSource(map[string]string{
					"secret": "value",
				}),
				"second": mock.NewMockSecretSource(map[string]string{
					"secret2": "value2",
				}),
			},
			script:        "await secrets.source('third').get('secret2')",
			expectedError: "no secret source with name \"third\" is configured",
		},
		"get secret without source": {
			secretsources: map[string]secretsource.Source{},
			script:        "await secrets.get('secret')",
			expectedError: "no secret sources are configured",
		},
		"get none existing source": {
			secretsources: map[string]secretsource.Source{
				"default": mock.NewMockSecretSource(map[string]string{
					"secret": "value",
				}),
			},
			script:        "(await secrets.source('second')) != undefined",
			expectedValue: true,
		},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			testruntime := testRuntimeWithSecrets(t, testCase.secretsources)

			_, err := testruntime.RunOnEventLoop("(async ()=>{globalThis.result = " + testCase.script + "})()")
			if testCase.expectedError != "" {
				require.ErrorContains(t, err, testCase.expectedError)
				return
			}
			require.NoError(t, err)
			v := testruntime.VU.Runtime().GlobalObject().Get("result")
			assert.Equal(t, testCase.expectedValue, v.Export())
		})
	}
}
