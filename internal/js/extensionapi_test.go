package js_test

import (
	"context"
	"fmt"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	extensionapi "go.k6.io/k6-extension-api"
	"go.k6.io/k6/v2/internal/js"
	"go.k6.io/k6/v2/internal/lib/testutils"
	"go.k6.io/k6/v2/internal/loader"
	"go.k6.io/k6/v2/internal/usage"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/lib/fsext"
	"go.k6.io/k6/v2/metrics"
)

var extensionAPIModuleNumber int64 //nolint:gochecknoglobals // registrations are process-global

type extensionAPITestModule struct{}

func (extensionAPITestModule) NewModuleInstance(vu extensionapi.VU) extensionapi.Instance {
	return extensionAPITestInstance{vu: vu}
}

type extensionAPITestInstance struct {
	vu extensionapi.VU
}

func (mi extensionAPITestInstance) Exports() extensionapi.Exports {
	environment, environmentAvailable := mi.vu.(extensionapi.Environment)
	environmentValue := ""
	if environmentAvailable {
		environmentValue, _ = environment.LookupEnv("K6_EXTENSION_API_TEST_ENV")
	}
	promises, promisesAvailable := mi.vu.(extensionapi.Promises)
	resolveAsync := func() any {
		if !promisesAvailable {
			return nil
		}

		promise, resolver := promises.NewPromise()
		go resolver.Resolve("resolved asynchronously")
		return promise
	}
	scheduler, schedulerAvailable := mi.vu.(extensionapi.Scheduler)
	scheduleAsync := func() any {
		if !schedulerAvailable {
			return nil
		}

		promise, resolve, _ := mi.vu.Runtime().NewPromise()
		callback := scheduler.RegisterCallback()
		go callback(func() error { return resolve("scheduled asynchronously") })
		return promise
	}

	return extensionapi.Exports{Default: map[string]any{
		"runtimeAvailable":     func() bool { return mi.vu.Runtime() != nil },
		"contextAvailable":     func() bool { return mi.vu.Context() != nil },
		"environmentAvailable": func() bool { return environmentAvailable },
		"environmentValue":     func() string { return environmentValue },
		"promisesAvailable":    func() bool { return promisesAvailable },
		"resolveAsync":         resolveAsync,
		"schedulerAvailable":   func() bool { return schedulerAvailable },
		"scheduleAsync":        scheduleAsync,
	}}
}

func TestNewJSRunnerWithExtensionAPI(t *testing.T) {
	t.Parallel()

	moduleName := fmt.Sprintf("k6/x/extension-api-%d", atomic.AddInt64(&extensionAPIModuleNumber, 1))
	rawModuleName := fmt.Sprintf("k6/x/extension-api-raw-%d", atomic.AddInt64(&extensionAPIModuleNumber, 1))
	extensionapi.Register(moduleName, extensionAPITestModule{})
	extensionapi.Register(rawModuleName, map[string]any{"value": 42})

	logger := testutils.NewLogger(t)
	registry := metrics.NewRegistry()
	preInitState := &lib.TestPreInitState{
		Logger:         logger,
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(registry),
		Registry:       registry,
		Usage:          usage.New(),
		LookupEnv: func(key string) (string, bool) {
			return "extension-api-test-environment", key == "K6_EXTENSION_API_TEST_ENV"
		},
	}
	moduleResolver := js.NewModuleResolver(&url.URL{}, preInitState, nil)
	runner, err := js.New(
		preInitState,
		&loader.SourceData{
			URL: &url.URL{Path: "extension-api.js", Scheme: "file"},
			Data: fmt.Appendf(nil, `
				import extension from %q;
				import rawExtension from %q;
				if (!extension.runtimeAvailable() || !extension.contextAvailable() ||
					!extension.environmentAvailable() ||
					extension.environmentValue() !== "extension-api-test-environment") {
					throw new Error("extension API VU was not adapted");
				}
				if (rawExtension.value !== 42) {
					throw new Error("raw extension API export was not adapted");
				}
				export const options = { vus: 1, iterations: 1 };
				export default async function () {
					if (!extension.runtimeAvailable() || !extension.contextAvailable() ||
						!extension.environmentAvailable() ||
						extension.environmentValue() !== "extension-api-test-environment") {
						throw new Error("extension API VU was not adapted");
					}
					if (!extension.promisesAvailable() ||
						await extension.resolveAsync() !== "resolved asynchronously") {
						throw new Error("extension API promise was not adapted");
					}
					if (!extension.schedulerAvailable() ||
						await extension.scheduleAsync() !== "scheduled asynchronously") {
						throw new Error("extension API scheduler was not adapted");
					}
				}
			`, moduleName, rawModuleName),
		},
		map[string]fsext.Fs{"file": fsext.NewMemMapFs(), "https": fsext.NewMemMapFs()},
		moduleResolver,
	)
	require.NoError(t, err)

	vu, err := runner.NewVU(context.Background(), 1, 1, make(chan metrics.SampleContainer, 100))
	require.NoError(t, err)
	require.NoError(t, vu.Activate(&lib.VUActivationParams{RunContext: context.Background()}).RunOnce())
}
