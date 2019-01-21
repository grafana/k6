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
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/loadimpact/k6/lib"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var envVars []string

func init() {
	envVars = os.Environ()
}

type EnvVarTest struct {
	name      string
	useSysEnv bool // Whether to include the system env vars by default (run) or not (cloud/archive/inspect)
	systemEnv map[string]string
	cliOpts   []string
	expErr    bool
	expEnv    map[string]string
}

var envVarTestCases = []EnvVarTest{
	{
		"empty env",
		true,
		map[string]string{},
		[]string{},
		false,
		map[string]string{},
	},
	{
		"disabled sys env by default",
		false,
		map[string]string{"test1": "val1"},
		[]string{},
		false,
		map[string]string{},
	},
	{
		"disabled sys env by cli 1",
		true,
		map[string]string{"test1": "val1"},
		[]string{"--include-system-env-vars=false"},
		false,
		map[string]string{},
	},
	{
		"disabled sys env by cli 2",
		true,
		map[string]string{"test1": "val1"},
		[]string{"--include-system-env-vars=0"},
		false,
		map[string]string{},
	},
	{
		"enabled sys env by default",
		true,
		map[string]string{"test1": "val1"},
		[]string{},
		false,
		map[string]string{"test1": "val1"},
	},
	{
		"enabled sys env by cli 1",
		false,
		map[string]string{"test1": "val1"},
		[]string{"--include-system-env-vars"},
		false,
		map[string]string{"test1": "val1"},
	},
	{
		"enabled sys env by cli 2",
		false,
		map[string]string{"test1": "val1"},
		[]string{"--include-system-env-vars=true"},
		false,
		map[string]string{"test1": "val1"},
	},
	{
		"run only system env",
		true,
		map[string]string{"test1": "val1"},
		[]string{},
		false,
		map[string]string{"test1": "val1"},
	},
	{
		"mixed system and cli env",
		true,
		map[string]string{"test1": "val1", "test2": ""},
		[]string{"--env", "test3=val3", "-e", "test4", "-e", "test5="},
		false,
		map[string]string{"test1": "val1", "test2": "", "test3": "val3", "test4": "", "test5": ""},
	},
	{
		"mixed system and cli env 2",
		false,
		map[string]string{"test1": "val1", "test2": ""},
		[]string{"--env", "test3=val3", "-e", "test4", "-e", "test5=", "--include-system-env-vars=1"},
		false,
		map[string]string{"test1": "val1", "test2": "", "test3": "val3", "test4": "", "test5": ""},
	},
	{
		"disabled system env with cli params",
		false,
		map[string]string{"test1": "val1"},
		[]string{"-e", "test2=overwriten", "-e", "test2=val2"},
		false,
		map[string]string{"test2": "val2"},
	},
	{
		"overwriting system env with cli param",
		true,
		map[string]string{"test1": "val1sys"},
		[]string{"--env", "test1=val1cli"},
		false,
		map[string]string{"test1": "val1cli"},
	},
	{
		"error invalid cli var name 1",
		true,
		map[string]string{},
		[]string{"--env", "test a=error"},
		true,
		map[string]string{},
	},
	{
		"error invalid cli var name 2",
		true,
		map[string]string{},
		[]string{"--env", "1var=error"},
		true,
		map[string]string{},
	},
	{
		"error invalid cli var name 3",
		true,
		map[string]string{},
		[]string{"--env", "уникод=unicode-disabled"},
		true,
		map[string]string{},
	},
	{
		"valid env vars with spaces",
		true,
		map[string]string{"test1": "value 1"},
		[]string{"--env", "test2=value 2"},
		false,
		map[string]string{"test1": "value 1", "test2": "value 2"},
	},
}

func TestEnvVars(t *testing.T) {
	for _, tc := range envVarTestCases {
		t.Run(fmt.Sprintf("EnvVar test '%s'", tc.name), func(t *testing.T) {
			os.Clearenv()
			for key, val := range tc.systemEnv {
				require.NoError(t, os.Setenv(key, val))
			}
			flags := runtimeOptionFlagSet(tc.useSysEnv)
			require.NoError(t, flags.Parse(tc.cliOpts))

			rtOpts, err := getRuntimeOptions(flags)
			if tc.expErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.EqualValues(t, tc.expEnv, rtOpts.Env)

			// Clear the env again so real system values don't accidentally pollute the end-to-end test
			os.Clearenv()

			jsCode := "export default function() {\n"
			for key, val := range tc.expEnv {
				jsCode += fmt.Sprintf(
					"if (__ENV.%s !== '%s') { throw new Error('Invalid %s: ' + __ENV.%s); }\n",
					key, val, key, key,
				)
			}
			jsCode += "}"

			// windows requires the environment variables to be loaded to gerenate the rand source
			if runtime.GOOS == "windows" {
				for _, e := range envVars {
					parts := strings.Split(e, "=")
					os.Setenv(parts[0], parts[1])
				}
			}

			runner, err := newRunner(
				&lib.SourceData{
					Data:     []byte(jsCode),
					Filename: "/script.js",
				},
				typeJS,
				afero.NewOsFs(),
				rtOpts,
			)
			require.NoError(t, err)

			archive := runner.MakeArchive()
			archiveBuf := &bytes.Buffer{}
			assert.NoError(t, archive.Write(archiveBuf))

			getRunnerErr := func(rtOpts lib.RuntimeOptions) (lib.Runner, error) {
				r, err := newRunner(
					&lib.SourceData{
						Data:     []byte(archiveBuf.Bytes()),
						Filename: "/script.tar",
					},
					typeArchive,
					afero.NewOsFs(),
					rtOpts,
				)
				return r, err
			}

			_, err = getRunnerErr(lib.RuntimeOptions{})
			require.NoError(t, err)
			for key, val := range tc.expEnv {
				r, err := getRunnerErr(lib.RuntimeOptions{Env: map[string]string{key: "almost " + val}})
				assert.NoError(t, err)
				assert.Equal(t, r.MakeArchive().Env[key], "almost "+val)
			}
		})
	}
}
