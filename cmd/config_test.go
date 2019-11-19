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
	"os"
	"testing"

	"github.com/kelseyhightower/envconfig"
	"github.com/stretchr/testify/assert"
	"gopkg.in/guregu/null.v3"
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
		os.Clearenv()
		t.Run(field.Name, func(t *testing.T) {
			for value, fn := range data {
				t.Run(`"`+value+`"`, func(t *testing.T) {
					assert.NoError(t, os.Setenv(field.Key, value))
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
