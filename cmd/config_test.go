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
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/loadimpact/k6/lib/scheduler"
	"github.com/loadimpact/k6/lib/testutils"
	"github.com/loadimpact/k6/lib/types"

	"github.com/spf13/afero"
	"github.com/spf13/pflag"

	"github.com/kelseyhightower/envconfig"
	"github.com/loadimpact/k6/lib"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// A helper funcion for setting arbitrary environment variables and
// restoring the old ones at the end, usually by deferring the returned callback
//TODO: remove these hacks when we improve the configuration... we shouldn't
// have to mess with the global environment at all...
func setEnv(t *testing.T, newEnv []string) (restoreEnv func()) {
	actuallSetEnv := func(env []string, abortOnSetErr bool) {
		os.Clearenv()
		for _, e := range env {
			val := ""
			pair := strings.SplitN(e, "=", 2)
			if len(pair) > 1 {
				val = pair[1]
			}
			err := os.Setenv(pair[0], val)
			if abortOnSetErr {
				require.NoError(t, err)
			} else if err != nil {
				t.Logf(
					"Received a non-aborting but unexpected error '%s' when setting env.var '%s' to '%s'",
					err, pair[0], val,
				)
			}
		}
	}
	oldEnv := os.Environ()
	actuallSetEnv(newEnv, true)

	return func() {
		actuallSetEnv(oldEnv, false)
	}
}

func verifyOneIterPerOneVU(t *testing.T, c Config) {
	// No config anywhere should result in a 1 VU with a 1 uninterruptible iteration config
	sched := c.Execution[lib.DefaultSchedulerName]
	require.NotEmpty(t, sched)
	require.IsType(t, scheduler.PerVUIteationsConfig{}, sched)
	perVuIters, ok := sched.(scheduler.PerVUIteationsConfig)
	require.True(t, ok)
	assert.Equal(t, null.NewBool(false, false), perVuIters.Interruptible)
	assert.Equal(t, null.NewInt(1, false), perVuIters.Iterations)
	assert.Equal(t, null.NewInt(1, false), perVuIters.VUs)
	//TODO: verify shortcut options as well?
}

func verifySharedIters(vus, iters int64) func(t *testing.T, c Config) {
	return func(t *testing.T, c Config) {
		sched := c.Execution[lib.DefaultSchedulerName]
		require.NotEmpty(t, sched)
		require.IsType(t, scheduler.SharedIteationsConfig{}, sched)
		sharedIterConfig, ok := sched.(scheduler.SharedIteationsConfig)
		require.True(t, ok)
		assert.Equal(t, null.NewInt(vus, true), sharedIterConfig.VUs)
		assert.Equal(t, null.NewInt(iters, true), sharedIterConfig.Iterations)
		//TODO: verify shortcut options as well?
	}
}

func verifyConstantLoopingVUs(vus int64, duration time.Duration) func(t *testing.T, c Config) {
	return func(t *testing.T, c Config) {
		sched := c.Execution[lib.DefaultSchedulerName]
		require.NotEmpty(t, sched)
		require.IsType(t, scheduler.ConstantLoopingVUsConfig{}, sched)
		clvc, ok := sched.(scheduler.ConstantLoopingVUsConfig)
		require.True(t, ok)
		assert.Equal(t, null.NewBool(true, false), clvc.Interruptible)
		assert.Equal(t, null.NewInt(vus, true), clvc.VUs)
		assert.Equal(t, types.NullDurationFrom(duration), clvc.Duration)
		//TODO: verify shortcut options as well?
	}
}

func mostFlagSets() []*pflag.FlagSet {
	//TODO: make this unneccesary... currently these are the only commands in which
	// getConsolidatedConfig() is used, but they also have differences in their CLI flags :/
	// sigh... compromises...
	return []*pflag.FlagSet{runCmdFlagSet(), archiveCmdFlagSet(), cloudCmdFlagSet()}
}

type opts struct {
	cliFlagSets []*pflag.FlagSet
	cli         []string
	env         []string
	runner      *lib.Options
	//TODO: test the JSON config as well... after most of https://github.com/loadimpact/k6/issues/883#issuecomment-468646291 is fixed
}

// exp contains the different events or errors we expect our test case to trigger.
// for space and clarity, we use the fact that by default, all of the struct values are false
type exp struct {
	cliParseError      bool
	cliReadError       bool
	consolidationError bool
	validationErrors   bool
	logWarning         bool //TODO: remove in the next version?
}

// A hell of a complicated test case, that still doesn't test things fully...
type configConsolidationTestCase struct {
	options         opts
	expected        exp
	customValidator func(t *testing.T, c Config)
}

var configConsolidationTestCases = []configConsolidationTestCase{
	// Check that no options will result in 1 VU 1 iter value for execution
	{opts{}, exp{}, verifyOneIterPerOneVU},
	// Verify some CLI errors
	{opts{cli: []string{"--blah", "blah"}}, exp{cliParseError: true}, nil},
	{opts{cli: []string{"--duration", "blah"}}, exp{cliParseError: true}, nil},
	{opts{cli: []string{"--iterations", "blah"}}, exp{cliParseError: true}, nil},
	{opts{cli: []string{"--execution", ""}}, exp{cliParseError: true}, nil},
	{opts{cli: []string{"--stage", "10:20s"}}, exp{cliReadError: true}, nil},
	// Check if CLI shortcuts generate correct execution values
	{opts{cli: []string{"--vus", "1", "--iterations", "5"}}, exp{}, verifySharedIters(1, 5)},
	{opts{cli: []string{"-u", "2", "-i", "6"}}, exp{}, verifySharedIters(2, 6)},
	{opts{cli: []string{"-u", "3", "-d", "30s"}}, exp{}, verifyConstantLoopingVUs(3, 30*time.Second)},
	{opts{cli: []string{"-u", "4", "--duration", "60s"}}, exp{}, verifyConstantLoopingVUs(4, 1*time.Minute)},
	//TODO: verify stages
	// This should get a validation error since VUs are more than the shared iterations
	{opts{cli: []string{"--vus", "10", "-i", "6"}}, exp{validationErrors: true}, verifySharedIters(10, 6)},
	// These should emit a warning
	{opts{cli: []string{"-u", "1", "-i", "6", "-d", "10s"}}, exp{logWarning: true}, nil},
	{opts{cli: []string{"-u", "2", "-d", "10s", "-s", "10s:20"}}, exp{logWarning: true}, nil},
	{opts{cli: []string{"-u", "3", "-i", "5", "-s", "10s:20"}}, exp{logWarning: true}, nil},
	{opts{cli: []string{"-u", "3", "-d", "0"}}, exp{logWarning: true}, nil},
	// Test if environment variable shortcuts are working as expected
	{opts{env: []string{"K6_VUS=5", "K6_ITERATIONS=15"}}, exp{}, verifySharedIters(5, 15)},
	{opts{env: []string{"K6_VUS=10", "K6_DURATION=20s"}}, exp{}, verifyConstantLoopingVUs(10, 20*time.Second)},

	//TODO: test combinations between options and levels
	//TODO: test the future full overwriting of the duration/iterations/stages/execution options

	// Just in case, verify that no options will result in the same 1 vu 1 iter config
	{opts{}, exp{}, verifyOneIterPerOneVU},
	//TODO: test for differences between flagsets
	//TODO: more tests in general...
}

func TestConfigConsolidation(t *testing.T) {
	// This test and its subtests shouldn't be ran in parallel, since they unfortunately have
	// to mess with shared global objects (env vars, variables, the log, ... santa?)
	logHook := testutils.SimpleLogrusHook{HookedLevels: []log.Level{log.WarnLevel}}
	log.AddHook(&logHook)
	log.SetOutput(ioutil.Discard)
	defer log.SetOutput(os.Stderr)

	runTestCase := func(t *testing.T, testCase configConsolidationTestCase, flagSet *pflag.FlagSet) {
		t.Logf("Test with opts=%#v and exp=%#v\n", testCase.options, testCase.expected)
		logHook.Drain()

		restoreEnv := setEnv(t, testCase.options.env)
		defer restoreEnv()

		cliErr := flagSet.Parse(testCase.options.cli)
		if testCase.expected.cliParseError {
			require.Error(t, cliErr)
			return
		}
		require.NoError(t, cliErr)

		//TODO: remove these hacks when we improve the configuration...
		var cliConf Config
		if flagSet.Lookup("out") != nil {
			cliConf, cliErr = getConfig(flagSet)
		} else {
			opts, errOpts := getOptions(flagSet)
			cliConf, cliErr = Config{Options: opts}, errOpts
		}
		if testCase.expected.cliReadError {
			require.Error(t, cliErr)
			return
		}
		require.NoError(t, cliErr)

		var runner lib.Runner
		if testCase.options.runner != nil {
			runner = &lib.MiniRunner{Options: *testCase.options.runner}
		}
		fs := afero.NewMemMapFs() //TODO: test JSON configs as well!
		result, err := getConsolidatedConfig(fs, cliConf, runner)
		if testCase.expected.consolidationError {
			require.Error(t, err)
			return
		}
		require.NoError(t, err)

		warnings := logHook.Drain()
		if testCase.expected.logWarning {
			assert.NotEmpty(t, warnings)
		} else {
			assert.Empty(t, warnings)
		}

		validationErrors := result.Validate()
		if testCase.expected.validationErrors {
			assert.NotEmpty(t, validationErrors)
		} else {
			assert.Empty(t, validationErrors)
		}

		if testCase.customValidator != nil {
			testCase.customValidator(t, result)
		}
	}

	for tcNum, testCase := range configConsolidationTestCases {
		flagSets := testCase.options.cliFlagSets
		if flagSets == nil { // handle the most common case
			flagSets = mostFlagSets()
		}
		for fsNum, flagSet := range flagSets {
			// I want to paralelize this, but I cannot... due to global variables and other
			// questionable architectural choices... :|
			t.Run(
				fmt.Sprintf("TestCase#%d_FlagSet#%d", tcNum, fsNum),
				func(t *testing.T) { runTestCase(t, testCase, flagSet) },
			)
		}
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
			"":         func(c Config) { assert.Equal(t, []string{""}, c.Out) },
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
					assert.NoError(t, envconfig.Process("k6", &config))
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
