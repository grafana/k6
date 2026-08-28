package icmp

import (
	_ "embed"
	"os"
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/require"
	extensionapi "go.k6.io/k6-extension-api"
	extensionapitest "go.k6.io/k6-extension-api/test"
)

func Test_module(t *testing.T) {
	t.Parallel()

	root := new(rootModule)
	vu := extensionapitest.NewVU()
	vu.RegisterBuiltinMetric(extensionapi.BuiltinDataSent, "data_sent")
	vu.RegisterBuiltinMetric(extensionapi.BuiltinDataReceived, "data_received")
	mod := root.NewModuleInstance(vu)

	exports := mod.Exports()
	require.NotNil(t, exports)

	require.Nil(t, exports.Default)
	require.Contains(t, exports.Named, "ping")
	require.Contains(t, exports.Named, "pingAsync")
}

type assertRootModule struct {
	tb testing.TB
}

func newAssertRoot(tb testing.TB) *assertRootModule {
	tb.Helper()

	return &assertRootModule{tb: tb}
}

func (r *assertRootModule) NewModuleInstance(_ extensionapi.VU) extensionapi.Instance {
	assertions := require.New(r.tb)
	return &assertModule{exports: map[string]any{
		"true":  func(value sobek.Value, message string) { assertions.True(value.ToBoolean(), message) },
		"false": func(value sobek.Value, message string) { assertions.False(value.ToBoolean(), message) },
		"equal": func(actual, expected sobek.Value, message string) {
			assertions.Equal(expected.Export(), actual.Export(), message)
		},
	}}
}

type assertModule struct {
	exports map[string]any
}

func (m *assertModule) Exports() extensionapi.Exports {
	return extensionapi.Exports{Named: m.exports}
}

func skipOnGitHubLinuxRunner(t *testing.T) {
	t.Helper()

	//nolint:forbidigo // reading CI env vars to skip a test is legitimate here
	if os.Getenv("GITHUB_ACTIONS") == "true" && os.Getenv("RUNNER_OS") == "Linux" {
		t.Skip("Skipping test in GitHub Actions Linux runner due to permission issues")
	}
}
