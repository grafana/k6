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

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/testutils"
	"github.com/loadimpact/k6/loader"
)

type runtimeOptionsTestCase struct {
	useSysEnv     bool // Whether to include the system env vars by default (run) or not (cloud/archive/inspect)
	expErr        bool
	cliFlags      []string
	systemEnv     map[string]string
	expEnv        map[string]string
	expCompatMode null.String
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
		expCompatMode: defaultCompatMode,
	},
	"disabled sys env by default": {
		useSysEnv:     false,
		systemEnv:     map[string]string{"test1": "val1"},
		expEnv:        map[string]string{},
		expCompatMode: defaultCompatMode,
	},
	"disabled sys env by default with ext compat mode": {
		useSysEnv:     false,
		systemEnv:     map[string]string{"test1": "val1", "K6_COMPATIBILITY_MODE": "extended"},
		expEnv:        map[string]string{},
		expCompatMode: extendedCompatMode,
	},
	"disabled sys env by cli 1": {
		useSysEnv:     true,
		systemEnv:     map[string]string{"test1": "val1", "K6_COMPATIBILITY_MODE": "base"},
		cliFlags:      []string{"--include-system-env-vars=false"},
		expEnv:        map[string]string{},
		expCompatMode: baseCompatMode,
	},
	"disabled sys env by cli 2": {
		useSysEnv:     true,
		systemEnv:     map[string]string{"K6_INCLUDE_SYSTEM_ENV_VARS": "true", "K6_COMPATIBILITY_MODE": "extended"},
		cliFlags:      []string{"--include-system-env-vars=0", "--compatibility-mode=base"},
		expEnv:        map[string]string{},
		expCompatMode: baseCompatMode,
	},
	"disabled sys env by env": {
		useSysEnv:     true,
		systemEnv:     map[string]string{"K6_INCLUDE_SYSTEM_ENV_VARS": "false", "K6_COMPATIBILITY_MODE": "extended"},
		expEnv:        map[string]string{},
		expCompatMode: extendedCompatMode,
	},
	"enabled sys env by env": {
		useSysEnv:     false,
		systemEnv:     map[string]string{"K6_INCLUDE_SYSTEM_ENV_VARS": "true", "K6_COMPATIBILITY_MODE": "extended"},
		expEnv:        map[string]string{"K6_INCLUDE_SYSTEM_ENV_VARS": "true", "K6_COMPATIBILITY_MODE": "extended"},
		expCompatMode: extendedCompatMode,
	},
	"enabled sys env by default": {
		useSysEnv:     true,
		systemEnv:     map[string]string{"test1": "val1"},
		cliFlags:      []string{},
		expEnv:        map[string]string{"test1": "val1"},
		expCompatMode: defaultCompatMode,
	},
	"enabled sys env by cli 1": {
		useSysEnv:     false,
		systemEnv:     map[string]string{"test1": "val1"},
		cliFlags:      []string{"--include-system-env-vars"},
		expEnv:        map[string]string{"test1": "val1"},
		expCompatMode: defaultCompatMode,
	},
	"enabled sys env by cli 2": {
		useSysEnv:     false,
		systemEnv:     map[string]string{"test1": "val1"},
		cliFlags:      []string{"--include-system-env-vars=true"},
		expEnv:        map[string]string{"test1": "val1"},
		expCompatMode: defaultCompatMode,
	},
	"run only system env": {
		useSysEnv:     true,
		systemEnv:     map[string]string{"test1": "val1"},
		cliFlags:      []string{},
		expEnv:        map[string]string{"test1": "val1"},
		expCompatMode: defaultCompatMode,
	},
	"mixed system and cli env": {
		useSysEnv:     true,
		systemEnv:     map[string]string{"test1": "val1", "test2": ""},
		cliFlags:      []string{"--env", "test3=val3", "-e", "test4", "-e", "test5="},
		expEnv:        map[string]string{"test1": "val1", "test2": "", "test3": "val3", "test4": "", "test5": ""},
		expCompatMode: defaultCompatMode,
	},
	"mixed system and cli env 2": {
		useSysEnv:     false,
		systemEnv:     map[string]string{"test1": "val1", "test2": ""},
		cliFlags:      []string{"--env", "test3=val3", "-e", "test4", "-e", "test5=", "--include-system-env-vars=1"},
		expEnv:        map[string]string{"test1": "val1", "test2": "", "test3": "val3", "test4": "", "test5": ""},
		expCompatMode: defaultCompatMode,
	},
	"disabled system env with cli params": {
		useSysEnv:     false,
		systemEnv:     map[string]string{"test1": "val1"},
		cliFlags:      []string{"-e", "test2=overwriten", "-e", "test2=val2"},
		expEnv:        map[string]string{"test2": "val2"},
		expCompatMode: defaultCompatMode,
	},
	"overwriting system env with cli param": {
		useSysEnv:     true,
		systemEnv:     map[string]string{"test1": "val1sys"},
		cliFlags:      []string{"--env", "test1=val1cli"},
		expEnv:        map[string]string{"test1": "val1cli"},
		expCompatMode: defaultCompatMode,
	},
	"error wrong compat mode env var value": {
		systemEnv: map[string]string{"K6_COMPATIBILITY_MODE": "asdf"},
		expErr:    true,
	},
	"error wrong compat mode cli flag value": {
		cliFlags: []string{"--compatibility-mode", "whatever"},
		expErr:   true,
	},
	"error invalid cli var name 1": {
		useSysEnv:     true,
		systemEnv:     map[string]string{},
		cliFlags:      []string{"--env", "test a=error"},
		expErr:        true,
		expEnv:        map[string]string{},
		expCompatMode: defaultCompatMode,
	},
	"error invalid cli var name 2": {
		useSysEnv:     true,
		systemEnv:     map[string]string{},
		cliFlags:      []string{"--env", "1var=error"},
		expErr:        true,
		expEnv:        map[string]string{},
		expCompatMode: defaultCompatMode,
	},
	"error invalid cli var name 3": {
		useSysEnv:     true,
		systemEnv:     map[string]string{},
		cliFlags:      []string{"--env", "уникод=unicode-disabled"},
		expErr:        true,
		expEnv:        map[string]string{},
		expCompatMode: defaultCompatMode,
	},
	"valid env vars with spaces": {
		useSysEnv:     true,
		systemEnv:     map[string]string{"test1": "value 1"},
		cliFlags:      []string{"--env", "test2=value 2"},
		expEnv:        map[string]string{"test1": "value 1", "test2": "value 2"},
		expCompatMode: defaultCompatMode,
	},
	"valid env vars with special chars": {
		useSysEnv:     true,
		systemEnv:     map[string]string{"test1": "value 1"},
		cliFlags:      []string{"--env", "test2=value,2", "-e", `test3= ,  ,,, value, ,, 2!'@#,"`},
		expEnv:        map[string]string{"test1": "value 1", "test2": "value,2", "test3": ` ,  ,,, value, ,, 2!'@#,"`},
		expCompatMode: defaultCompatMode,
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
	require.EqualValues(t, tc.expEnv, rtOpts.Env)
	assert.Equal(t, tc.expCompatMode, rtOpts.CompatibilityMode)

	compatMode, err := lib.ValidateCompatibilityMode(rtOpts.CompatibilityMode.String)
	require.NoError(t, err)

	jsCode := new(bytes.Buffer)
	if compatMode == lib.CompatibilityModeExtended {
		fmt.Fprint(jsCode, "export default function() {")
	} else {
		fmt.Fprint(jsCode, "module.exports.default = function() {")
	}

	for key, val := range tc.expEnv {
		fmt.Fprintf(jsCode,
			"if (__ENV.%s !== `%s`) { throw new Error('Invalid %s: ' + __ENV.%s); }",
			key, val, key, key,
		)
	}
	fmt.Fprint(jsCode, "}")

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/script.js", jsCode.Bytes(), 0o644))
	runner, err := newRunner(
		testutils.NewLogger(t),
		&loader.SourceData{Data: jsCode.Bytes(), URL: &url.URL{Path: "/script.js", Scheme: "file"}},
		typeJS,
		map[string]afero.Fs{"file": fs},
		rtOpts,
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
		)
	}

	_, err = getRunnerErr(lib.RuntimeOptions{})
	require.NoError(t, err)
	for key, val := range tc.expEnv {
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
