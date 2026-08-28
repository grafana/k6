package icmp

import (
	"context"
	_ "embed"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/require"
	extensionapi "go.k6.io/k6-extension-api"
	extensionapitest "go.k6.io/k6-extension-api/test"
)

func runScriptTest(t *testing.T, filename string) {
	t.Helper()

	runtime := extensionapitest.NewRuntime()
	vu := runtime.VU
	vu.LookupEnvFunc = func(key string) (string, bool) {
		if key == "K6_PING_MINIMUM_INTERVAL" {
			return "0s", true
		}
		return "", false
	}
	vu.LookupHostFunc = func(ctx context.Context, host string) ([]string, error) {
		return net.DefaultResolver.LookupHost(ctx, host)
	}
	vu.RegisterBuiltinMetric(extensionapi.BuiltinDataSent, "data_sent")
	vu.RegisterBuiltinMetric(extensionapi.BuiltinDataReceived, "data_received")

	icmpExports := New().NewModuleInstance(vu).Exports().Named
	assertExports := newAssertRoot(t).NewModuleInstance(vu).Exports().Named

	module := vu.Runtime().NewObject()
	exports := vu.Runtime().NewObject()

	require.NoError(t, module.Set("exports", exports))
	require.NoError(t, vu.Runtime().Set("module", module))
	require.NoError(t, vu.Runtime().Set("require", func(path string) any {
		switch path {
		case ImportPath:
			return icmpExports
		case "k6/x/assert":
			return assertExports
		default:
			panic("unexpected module: " + path)
		}
	}))

	source, err := os.ReadFile(filename)
	require.NoError(t, err)

	_, err = vu.Runtime().RunString(string(source))
	require.NoError(t, err)

	result, err := vu.Runtime().RunString("module.exports")
	require.NoError(t, err)

	get := result.ToObject(vu.Runtime()).Get

	if fn, ok := sobek.AssertFunction(get("setup")); ok {
		require.NoError(t, runtime.EventLoop.Start(func() error {
			_, err := fn(sobek.Undefined())
			return err
		}))
	}

	fn, ok := sobek.AssertFunction(result)
	require.True(t, ok, "module.exports should be a function")

	require.NoError(t, runtime.EventLoop.Start(func() error {
		_, err := fn(sobek.Undefined())
		return err
	}))

	if fn, ok = sobek.AssertFunction(get("teardown")); ok {
		require.NoError(t, runtime.EventLoop.Start(func() error {
			_, err := fn(sobek.Undefined())
			return err
		}))
	}
}

func Test_script(t *testing.T) { //nolint:tparallel
	t.Parallel()

	skipOnGitHubLinuxRunner(t)

	files, err := filepath.Glob("testdata/*_test.cjs")

	require.NoError(t, err)
	require.NotEmpty(t, files, "No test scripts found in testdata directory")

	for _, file := range files { //nolint:paralleltest
		t.Run(filepath.ToSlash(file), func(t *testing.T) {
			runScriptTest(t, file)
		})
	}
}
