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
	"sync"
	"testing"
	"time"

	"github.com/loadimpact/k6/lib/scheduler"
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

// A simple logrus hook that could be used to check if log messages were outputted
type simpleLogrusHook struct {
	mutex        sync.Mutex
	levels       []log.Level
	messageCache []log.Entry
}

func (smh *simpleLogrusHook) Levels() []log.Level {
	return smh.levels
}
func (smh *simpleLogrusHook) Fire(e *log.Entry) error {
	smh.mutex.Lock()
	defer smh.mutex.Unlock()
	smh.messageCache = append(smh.messageCache, *e)
	return nil
}

func (smh *simpleLogrusHook) drain() []log.Entry {
	smh.mutex.Lock()
	defer smh.mutex.Unlock()
	res := smh.messageCache
	smh.messageCache = []log.Entry{}
	return res
}

var _ log.Hook = &simpleLogrusHook{}

// A helper funcion for setting arbitrary environment variables and
// restoring the old ones at the end, usually by deferring the returned callback
//TODO: remove these hacks when we improve the configuration... we shouldn't
// have to mess with the global environment at all...
func setEnv(t *testing.T, newEnv []string) (restoreEnv func()) {
	actuallSetEnv := func(env []string) {
		os.Clearenv()
		for _, e := range env {
			val := ""
			pair := strings.SplitN(e, "=", 2)
			if len(pair) > 1 {
				val = pair[1]
			}
			require.NoError(t, os.Setenv(pair[0], val))
		}
	}
	oldEnv := os.Environ()
	actuallSetEnv(newEnv)

	return func() {
		actuallSetEnv(oldEnv)
	}
}

var verifyOneIterPerOneVU = func(t *testing.T, c Config) {
	// No config anywhere should result in a 1 VU with a 1 uninterruptible iteration config
	sched := c.Execution[lib.DefaultSchedulerName]
	require.NotEmpty(t, sched)
	require.IsType(t, scheduler.PerVUIteationsConfig{}, sched)
	perVuIters, ok := sched.(scheduler.PerVUIteationsConfig)
	require.True(t, ok)
	assert.Equal(t, null.NewBool(false, false), perVuIters.Interruptible)
	assert.Equal(t, null.NewInt(1, false), perVuIters.Iterations)
	assert.Equal(t, null.NewInt(1, false), perVuIters.VUs)
}

var verifySharedIters = func(vus, iters int64) func(t *testing.T, c Config) {
	return func(t *testing.T, c Config) {
		sched := c.Execution[lib.DefaultSchedulerName]
		require.NotEmpty(t, sched)
		require.IsType(t, scheduler.SharedIteationsConfig{}, sched)
		sharedIterConfig, ok := sched.(scheduler.SharedIteationsConfig)
		require.True(t, ok)
		assert.Equal(t, null.NewInt(vus, true), sharedIterConfig.VUs)
		assert.Equal(t, null.NewInt(iters, true), sharedIterConfig.Iterations)
	}
}

var verifyConstantLoopingVUs = func(vus int64, duration time.Duration) func(t *testing.T, c Config) {
	return func(t *testing.T, c Config) {
		sched := c.Execution[lib.DefaultSchedulerName]
		require.NotEmpty(t, sched)
		require.IsType(t, scheduler.ConstantLoopingVUsConfig{}, sched)
		clvc, ok := sched.(scheduler.ConstantLoopingVUsConfig)
		require.True(t, ok)
		assert.Equal(t, null.NewBool(true, false), clvc.Interruptible)
		assert.Equal(t, null.NewInt(vus, true), clvc.VUs)
		assert.Equal(t, types.NullDurationFrom(duration), clvc.Duration)
	}
}

func mostFlagSets() []*pflag.FlagSet {
	// sigh... compromises...
	return []*pflag.FlagSet{runCmdFlagSet(), archiveCmdFlagSet(), cloudCmdFlagSet()}
}

// exp contains the different events or errors we expect our test case to trigger.
// for space and clarity, we use the fact that by default, all of the struct values are false
type exp struct {
	cliError           bool
	consolidationError bool
	validationErrors   bool
	logWarning         bool //TODO: remove in the next version?
}

// A hell of a complicated test case, that still doesn't test things fully...
type configConsolidationTestCase struct {
	cliFlagSets   []*pflag.FlagSet
	cliFlagValues []string
	env           []string
	runnerOptions *lib.Options
	//TODO: test the JSON config as well... after most of https://github.com/loadimpact/k6/issues/883#issuecomment-468646291 is fixed
	expected        exp
	customValidator func(t *testing.T, c Config)
}

var configConsolidationTestCases = []configConsolidationTestCase{
	// Check that no options will result in 1 VU 1 iter value for execution
	{mostFlagSets(), nil, nil, nil, exp{}, verifyOneIterPerOneVU},
	// Verify some CLI errors
	{mostFlagSets(), []string{"--blah", "blah"}, nil, nil, exp{cliError: true}, nil},
	{mostFlagSets(), []string{"--duration", "blah"}, nil, nil, exp{cliError: true}, nil},
	{mostFlagSets(), []string{"--iterations", "blah"}, nil, nil, exp{cliError: true}, nil},
	{mostFlagSets(), []string{"--execution", ""}, nil, nil, exp{cliError: true}, nil},
	{mostFlagSets(), []string{"--stage", "10:20s"}, nil, nil, exp{cliError: true}, nil},
	// Check if CLI shortcuts generate correct execution values
	{mostFlagSets(), []string{"--vus", "1", "--iterations", "5"}, nil, nil, exp{}, verifySharedIters(1, 5)},
	{mostFlagSets(), []string{"-u", "2", "-i", "6"}, nil, nil, exp{}, verifySharedIters(2, 6)},
	{mostFlagSets(), []string{"-u", "3", "-d", "30s"}, nil, nil, exp{}, verifyConstantLoopingVUs(3, 30*time.Second)},
	{mostFlagSets(), []string{"-u", "4", "--duration", "60s"}, nil, nil, exp{}, verifyConstantLoopingVUs(4, 1*time.Minute)},
	//TODO: verify stages
	// This should get a validation error since VUs are more than the shared iterations
	{mostFlagSets(), []string{"--vus", "10", "-i", "6"}, nil, nil, exp{validationErrors: true}, verifySharedIters(10, 6)},
	// These should emit a warning
	{mostFlagSets(), []string{"-u", "1", "-i", "6", "-d", "10s"}, nil, nil, exp{logWarning: true}, nil},
	{mostFlagSets(), []string{"-u", "2", "-d", "10s", "-s", "10s:20"}, nil, nil, exp{logWarning: true}, nil},
	{mostFlagSets(), []string{"-u", "3", "-i", "5", "-s", "10s:20"}, nil, nil, exp{logWarning: true}, nil},
	{mostFlagSets(), []string{"-u", "3", "-d", "0"}, nil, nil, exp{logWarning: true}, nil},
	// Test if environment variable shortcuts are working as expected
	{mostFlagSets(), nil, []string{"K6_VUS=5", "K6_ITERATIONS=15"}, nil, exp{}, verifySharedIters(5, 15)},
	{mostFlagSets(), nil, []string{"K6_VUS=10", "K6_DURATION=20s"}, nil, exp{}, verifyConstantLoopingVUs(10, 20*time.Second)},

	//TODO: test combinations between options and levels
	//TODO: test the future full overwriting of the duration/iterations/stages/execution options

	// Just in case, verify that no options will result in the same 1 vu 1 iter config
	{mostFlagSets(), nil, nil, nil, exp{}, verifyOneIterPerOneVU},
	//TODO: test for differences between flagsets
	//TODO: more tests in general...
}

func TestConfigConsolidation(t *testing.T) {
	logHook := simpleLogrusHook{levels: []log.Level{log.WarnLevel}}
	log.AddHook(&logHook)
	log.SetOutput(ioutil.Discard)
	defer log.SetOutput(os.Stderr)

	runTestCase := func(t *testing.T, testCase configConsolidationTestCase, flagSet *pflag.FlagSet) {
		logHook.drain()

		restoreEnv := setEnv(t, testCase.env)
		defer restoreEnv()

		//TODO: also remove these hacks when we improve the configuration...
		getTestCaseCliConf := func() (Config, error) {
			if err := flagSet.Parse(testCase.cliFlagValues); err != nil {
				return Config{}, err
			}
			if flagSet.Lookup("out") != nil {
				return getConfig(flagSet)
			}
			opts, errOpts := getOptions(flagSet)
			return Config{Options: opts}, errOpts
		}

		cliConf, err := getTestCaseCliConf()
		if testCase.expected.cliError {
			require.Error(t, err)
			return
		}
		require.NoError(t, err)

		var runner lib.Runner
		if testCase.runnerOptions != nil {
			runner = &lib.MiniRunner{Options: *testCase.runnerOptions}
		}
		fs := afero.NewMemMapFs() //TODO: test JSON configs as well!
		result, err := getConsolidatedConfig(fs, cliConf, runner)
		if testCase.expected.consolidationError {
			require.Error(t, err)
			return
		}
		require.NoError(t, err)

		warnings := logHook.drain()
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
		for fsNum, flagSet := range testCase.cliFlagSets {
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
