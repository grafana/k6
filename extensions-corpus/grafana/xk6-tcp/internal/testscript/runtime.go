package testscript

import (
	"net"
	"os"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/js/modulestest"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/lib/netext"
	"go.k6.io/k6/v2/lib/types"
	"go.k6.io/k6/v2/metrics"
)

func newTestRuntime(t *testing.T, modulePairs ...any) *modulestest.Runtime {
	t.Helper()

	runtime := modulestest.NewRuntime(t)
	runtime.BuiltinMetrics = metrics.RegisterBuiltinMetrics(runtime.VU.InitEnvField.Registry)
	runtime.VU.InitEnvField.BuiltinMetrics = runtime.BuiltinMetrics

	err := runtime.VU.RuntimeField.Set("console", newConsole(t))
	require.NoError(t, err)

	moduleMap := make(map[string]any, len(modulePairs)/2+1)

	// Add modules from variadic pairs (importPath, moduleInstance, importPath, moduleInstance, ...)
	for i := 0; i < len(modulePairs); i += 2 {
		if i+1 < len(modulePairs) {
			importPath, ok := modulePairs[i].(string)
			if !ok {
				t.Fatalf("module pair at index %d: expected string for import path, got %T", i, modulePairs[i])
			}

			moduleMap[importPath] = modulePairs[i+1]
		}
	}

	moduleMap["k6/execution"] = newExecutionRoot(t)

	err = runtime.SetupModuleSystem(moduleMap, nil, nil)
	require.NoError(t, err)

	env := make(map[string]string)

	for _, e := range os.Environ() { //nolint:forbidigo // test harness snapshots the process env
		if key, value, ok := strings.Cut(e, "="); ok {
			env[key] = value
		}
	}

	require.NoError(t, runtime.VU.Runtime().Set("__ENV", env))

	return runtime
}

const maxTestSampleContainers = 1000

func newTestVUState(t *testing.T) *lib.State {
	t.Helper()

	samples := make(chan metrics.SampleContainer, maxTestSampleContainers)

	t.Cleanup(func() {
		close(samples)
	})

	registry := metrics.NewRegistry()

	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	logger.Out = t.Output()

	dialer := netext.NewDialer(net.Dialer{}, netext.NewResolver(net.LookupIP, 0, types.DNSfirst, types.DNSpreferIPv4))

	return &lib.State{
		Options: lib.Options{
			SystemTags: &metrics.DefaultSystemTagSet,
		},
		Dialer:         dialer,
		Samples:        samples,
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(registry),
		Tags: lib.NewVUStateTags(registry.RootTagSet().WithTagsFromMap(
			map[string]string{"group": lib.RootGroupPath}),
		),
		Logger: logger,
	}
}
