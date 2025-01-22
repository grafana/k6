package cmd

import (
	"bytes"
	"fmt"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/internal/cmd/tests"
	"go.k6.io/k6/internal/loader"
	"go.k6.io/k6/internal/usage"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/fsext"
	"go.k6.io/k6/metrics"
)

type runtimeOptionsTestCase struct {
	useSysEnv bool // Whether to include the system env vars by default (run) or not (cloud/archive/inspect)
	expErr    bool
	cliFlags  []string
	systemEnv map[string]string
	expRTOpts lib.RuntimeOptions
}

func testRuntimeOptionsCase(t *testing.T, tc runtimeOptionsTestCase) {
	t.Helper()
	flags := runtimeOptionFlagSet(tc.useSysEnv)
	require.NoError(t, flags.Parse(tc.cliFlags))

	rtOpts, err := getRuntimeOptions(flags, tc.systemEnv)
	if tc.expErr {
		require.Error(t, err)
		return
	}
	require.NoError(t, err)
	require.Equal(t, tc.expRTOpts, rtOpts)

	compatMode, err := lib.ValidateCompatibilityMode(rtOpts.CompatibilityMode.String)
	require.NoError(t, err)

	jsCode := new(bytes.Buffer)
	if compatMode == lib.CompatibilityModeExtended {
		fmt.Fprint(jsCode, "export default function() {")
	} else {
		fmt.Fprint(jsCode, "module.exports.default = function() {")
	}

	for key, val := range tc.expRTOpts.Env {
		fmt.Fprintf(jsCode,
			"if (__ENV.%s !== `%s`) { throw new Error('Invalid %s: ' + __ENV.%s); }",
			key, val, key, key,
		)
	}
	fmt.Fprint(jsCode, "}")

	fs := fsext.NewMemMapFs()
	require.NoError(t, fsext.WriteFile(fs, "/script.js", jsCode.Bytes(), 0o644))

	ts := tests.NewGlobalTestState(t) // TODO: move upwards, make this into an almost full integration test
	registry := metrics.NewRegistry()
	test := &loadedTest{
		sourceRootPath: "script.js",
		source:         &loader.SourceData{Data: jsCode.Bytes(), URL: &url.URL{Path: "/script.js", Scheme: "file"}},
		fileSystems:    map[string]fsext.Fs{"file": fs},
		preInitState: &lib.TestPreInitState{
			Logger:         ts.Logger,
			RuntimeOptions: rtOpts,
			Registry:       registry,
			BuiltinMetrics: metrics.RegisterBuiltinMetrics(registry),
			Usage:          usage.New(),
		},
	}

	require.NoError(t, test.initializeFirstRunner(ts.GlobalState))

	archive := test.initRunner.MakeArchive()
	archiveBuf := &bytes.Buffer{}
	require.NoError(t, archive.Write(archiveBuf))

	getRunnerErr := func(rtOpts lib.RuntimeOptions) *loadedTest {
		return &loadedTest{
			sourceRootPath: "script.tar",
			source:         &loader.SourceData{Data: archiveBuf.Bytes(), URL: &url.URL{Path: "/script.tar", Scheme: "file"}},
			fileSystems:    map[string]fsext.Fs{"file": fs},
			preInitState: &lib.TestPreInitState{
				Logger:         ts.Logger,
				RuntimeOptions: rtOpts,
				Registry:       registry,
				BuiltinMetrics: metrics.RegisterBuiltinMetrics(registry),
				Usage:          usage.New(),
			},
		}
	}

	archTest := getRunnerErr(lib.RuntimeOptions{})
	require.NoError(t, archTest.initializeFirstRunner(ts.GlobalState))

	for key, val := range tc.expRTOpts.Env {
		archTest = getRunnerErr(lib.RuntimeOptions{Env: map[string]string{key: "almost " + val}})
		require.NoError(t, archTest.initializeFirstRunner(ts.GlobalState))
		assert.Equal(t, archTest.initRunner.MakeArchive().Env[key], "almost "+val)
	}
}

func TestRuntimeOptions(t *testing.T) {
	t.Parallel()
	var (
		defaultCompatMode   = null.NewString("extended", false)
		baseCompatMode      = null.NewString("base", true)
		extendedCompatMode  = null.NewString("extended", true)
		enhancedCompatMode  = null.NewString("experimental_enhanced", true)
		defaultTracesOutput = null.NewString("none", false)
	)

	runtimeOptionsTestCases := map[string]runtimeOptionsTestCase{
		"empty env": {
			useSysEnv: true,
			// everything else is empty
			expRTOpts: lib.RuntimeOptions{
				IncludeSystemEnvVars: null.NewBool(true, false),
				CompatibilityMode:    defaultCompatMode,
				Env:                  nil,
				TracesOutput:         defaultTracesOutput,
			},
		},
		"disabled sys env by default": {
			useSysEnv: false,
			systemEnv: map[string]string{"test1": "val1"},
			expRTOpts: lib.RuntimeOptions{
				IncludeSystemEnvVars: null.NewBool(false, false),
				CompatibilityMode:    defaultCompatMode,
				Env:                  map[string]string{},
				TracesOutput:         defaultTracesOutput,
			},
		},
		"disabled sys env by default with ext compat mode": {
			useSysEnv: false,
			systemEnv: map[string]string{"test1": "val1", "K6_COMPATIBILITY_MODE": "extended"},
			expRTOpts: lib.RuntimeOptions{
				IncludeSystemEnvVars: null.NewBool(false, false),
				CompatibilityMode:    extendedCompatMode,
				Env:                  map[string]string{},
				TracesOutput:         defaultTracesOutput,
			},
		},
		"disabled sys env by default with experimental_enhanced compat mode": {
			useSysEnv: false,
			systemEnv: map[string]string{"test1": "val1", "K6_COMPATIBILITY_MODE": "experimental_enhanced"},
			expRTOpts: lib.RuntimeOptions{
				IncludeSystemEnvVars: null.NewBool(false, false),
				CompatibilityMode:    enhancedCompatMode,
				Env:                  map[string]string{},
				TracesOutput:         defaultTracesOutput,
			},
		},
		"disabled sys env by cli 1": {
			useSysEnv: true,
			systemEnv: map[string]string{"test1": "val1", "K6_COMPATIBILITY_MODE": "base"},
			cliFlags:  []string{"--include-system-env-vars=false"},
			expRTOpts: lib.RuntimeOptions{
				IncludeSystemEnvVars: null.NewBool(false, true),
				CompatibilityMode:    baseCompatMode,
				Env:                  map[string]string{},
				TracesOutput:         defaultTracesOutput,
			},
		},
		"disabled sys env by cli 2": {
			useSysEnv: true,
			systemEnv: map[string]string{"K6_INCLUDE_SYSTEM_ENV_VARS": "true", "K6_COMPATIBILITY_MODE": "extended"},
			cliFlags:  []string{"--include-system-env-vars=0", "--compatibility-mode=base"},
			expRTOpts: lib.RuntimeOptions{
				IncludeSystemEnvVars: null.NewBool(false, true),
				CompatibilityMode:    baseCompatMode,
				Env:                  map[string]string{},
				TracesOutput:         defaultTracesOutput,
			},
		},
		"disabled sys env by env": {
			useSysEnv: true,
			systemEnv: map[string]string{"K6_INCLUDE_SYSTEM_ENV_VARS": "false", "K6_COMPATIBILITY_MODE": "extended"},
			expRTOpts: lib.RuntimeOptions{
				IncludeSystemEnvVars: null.NewBool(false, true),
				CompatibilityMode:    extendedCompatMode,
				Env:                  map[string]string{},
				TracesOutput:         defaultTracesOutput,
			},
		},
		"enabled sys env by env": {
			useSysEnv: false,
			systemEnv: map[string]string{"K6_INCLUDE_SYSTEM_ENV_VARS": "true", "K6_COMPATIBILITY_MODE": "extended"},
			expRTOpts: lib.RuntimeOptions{
				IncludeSystemEnvVars: null.NewBool(true, true),
				CompatibilityMode:    extendedCompatMode,
				Env:                  map[string]string{"K6_INCLUDE_SYSTEM_ENV_VARS": "true", "K6_COMPATIBILITY_MODE": "extended"},
				TracesOutput:         defaultTracesOutput,
			},
		},
		"enabled sys env by default": {
			useSysEnv: true,
			systemEnv: map[string]string{"test1": "val1"},
			cliFlags:  []string{},
			expRTOpts: lib.RuntimeOptions{
				IncludeSystemEnvVars: null.NewBool(true, false),
				CompatibilityMode:    defaultCompatMode,
				Env:                  map[string]string{"test1": "val1"},
				TracesOutput:         defaultTracesOutput,
			},
		},
		"enabled sys env by cli 1": {
			useSysEnv: false,
			systemEnv: map[string]string{"test1": "val1"},
			cliFlags:  []string{"--include-system-env-vars"},
			expRTOpts: lib.RuntimeOptions{
				IncludeSystemEnvVars: null.NewBool(true, true),
				CompatibilityMode:    defaultCompatMode,
				Env:                  map[string]string{"test1": "val1"},
				TracesOutput:         defaultTracesOutput,
			},
		},
		"enabled sys env by cli 2": {
			useSysEnv: false,
			systemEnv: map[string]string{"test1": "val1"},
			cliFlags:  []string{"--include-system-env-vars=true"},
			expRTOpts: lib.RuntimeOptions{
				IncludeSystemEnvVars: null.NewBool(true, true),
				CompatibilityMode:    defaultCompatMode,
				Env:                  map[string]string{"test1": "val1"},
				TracesOutput:         defaultTracesOutput,
			},
		},
		"run only system env": {
			useSysEnv: true,
			systemEnv: map[string]string{"test1": "val1"},
			cliFlags:  []string{},
			expRTOpts: lib.RuntimeOptions{
				IncludeSystemEnvVars: null.NewBool(true, false),
				CompatibilityMode:    defaultCompatMode,
				Env:                  map[string]string{"test1": "val1"},
				TracesOutput:         defaultTracesOutput,
			},
		},
		"mixed system and cli env": {
			useSysEnv: true,
			systemEnv: map[string]string{"test1": "val1", "test2": ""},
			cliFlags:  []string{"--env", "test3=val3", "-e", "test4", "-e", "test5="},
			expRTOpts: lib.RuntimeOptions{
				IncludeSystemEnvVars: null.NewBool(true, false),
				CompatibilityMode:    defaultCompatMode,
				Env:                  map[string]string{"test1": "val1", "test2": "", "test3": "val3", "test4": "", "test5": ""},
				TracesOutput:         defaultTracesOutput,
			},
		},
		"mixed system and cli env 2": {
			useSysEnv: false,
			systemEnv: map[string]string{"test1": "val1", "test2": ""},
			cliFlags:  []string{"--env", "test3=val3", "-e", "test4", "-e", "test5=", "--include-system-env-vars=1"},
			expRTOpts: lib.RuntimeOptions{
				IncludeSystemEnvVars: null.NewBool(true, true),
				CompatibilityMode:    defaultCompatMode,
				Env:                  map[string]string{"test1": "val1", "test2": "", "test3": "val3", "test4": "", "test5": ""},
				TracesOutput:         defaultTracesOutput,
			},
		},
		"disabled system env with cli params": {
			useSysEnv: false,
			systemEnv: map[string]string{"test1": "val1"},
			cliFlags:  []string{"-e", "test2=overwriten", "-e", "test2=val2"},
			expRTOpts: lib.RuntimeOptions{
				IncludeSystemEnvVars: null.NewBool(false, false),
				CompatibilityMode:    defaultCompatMode,
				Env:                  map[string]string{"test2": "val2"},
				TracesOutput:         defaultTracesOutput,
			},
		},
		"overwriting system env with cli param": {
			useSysEnv: true,
			systemEnv: map[string]string{"test1": "val1sys"},
			cliFlags:  []string{"--env", "test1=val1cli"},
			expRTOpts: lib.RuntimeOptions{
				IncludeSystemEnvVars: null.NewBool(true, false),
				CompatibilityMode:    defaultCompatMode,
				Env:                  map[string]string{"test1": "val1cli"},
				TracesOutput:         defaultTracesOutput,
			},
		},
		"error wrong compat mode env var value": {
			systemEnv: map[string]string{"K6_COMPATIBILITY_MODE": "asdf"},
			expErr:    true,
		},
		"error wrong compat mode env var value even with CLI flag": {
			systemEnv: map[string]string{"K6_COMPATIBILITY_MODE": "asdf"},
			cliFlags:  []string{"--compatibility-mode", "true"},
			expErr:    true,
		},
		"error wrong compat mode cli flag value": {
			cliFlags: []string{"--compatibility-mode", "whatever"},
			expErr:   true,
		},
		"error invalid cli var name 1": {
			useSysEnv: true,
			systemEnv: map[string]string{},
			cliFlags:  []string{"--env", "test a=error"},
			expErr:    true,
		},
		"error invalid cli var name 2": {
			useSysEnv: true,
			systemEnv: map[string]string{},
			cliFlags:  []string{"--env", "1var=error"},
			expErr:    true,
		},
		"error invalid cli var name 3": {
			useSysEnv: true,
			systemEnv: map[string]string{},
			cliFlags:  []string{"--env", "уникод=unicode-disabled"},
			expErr:    true,
		},
		"valid env vars with spaces": {
			useSysEnv: true,
			systemEnv: map[string]string{"test1": "value 1"},
			cliFlags:  []string{"--env", "test2=value 2"},
			expRTOpts: lib.RuntimeOptions{
				IncludeSystemEnvVars: null.NewBool(true, false),
				CompatibilityMode:    defaultCompatMode,
				Env:                  map[string]string{"test1": "value 1", "test2": "value 2"},
				TracesOutput:         defaultTracesOutput,
			},
		},
		"valid env vars with special chars": {
			useSysEnv: true,
			systemEnv: map[string]string{"test1": "value 1"},
			cliFlags:  []string{"--env", "test2=value,2", "-e", `test3= ,  ,,, value, ,, 2!'@#,"`},
			expRTOpts: lib.RuntimeOptions{
				IncludeSystemEnvVars: null.NewBool(true, false),
				CompatibilityMode:    defaultCompatMode,
				Env:                  map[string]string{"test1": "value 1", "test2": "value,2", "test3": ` ,  ,,, value, ,, 2!'@#,"`},
				TracesOutput:         defaultTracesOutput,
			},
		},
		"summary and thresholds from env": {
			useSysEnv: false,
			systemEnv: map[string]string{"K6_NO_THRESHOLDS": "false", "K6_NO_SUMMARY": "0", "K6_SUMMARY_EXPORT": "foo"},
			expRTOpts: lib.RuntimeOptions{
				IncludeSystemEnvVars: null.NewBool(false, false),
				CompatibilityMode:    defaultCompatMode,
				Env:                  map[string]string{},
				NoThresholds:         null.NewBool(false, true),
				NoSummary:            null.NewBool(false, true),
				SummaryExport:        null.NewString("foo", true),
				TracesOutput:         defaultTracesOutput,
			},
		},
		"summary and thresholds from env overwritten by CLI": {
			useSysEnv: false,
			systemEnv: map[string]string{"K6_NO_THRESHOLDS": "FALSE", "K6_NO_SUMMARY": "0", "K6_SUMMARY_EXPORT": "foo"},
			cliFlags:  []string{"--no-thresholds", "true", "--no-summary", "true", "--summary-export", "bar"},
			expRTOpts: lib.RuntimeOptions{
				IncludeSystemEnvVars: null.NewBool(false, false),
				CompatibilityMode:    defaultCompatMode,
				Env:                  map[string]string{},
				NoThresholds:         null.NewBool(true, true),
				NoSummary:            null.NewBool(true, true),
				SummaryExport:        null.NewString("bar", true),
				TracesOutput:         defaultTracesOutput,
			},
		},
		"env var error detected even when CLI flags overwrite 1": {
			useSysEnv: false,
			systemEnv: map[string]string{"K6_NO_THRESHOLDS": "boo"},
			cliFlags:  []string{"--no-thresholds", "true"},
			expErr:    true,
		},
		"env var error detected even when CLI flags overwrite 2": {
			useSysEnv: false,
			systemEnv: map[string]string{"K6_NO_SUMMARY": "hoo"},
			cliFlags:  []string{"--no-summary", "true"},
			expErr:    true,
		},
		"traces output default": {
			useSysEnv: false,
			expRTOpts: lib.RuntimeOptions{
				IncludeSystemEnvVars: null.NewBool(false, false),
				CompatibilityMode:    defaultCompatMode,
				Env:                  map[string]string{},
				TracesOutput:         null.NewString("none", false),
			},
		},
		"traces output from env": {
			useSysEnv: false,
			systemEnv: map[string]string{"K6_TRACES_OUTPUT": "foo"},
			expRTOpts: lib.RuntimeOptions{
				IncludeSystemEnvVars: null.NewBool(false, false),
				CompatibilityMode:    defaultCompatMode,
				Env:                  map[string]string{},
				TracesOutput:         null.NewString("foo", true),
			},
		},
		"traces output from env overwritten by CLI": {
			useSysEnv: false,
			systemEnv: map[string]string{"K6_TRACES_OUTPUT": "foo"},
			cliFlags:  []string{"--traces-output", "bar"},
			expRTOpts: lib.RuntimeOptions{
				IncludeSystemEnvVars: null.NewBool(false, false),
				CompatibilityMode:    defaultCompatMode,
				Env:                  map[string]string{},
				TracesOutput:         null.NewString("bar", true),
			},
		},
	}
	for name, tc := range runtimeOptionsTestCases {
		tc := tc
		t.Run(fmt.Sprintf("RuntimeOptions test '%s'", name), func(t *testing.T) {
			t.Parallel()
			testRuntimeOptionsCase(t, tc)
		})
	}
}
