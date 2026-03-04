package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPopulateRuntimeOptionsFromEnvJSObservability(t *testing.T) {
	flags := runtimeOptionFlagSet(false)
	opts := runtimeOptionsFromFlags(flags)

	env := map[string]string{
		"K6_JS_PROFILING_ENABLED":             "true",
		"K6_JS_PROFILING_SCOPE":               "vu",
		"K6_JS_CPU_PROFILE_OUTPUT":            "cpu.pprof",
		"K6_JS_RUNTIME_TRACE_OUTPUT":          "run.trace",
		"K6_JS_PROFILE_ID":                    "abc123",
		"K6_JS_FIRST_RUNNER_MEM_MAX_BYTES":    "500mb",
		"K6_JS_FIRST_RUNNER_MEM_STEP_PERCENT": "5",
	}
	opts, err := populateRuntimeOptionsFromEnv(opts, env)
	require.NoError(t, err)
	require.True(t, opts.JSProfilingEnabled.Valid)
	require.True(t, opts.JSProfilingEnabled.Bool)
	require.Equal(t, "vu", opts.JSProfilingScope.String)
	require.Equal(t, "cpu.pprof", opts.JSCPUProfileOutput.String)
	require.Equal(t, "run.trace", opts.JSRuntimeTraceOutput.String)
	require.Equal(t, "abc123", opts.JSProfileID.String)
	require.Equal(t, "500mb", opts.JSFirstRunnerMemMaxBytes.String)
	require.Equal(t, int64(5), opts.JSFirstRunnerMemStepPercent.Int64)
}

func TestRuntimeOptionsFromFlagsJSObservability(t *testing.T) {
	flags := runtimeOptionFlagSet(false)
	require.NoError(t, flags.Parse([]string{
		"--js-profiling-enabled",
		"--js-profiling-scope=init",
		"--js-cpu-profile-output=one.pprof",
		"--js-runtime-trace-output=one.trace",
		"--js-profile-id=test1",
		"--js-first-runner-mem-max-bytes=2gb",
		"--js-first-runner-mem-step-percent=7",
	}))

	opts := runtimeOptionsFromFlags(flags)
	require.True(t, opts.JSProfilingEnabled.Bool)
	require.Equal(t, "init", opts.JSProfilingScope.String)
	require.Equal(t, "one.pprof", opts.JSCPUProfileOutput.String)
	require.Equal(t, "one.trace", opts.JSRuntimeTraceOutput.String)
	require.Equal(t, "test1", opts.JSProfileID.String)
	require.Equal(t, "2gb", opts.JSFirstRunnerMemMaxBytes.String)
	require.Equal(t, int64(7), opts.JSFirstRunnerMemStepPercent.Int64)
}

func TestRuntimeOptionsInvalidJSProfilingEnv(t *testing.T) {
	opts := runtimeOptionsFromFlags(runtimeOptionFlagSet(false))
	_, err := populateRuntimeOptionsFromEnv(opts, map[string]string{
		"K6_JS_PROFILING_ENABLED": "not-bool",
	})
	require.Error(t, err)
}

func TestRuntimeOptionsInvalidFirstRunnerMemMaxBytes(t *testing.T) {
	flags := runtimeOptionFlagSet(false)
	require.NoError(t, flags.Parse([]string{
		"--js-first-runner-mem-max-bytes=1xb",
	}))
	_, err := getRuntimeOptions(flags, map[string]string{})
	require.Error(t, err)
}
