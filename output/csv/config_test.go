/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
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

package csv

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus"

	"gopkg.in/guregu/null.v3"

	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/lib/types"
)

func TestNewConfig(t *testing.T) {
	config := NewConfig()
	assert.Equal(t, "file.csv", config.FileName.String)
	assert.Equal(t, "1s", config.SaveInterval.String())
	assert.Equal(t, "unix", config.TimeFormat.String)
}

func TestApply(t *testing.T) {
	configs := []Config{
		{
			FileName:     null.StringFrom(""),
			SaveInterval: types.NullDurationFrom(2 * time.Second),
			TimeFormat:   null.StringFrom("unix"),
		},
		{
			FileName:     null.StringFrom("newPath"),
			SaveInterval: types.NewNullDuration(time.Duration(1), false),
			TimeFormat:   null.StringFrom("rfc3399"),
		},
	}
	expected := []struct {
		FileName     string
		SaveInterval string
		TimeFormat   string
	}{
		{
			FileName:     "",
			SaveInterval: "2s",
			TimeFormat:   "unix",
		},
		{
			FileName:     "newPath",
			SaveInterval: "1s",
			TimeFormat:   "rfc3399",
		},
	}

	for i := range configs {
		config := configs[i]
		expected := expected[i]
		t.Run(expected.FileName+"_"+expected.SaveInterval, func(t *testing.T) {
			baseConfig := NewConfig()
			baseConfig = baseConfig.Apply(config)

			assert.Equal(t, expected.FileName, baseConfig.FileName.String)
			assert.Equal(t, expected.SaveInterval, baseConfig.SaveInterval.String())
			assert.Equal(t, expected.TimeFormat, baseConfig.TimeFormat.String)
		})
	}
}

func TestParseArg(t *testing.T) {
	cases := map[string]struct {
		config             Config
		expectedLogEntries []string
		expectedErr        bool
	}{
		"test_file.csv": {
			config: Config{
				FileName:     null.StringFrom("test_file.csv"),
				SaveInterval: types.NewNullDuration(1*time.Second, false),
				TimeFormat:   null.NewString("unix", false),
			},
		},
		"save_interval=5s": {
			config: Config{
				FileName:     null.NewString("file.csv", false),
				SaveInterval: types.NullDurationFrom(5 * time.Second),
				TimeFormat:   null.NewString("unix", false),
			},
			expectedLogEntries: []string{
				"CSV output argument 'save_interval' is deprecated, please use 'saveInterval' instead.",
			},
		},
		"saveInterval=5s": {
			config: Config{
				FileName:     null.NewString("file.csv", false),
				SaveInterval: types.NullDurationFrom(5 * time.Second),
				TimeFormat:   null.NewString("unix", false),
			},
		},
		"file_name=test.csv,save_interval=5s": {
			config: Config{
				FileName:     null.StringFrom("test.csv"),
				SaveInterval: types.NullDurationFrom(5 * time.Second),
				TimeFormat:   null.NewString("unix", false),
			},
			expectedLogEntries: []string{
				"CSV output argument 'file_name' is deprecated, please use 'fileName' instead.",
				"CSV output argument 'save_interval' is deprecated, please use 'saveInterval' instead.",
			},
		},
		"fileName=test.csv,save_interval=5s": {
			config: Config{
				FileName:     null.StringFrom("test.csv"),
				SaveInterval: types.NullDurationFrom(5 * time.Second),
				TimeFormat:   null.NewString("unix", false),
			},
			expectedLogEntries: []string{
				"CSV output argument 'save_interval' is deprecated, please use 'saveInterval' instead.",
			},
		},
		"filename=test.csv,save_interval=5s": {
			expectedErr: true,
		},
		"fileName=test.csv,timeFormat=rfc3399": {
			config: Config{
				FileName:     null.StringFrom("test.csv"),
				SaveInterval: types.NewNullDuration(1*time.Second, false),
				TimeFormat:   null.StringFrom("rfc3399"),
			},
		},
	}

	for arg, testCase := range cases {
		arg := arg
		testCase := testCase

		testLogger, hook := test.NewNullLogger()
		testLogger.SetOutput(testutils.NewTestOutput(t))

		t.Run(arg, func(t *testing.T) {
			config, err := ParseArg(arg, testLogger)

			if testCase.expectedErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, testCase.config, config)

			var entries []string
			for _, v := range hook.AllEntries() {
				assert.Equal(t, v.Level, logrus.WarnLevel)
				entries = append(entries, v.Message)
			}
			assert.ElementsMatch(t, entries, testCase.expectedLogEntries)
		})
	}
}
