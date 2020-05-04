/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
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
	"fmt"
	"testing"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/stretchr/testify/assert"
	"gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/executor"
	"github.com/loadimpact/k6/lib/testutils"
	"github.com/loadimpact/k6/lib/types"
)

type testCmdData struct {
	Name  string
	Tests []testCmdTest
}

type testCmdTest struct {
	Args     []string
	Expected []string
	Name     string
}

func TestConfigCmd(t *testing.T) {
	testdata := []testCmdData{
		{
			Name: "Out",

			Tests: []testCmdTest{
				{
					Name:     "NoArgs",
					Args:     []string{""},
					Expected: []string{},
				},
				{
					Name:     "SingleArg",
					Args:     []string{"--out", "influxdb=http://localhost:8086/k6"},
					Expected: []string{"influxdb=http://localhost:8086/k6"},
				},
				{
					Name:     "MultiArg",
					Args:     []string{"--out", "influxdb=http://localhost:8086/k6", "--out", "json=test.json"},
					Expected: []string{"influxdb=http://localhost:8086/k6", "json=test.json"},
				},
			},
		},
	}

	for _, data := range testdata {
		t.Run(data.Name, func(t *testing.T) {
			for _, test := range data.Tests {
				t.Run(`"`+test.Name+`"`, func(t *testing.T) {
					fs := configFlagSet()
					fs.AddFlagSet(optionFlagSet())
					assert.NoError(t, fs.Parse(test.Args))

					config, err := getConfig(fs)
					assert.NoError(t, err)
					assert.Equal(t, test.Expected, config.Out)
				})
			}
		})
	}
}

func TestConfigEnv(t *testing.T) {
	testdata := map[struct{ Name, Key string }]map[string]func(Config){
		{"Linger", "K6_LINGER"}: {
			"":      func(c Config) { assert.Equal(t, null.Bool{}, c.Linger) },
			"true":  func(c Config) { assert.Equal(t, null.BoolFrom(true), c.Linger) },
			"false": func(c Config) { assert.Equal(t, null.BoolFrom(false), c.Linger) },
		},
		{"NoUsageReport", "K6_NO_USAGE_REPORT"}: {
			"":      func(c Config) { assert.Equal(t, null.Bool{}, c.NoUsageReport) },
			"true":  func(c Config) { assert.Equal(t, null.BoolFrom(true), c.NoUsageReport) },
			"false": func(c Config) { assert.Equal(t, null.BoolFrom(false), c.NoUsageReport) },
		},
		{"Out", "K6_OUT"}: {
			"":         func(c Config) { assert.Equal(t, []string{}, c.Out) },
			"influxdb": func(c Config) { assert.Equal(t, []string{"influxdb"}, c.Out) },
		},
	}
	for field, data := range testdata {
		field, data := field, data
		t.Run(field.Name, func(t *testing.T) {
			for value, fn := range data {
				value, fn := value, fn
				t.Run(`"`+value+`"`, func(t *testing.T) {
					restore := testutils.SetEnv(t, []string{fmt.Sprintf("%s=%s", field.Key, value)})
					defer restore()
					var config Config
					assert.NoError(t, envconfig.Process("", &config))
					fn(config)
				})
			}
		})
	}
}

func TestConfigApply(t *testing.T) {
	t.Run("Linger", func(t *testing.T) {
		conf := Config{}.Apply(Config{Linger: null.BoolFrom(true)})
		assert.Equal(t, null.BoolFrom(true), conf.Linger)
	})
	t.Run("NoUsageReport", func(t *testing.T) {
		conf := Config{}.Apply(Config{NoUsageReport: null.BoolFrom(true)})
		assert.Equal(t, null.BoolFrom(true), conf.NoUsageReport)
	})
	t.Run("Out", func(t *testing.T) {
		conf := Config{}.Apply(Config{Out: []string{"influxdb"}})
		assert.Equal(t, []string{"influxdb"}, conf.Out)

		conf = Config{}.Apply(Config{Out: []string{"influxdb", "json"}})
		assert.Equal(t, []string{"influxdb", "json"}, conf.Out)
	})
}

func TestDeriveAndValidateConfig(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		conf    Config
		exports map[string]struct{}
		err     string
	}{
		{"defaultOK", Config{}, map[string]struct{}{"default": {}}, ""},
		{"defaultErr", Config{}, map[string]struct{}{},
			"executor default: function 'default' not found in exports"},
		{"nonDefaultOK", Config{Options: lib.Options{Execution: lib.ExecutorConfigMap{
			"per_vu_iters": executor.PerVUIterationsConfig{BaseConfig: executor.BaseConfig{
				Name: "per_vu_iters", Type: "per-vu-iterations", Exec: null.StringFrom("nonDefault")},
				VUs:         null.IntFrom(1),
				Iterations:  null.IntFrom(1),
				MaxDuration: types.NullDurationFrom(time.Second),
			}}}},
			map[string]struct{}{"nonDefault": {}}, "",
		},
		{"nonDefaultErr", Config{Options: lib.Options{Execution: lib.ExecutorConfigMap{
			"per_vu_iters": executor.PerVUIterationsConfig{BaseConfig: executor.BaseConfig{
				Name: "per_vu_iters", Type: "per-vu-iterations", Exec: null.StringFrom("nonDefaultErr")},
				VUs:         null.IntFrom(1),
				Iterations:  null.IntFrom(1),
				MaxDuration: types.NullDurationFrom(time.Second),
			}}}},
			map[string]struct{}{"nonDefault": {}},
			"executor per_vu_iters: function 'nonDefaultErr' not found in exports",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, err := deriveAndValidateConfig(tc.conf, tc.exports)
			if tc.err != "" {
				assert.Contains(t, err.Error(), tc.err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
