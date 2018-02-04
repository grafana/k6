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
	"testing"

	"github.com/loadimpact/k6/lib"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

type EnvVarTest struct {
	name      string
	systemEnv map[string]string
	cliOpts   []string
	expErr    bool
	expEnv    map[string]string
}

var envVarTestCases = []EnvVarTest{
	{
		"empty env",
		map[string]string{},
		[]string{},
		false,
		map[string]string{},
	},
	{
		"disabled sys env",
		map[string]string{"test1": "val1"},
		[]string{"--no-system-env-vars"},
		false,
		map[string]string{},
	},
	{
		"only system env",
		map[string]string{"test1": "val1"},
		[]string{},
		false,
		map[string]string{"test1": "val1"},
	},
	{
		"mixed system and cli env",
		map[string]string{"test1": "val1", "test2": ""},
		[]string{"--env", "test3=val3", "-e", "test4", "-e", "test5="},
		false,
		map[string]string{"test1": "val1", "test2": "", "test3": "val3", "test4": "", "test5": ""},
	},
	{
		"disabled system env with cli params",
		map[string]string{"test1": "val1"},
		[]string{"-e", "test2=overwriten", "-e", "test2=val2", "--no-system-env-vars"},
		false,
		map[string]string{"test2": "val2"},
	},
	{
		"overwriting system env with cli param",
		map[string]string{"test1": "val1sys"},
		[]string{"--env", "test1=val1cli"},
		false,
		map[string]string{"test1": "val1cli"},
	},
	{
		"error invalid cli var name 1",
		map[string]string{},
		[]string{"--env", "test a=error"},
		true,
		map[string]string{},
	},
	{
		"error invalid cli var name 2",
		map[string]string{},
		[]string{"--env", "1var=error"},
		true,
		map[string]string{},
	},
	{
		"error invalid cli var name 3",
		map[string]string{},
		[]string{"--env", "уникод=unicode-disabled"},
		true,
		map[string]string{},
	},
	{
		"valid env vars with spaces",
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
			flags := runtimeOptionFlagSet()
			require.NoError(t, flags.Parse(tc.cliOpts))

			rtOpts, err := getRuntimeOptions(flags)
			if tc.expErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.EqualValues(t, tc.expEnv, rtOpts.Env)

			// Clear the env again so real system values don't accidentally polute the end-to-end test
			os.Clearenv()

			jsCode := "export default function() {\n"
			for key, val := range tc.expEnv {
				jsCode += fmt.Sprintf(
					"if (__ENV.%s !== '%s') { throw new Error('Invalid %s: ' + __ENV.%s); }\n",
					key, val, key, key,
				)
			}
			jsCode += "}"

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
			archive.Write(archiveBuf)

			_, err = newRunner(
				&lib.SourceData{
					Data:     []byte(archiveBuf.Bytes()),
					Filename: "/script.tar",
				},
				typeArchive,
				afero.NewOsFs(),
				lib.RuntimeOptions{}, // Empty runtime options!
			)
			require.NoError(t, err)

			//TODO: write test when the runner overwrites some env vars in the archive?
		})
	}
}
