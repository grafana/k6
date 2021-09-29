/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2018 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package cmd

import (
	"bytes"
	"fmt"
	"net/url"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/metrics"
	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/loader"
)

type runtimeOptionsTestCase struct {
	useSysEnv bool // Whether to include the system env vars by default (run) or not (cloud/archive/inspect)
	expErr    bool
	cliFlags  []string
	systemEnv map[string]string
	expRTOpts lib.RuntimeOptions
}

//nolint:gochecknoglobals
var (
	defaultCompatMode  = null.NewString("extended", false)
	baseCompatMode     = null.NewString("base", true)
	extendedCompatMode = null.NewString("extended", true)
)

var runtimeOptionsTestCases = map[string]runtimeOptionsTestCase{ //nolint:gochecknoglobals
	"empty env": {
		useSysEnv: true,
		// everything else is empty
		expRTOpts: lib.RuntimeOptions{
			IncludeSystemEnvVars: null.NewBool(true, false),
			CompatibilityMode:    defaultCompatMode,
			Env:                  nil,
		},
	},
	"disabled sys env by default": {
		useSysEnv: false,
		systemEnv: map[string]string{"test1": "val1"},
		expRTOpts: lib.RuntimeOptions{
			IncludeSystemEnvVars: null.NewBool(false, false),
			CompatibilityMode:    defaultCompatMode,
			Env:                  map[string]string{},
		},
	},
	"disabled sys env by default with ext compat mode": {
		useSysEnv: false,
		systemEnv: map[string]string{"test1": "val1", "K6_COMPATIBILITY_MODE": "extended"},
		expRTOpts: lib.RuntimeOptions{
			IncludeSystemEnvVars: null.NewBool(false, false),
			CompatibilityMode:    extendedCompatMode,
			Env:                  map[string]string{},
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
		},
	},
	"disabled sys env by env": {
		useSysEnv: true,
		systemEnv: map[string]string{"K6_INCLUDE_SYSTEM_ENV_VARS": "false", "K6_COMPATIBILITY_MODE": "extended"},
		expRTOpts: lib.RuntimeOptions{
			IncludeSystemEnvVars: null.NewBool(false, true),
			CompatibilityMode:    extendedCompatMode,
			Env:                  map[string]string{},
		},
	},
	"enabled sys env by env": {
		useSysEnv: false,
		systemEnv: map[string]string{"K6_INCLUDE_SYSTEM_ENV_VARS": "true", "K6_COMPATIBILITY_MODE": "extended"},
		expRTOpts: lib.RuntimeOptions{
			IncludeSystemEnvVars: null.NewBool(true, true),
			CompatibilityMode:    extendedCompatMode,
			Env:                  map[string]string{"K6_INCLUDE_SYSTEM_ENV_VARS": "true", "K6_COMPATIBILITY_MODE": "extended"},
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
}

func testRuntimeOptionsCase(t *testing.T, tc runtimeOptionsTestCase) {
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

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/script.js", jsCode.Bytes(), 0o644))
	registry := metrics.NewRegistry()
	builtinMetrics := metrics.RegisterBuiltinMetrics(registry)
	runner, err := newRunner(
		testutils.NewLogger(t),
		&loader.SourceData{Data: jsCode.Bytes(), URL: &url.URL{Path: "/script.js", Scheme: "file"}},
		typeJS,
		map[string]afero.Fs{"file": fs},
		rtOpts,
		builtinMetrics,
		registry,
	)
	require.NoError(t, err)

	archive := runner.MakeArchive()
	archiveBuf := &bytes.Buffer{}
	require.NoError(t, archive.Write(archiveBuf))

	getRunnerErr := func(rtOpts lib.RuntimeOptions) (lib.Runner, error) {
		return newRunner(
			testutils.NewLogger(t),
			&loader.SourceData{
				Data: archiveBuf.Bytes(),
				URL:  &url.URL{Path: "/script.js"},
			},
			typeArchive,
			nil,
			rtOpts,
			builtinMetrics,
			registry,
		)
	}

	_, err = getRunnerErr(lib.RuntimeOptions{})
	require.NoError(t, err)
	for key, val := range tc.expRTOpts.Env {
		r, err := getRunnerErr(lib.RuntimeOptions{Env: map[string]string{key: "almost " + val}})
		assert.NoError(t, err)
		assert.Equal(t, r.MakeArchive().Env[key], "almost "+val)
	}
}

func TestRuntimeOptions(t *testing.T) {
	for name, tc := range runtimeOptionsTestCases {
		tc := tc
		t.Run(fmt.Sprintf("RuntimeOptions test '%s'", name), func(t *testing.T) {
			t.Parallel()
			testRuntimeOptionsCase(t, tc)
		})
	}
}
