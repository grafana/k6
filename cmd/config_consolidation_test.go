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
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	null "gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/scheduler"
	"github.com/loadimpact/k6/lib/testutils"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/stats"
)

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
	assert.Equal(t, null.NewInt(1, false), perVuIters.Iterations)
	assert.Equal(t, null.NewInt(1, false), perVuIters.VUs)
}

func verifySharedIters(vus, iters null.Int) func(t *testing.T, c Config) {
	return func(t *testing.T, c Config) {
		sched := c.Execution[lib.DefaultSchedulerName]
		require.NotEmpty(t, sched)
		require.IsType(t, scheduler.SharedIteationsConfig{}, sched)
		sharedIterConfig, ok := sched.(scheduler.SharedIteationsConfig)
		require.True(t, ok)
		assert.Equal(t, vus, sharedIterConfig.VUs)
		assert.Equal(t, iters, sharedIterConfig.Iterations)
		assert.Equal(t, vus, c.VUs)
		assert.Equal(t, iters, c.Iterations)
	}
}

func verifyConstLoopingVUs(vus null.Int, duration time.Duration) func(t *testing.T, c Config) {
	return func(t *testing.T, c Config) {
		sched := c.Execution[lib.DefaultSchedulerName]
		require.NotEmpty(t, sched)
		require.IsType(t, scheduler.ConstantLoopingVUsConfig{}, sched)
		clvc, ok := sched.(scheduler.ConstantLoopingVUsConfig)
		require.True(t, ok)
		assert.Equal(t, vus, clvc.VUs)
		assert.Equal(t, types.NullDurationFrom(duration), clvc.Duration)
		assert.Equal(t, vus, c.VUs)
		assert.Equal(t, types.NullDurationFrom(duration), c.Duration)
	}
}

func verifyVarLoopingVUs(startVus null.Int, stages []scheduler.Stage) func(t *testing.T, c Config) {
	return func(t *testing.T, c Config) {
		sched := c.Execution[lib.DefaultSchedulerName]
		require.NotEmpty(t, sched)
		require.IsType(t, scheduler.VariableLoopingVUsConfig{}, sched)
		clvc, ok := sched.(scheduler.VariableLoopingVUsConfig)
		require.True(t, ok)
		assert.Equal(t, startVus, clvc.StartVUs)
		assert.Equal(t, startVus, c.VUs)
		assert.Equal(t, stages, clvc.Stages)
		assert.Len(t, c.Stages, len(stages))
		for i, s := range stages {
			assert.Equal(t, s.Duration, c.Stages[i].Duration)
			assert.Equal(t, s.Target, c.Stages[i].Target)
		}
	}
}

// A helper function that accepts (duration in second, VUs) pairs and returns
// a valid slice of stage structs
func buildStages(durationsAndVUs ...int64) []scheduler.Stage {
	l := len(durationsAndVUs)
	if l%2 != 0 {
		panic("wrong len")
	}
	result := make([]scheduler.Stage, 0, l/2)
	for i := 0; i < l; i += 2 {
		result = append(result, scheduler.Stage{
			Duration: types.NullDurationFrom(time.Duration(durationsAndVUs[i]) * time.Second),
			Target:   null.IntFrom(durationsAndVUs[i+1]),
		})
	}
	return result
}

func mostFlagSets() []flagSetInit {
	//TODO: make this unnecessary... currently these are the only commands in which
	// getConsolidatedConfig() is used, but they also have differences in their CLI flags :/
	// sigh... compromises...
	result := []flagSetInit{}
	for i, fsi := range []flagSetInit{runCmdFlagSet, archiveCmdFlagSet, cloudCmdFlagSet} {
		i, fsi := i, fsi // go...
		result = append(result, func() *pflag.FlagSet {
			flags := pflag.NewFlagSet(fmt.Sprintf("superContrivedFlags_%d", i), pflag.ContinueOnError)
			flags.AddFlagSet(rootCmdPersistentFlagSet())
			flags.AddFlagSet(fsi())
			return flags
		})
	}
	return result
}

type file struct {
	filepath, contents string
}

func getFS(files []file) afero.Fs {
	fs := afero.NewMemMapFs()
	for _, f := range files {
		must(afero.WriteFile(fs, f.filepath, []byte(f.contents), 0644)) // modes don't matter in the afero.MemMapFs
	}
	return fs
}

func defaultConfig(jsonConfig string) afero.Fs {
	return getFS([]file{{defaultConfigFilePath, jsonConfig}})
}

type flagSetInit func() *pflag.FlagSet

type opts struct {
	cli    []string
	env    []string
	runner *lib.Options
	fs     afero.Fs

	//TODO: remove this when the configuration is more reproducible and sane...
	// We use a func, because initializing a FlagSet that points to variables
	// actually will change those variables to their default values :| In our
	// case, this happens only some of the time, for global variables that
	// are configurable only via CLI flags, but not environment variables.
	//
	// For the rest, their default value is their current value, since that
	// has been set from the environment variable. That has a bunch of other
	// issues on its own, and the func() doesn't help at all, and we need to
	// use the resetStickyGlobalVars() hack on top of that...
	cliFlagSetInits []flagSetInit
}

func resetStickyGlobalVars() {
	//TODO: remove after fixing the config, obviously a dirty hack
	exitOnRunning = false
	configFilePath = ""
	runType = ""
	runNoSetup = false
	runNoTeardown = false
}

// Something that makes the test also be a valid io.Writer, useful for passing it
// as an output for logs and CLI flag help messages...
type testOutput struct{ *testing.T }

func (to testOutput) Write(p []byte) (n int, err error) {
	to.Logf("%s", p)
	return len(p), nil
}

var _ io.Writer = testOutput{}

// exp contains the different events or errors we expect our test case to trigger.
// for space and clarity, we use the fact that by default, all of the struct values are false
type exp struct {
	cliParseError      bool
	cliReadError       bool
	consolidationError bool
	derivationError    bool
	validationErrors   bool
	logWarning         bool //TODO: remove in the next version?
}

// A hell of a complicated test case, that still doesn't test things fully...
type configConsolidationTestCase struct {
	options         opts
	expected        exp
	customValidator func(t *testing.T, c Config)
}

func getConfigConsolidationTestCases() []configConsolidationTestCase {
	I := null.IntFrom // shortcut for "Valid" (i.e. user-specified) ints
	// This is a function, because some of these test cases actually need for the init() functions
	// to be executed, since they depend on defaultConfigFilePath
	return []configConsolidationTestCase{
		// Check that no options will result in 1 VU 1 iter value for execution
		{opts{}, exp{}, verifyOneIterPerOneVU},
		// Verify some CLI errors
		{opts{cli: []string{"--blah", "blah"}}, exp{cliParseError: true}, nil},
		{opts{cli: []string{"--duration", "blah"}}, exp{cliParseError: true}, nil},
		{opts{cli: []string{"--iterations", "blah"}}, exp{cliParseError: true}, nil},
		{opts{cli: []string{"--execution", ""}}, exp{cliParseError: true}, nil},
		{opts{cli: []string{"--stage", "10:20s"}}, exp{cliReadError: true}, nil},
		// Check if CLI shortcuts generate correct execution values
		{opts{cli: []string{"--vus", "1", "--iterations", "5"}}, exp{}, verifySharedIters(I(1), I(5))},
		{opts{cli: []string{"-u", "2", "-i", "6"}}, exp{}, verifySharedIters(I(2), I(6))},
		{opts{cli: []string{"-d", "123s"}}, exp{}, verifyConstLoopingVUs(null.NewInt(1, false), 123*time.Second)},
		{opts{cli: []string{"-u", "3", "-d", "30s"}}, exp{}, verifyConstLoopingVUs(I(3), 30*time.Second)},
		{opts{cli: []string{"-u", "4", "--duration", "60s"}}, exp{}, verifyConstLoopingVUs(I(4), 1*time.Minute)},
		{
			opts{cli: []string{"--stage", "20s:10", "-s", "3m:5"}}, exp{},
			verifyVarLoopingVUs(null.NewInt(1, false), buildStages(20, 10, 180, 5)),
		},
		{
			opts{cli: []string{"-s", "1m6s:5", "--vus", "10"}}, exp{},
			verifyVarLoopingVUs(null.NewInt(10, true), buildStages(66, 5)),
		},
		{opts{cli: []string{"-u", "1", "-i", "6", "-d", "10s"}}, exp{}, func(t *testing.T, c Config) {
			verifySharedIters(I(1), I(6))(t, c)
			sharedIterConfig := c.Execution[lib.DefaultSchedulerName].(scheduler.SharedIteationsConfig)
			assert.Equal(t, time.Duration(sharedIterConfig.MaxDuration.Duration), 10*time.Second)
		}},
		// This should get a validation error since VUs are more than the shared iterations
		{opts{cli: []string{"--vus", "10", "-i", "6"}}, exp{validationErrors: true}, verifySharedIters(I(10), I(6))},
		{opts{cli: []string{"-s", "10s:5", "-s", "10s:"}}, exp{validationErrors: true}, nil},
		{opts{fs: defaultConfig(`{"stages": [{"duration": "20s"}], "vus": 10}`)}, exp{validationErrors: true}, nil},
		// These should emit a warning
		//TODO: in next version, those should be an error
		{opts{cli: []string{"-u", "2", "-d", "10s", "-s", "10s:20"}}, exp{logWarning: true}, nil},
		{opts{cli: []string{"-u", "3", "-i", "5", "-s", "10s:20"}}, exp{logWarning: true}, nil},
		{opts{cli: []string{"-u", "3", "-d", "0"}}, exp{logWarning: true}, nil},
		{
			opts{runner: &lib.Options{
				VUs:      null.IntFrom(5),
				Duration: types.NullDurationFrom(44 * time.Second),
				Stages: []lib.Stage{
					{Duration: types.NullDurationFrom(3 * time.Second), Target: I(20)},
				},
			}}, exp{logWarning: true}, nil,
		},
		{opts{fs: defaultConfig(`{"execution": {}}`)}, exp{logWarning: true}, verifyOneIterPerOneVU},
		// Test if environment variable shortcuts are working as expected
		{opts{env: []string{"K6_VUS=5", "K6_ITERATIONS=15"}}, exp{}, verifySharedIters(I(5), I(15))},
		{opts{env: []string{"K6_VUS=10", "K6_DURATION=20s"}}, exp{}, verifyConstLoopingVUs(I(10), 20*time.Second)},
		{
			opts{env: []string{"K6_STAGES=2m30s:11,1h1m:100"}}, exp{},
			verifyVarLoopingVUs(null.NewInt(1, false), buildStages(150, 11, 3660, 100)),
		},
		{
			opts{env: []string{"K6_STAGES=100s:100,0m30s:0", "K6_VUS=0"}}, exp{},
			verifyVarLoopingVUs(null.NewInt(0, true), buildStages(100, 100, 30, 0)),
		},
		// Test if JSON configs work as expected
		{opts{fs: defaultConfig(`{"iterations": 77, "vus": 7}`)}, exp{}, verifySharedIters(I(7), I(77))},
		{opts{fs: defaultConfig(`wrong-json`)}, exp{consolidationError: true}, nil},
		{opts{fs: getFS(nil), cli: []string{"--config", "/my/config.file"}}, exp{consolidationError: true}, nil},

		// Test combinations between options and levels
		{
			opts{
				fs:  getFS([]file{{"/my/config.file", `{"vus": 8, "duration": "2m"}`}}),
				cli: []string{"--config", "/my/config.file"},
			}, exp{}, verifyConstLoopingVUs(I(8), 120*time.Second),
		},
		{
			opts{
				fs:  defaultConfig(`{"stages": [{"duration": "20s", "target": 20}], "vus": 10}`),
				env: []string{"K6_DURATION=15s"},
				cli: []string{"--stage", ""},
			},
			exp{}, verifyConstLoopingVUs(I(10), 15*time.Second),
		},
		{
			opts{
				runner: &lib.Options{VUs: null.IntFrom(5), Duration: types.NullDurationFrom(50 * time.Second)},
				cli:    []string{"--stage", "5s:5"},
			},
			//TODO: this shouldn't be a warning in the next version, but the result will be different
			exp{logWarning: true}, verifyConstLoopingVUs(I(5), 50*time.Second),
		},
		{
			opts{
				fs:     defaultConfig(`{"stages": [{"duration": "20s", "target": 10}]}`),
				runner: &lib.Options{VUs: null.IntFrom(5)},
			},
			exp{},
			verifyVarLoopingVUs(null.NewInt(5, true), buildStages(20, 10)),
		},
		{
			opts{
				fs:     defaultConfig(`{"stages": [{"duration": "20s", "target": 10}]}`),
				runner: &lib.Options{VUs: null.IntFrom(5)},
				env:    []string{"K6_VUS=15", "K6_ITERATIONS=15"},
			},
			exp{logWarning: true}, //TODO: this won't be a warning in the next version, but the result will be different
			verifySharedIters(I(15), I(15)),
		},
		{
			opts{
				fs:     defaultConfig(`{"stages": [{"duration": "11s", "target": 11}]}`),
				runner: &lib.Options{VUs: null.IntFrom(22)},
				env:    []string{"K6_VUS=33"},
				cli:    []string{"--stage", "44s:44", "-s", "55s:55"},
			},
			exp{},
			verifyVarLoopingVUs(null.NewInt(33, true), buildStages(44, 44, 55, 55)),
		},

		//TODO: test the future full overwriting of the duration/iterations/stages/execution options
		{
			opts{
				fs: defaultConfig(`{
					"execution": { "someKey": {
						"type": "constant-looping-vus", "vus": 10, "duration": "60s", "interruptible": false,
						"iterationTimeout": "10s", "startTime": "70s", "env": {"test": "mest"}, "exec": "someFunc"
					}}}`),
				env: []string{"K6_ITERATIONS=25"},
				cli: []string{"--vus", "12"},
			},
			exp{}, verifySharedIters(I(12), I(25)),
		},
		{
			opts{
				fs: defaultConfig(`
					{
						"execution": {
							"default": {
								"type": "constant-looping-vus",
								"vus": 10,
								"duration": "60s"
							}
						},
						"vus": 10,
						"duration": "60s"
					}`,
				),
			},
			exp{}, verifyConstLoopingVUs(I(10), 60*time.Second),
		},
		// Just in case, verify that no options will result in the same 1 vu 1 iter config
		{opts{}, exp{}, verifyOneIterPerOneVU},

		// Test system tags
		{opts{}, exp{}, func(t *testing.T, c Config) {
			assert.Equal(t, stats.ToSystemTagSet(stats.DefaultSystemTagList), c.Options.SystemTags)
		}},
		{opts{cli: []string{"--system-tags", `""`}}, exp{}, func(t *testing.T, c Config) {
			assert.Equal(t, stats.SystemTagSet(0), *c.Options.SystemTags)
		}},
		{
			opts{runner: &lib.Options{
				SystemTags: stats.ToSystemTagSet([]string{stats.TagSubProto.String(), stats.TagURL.String()})},
			},
			exp{},
			func(t *testing.T, c Config) {
				assert.Equal(
					t,
					*stats.ToSystemTagSet([]string{stats.TagSubProto.String(), stats.TagURL.String()}),
					*c.Options.SystemTags,
				)
			},
		},
		//TODO: test for differences between flagsets
		//TODO: more tests in general, especially ones not related to execution parameters...
	}
}

func runTestCase(
	t *testing.T,
	testCase configConsolidationTestCase,
	newFlagSet flagSetInit,
	logHook *testutils.SimpleLogrusHook,
) {
	t.Logf("Test with opts=%#v and exp=%#v\n", testCase.options, testCase.expected)
	logrus.SetOutput(testOutput{t})
	logHook.Drain()

	restoreEnv := setEnv(t, testCase.options.env)
	defer restoreEnv()

	flagSet := newFlagSet()
	defer resetStickyGlobalVars()
	flagSet.SetOutput(testOutput{t})
	//flagSet.PrintDefaults()

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
	if testCase.options.fs == nil {
		t.Logf("Creating an empty FS for this test")
		testCase.options.fs = afero.NewMemMapFs() // create an empty FS if it wasn't supplied
	}

	consolidatedConfig, err := getConsolidatedConfig(testCase.options.fs, cliConf, runner)
	if testCase.expected.consolidationError {
		require.Error(t, err)
		return
	}
	require.NoError(t, err)

	derivedConfig, err := deriveExecutionConfig(consolidatedConfig)
	if testCase.expected.derivationError {
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

	validationErrors := derivedConfig.Validate()
	if testCase.expected.validationErrors {
		assert.NotEmpty(t, validationErrors)
	} else {
		assert.Empty(t, validationErrors)
	}

	if testCase.customValidator != nil {
		testCase.customValidator(t, derivedConfig)
	}
}

func TestConfigConsolidation(t *testing.T) {
	// This test and its subtests shouldn't be ran in parallel, since they unfortunately have
	// to mess with shared global objects (env vars, variables, the log, ... santa?)
	logHook := testutils.SimpleLogrusHook{HookedLevels: []logrus.Level{logrus.WarnLevel}}
	logrus.AddHook(&logHook)
	logrus.SetOutput(ioutil.Discard)
	defer logrus.SetOutput(os.Stderr)

	for tcNum, testCase := range getConfigConsolidationTestCases() {
		flagSetInits := testCase.options.cliFlagSetInits
		if flagSetInits == nil { // handle the most common case
			flagSetInits = mostFlagSets()
		}
		for fsNum, flagSet := range flagSetInits {
			// I want to paralelize this, but I cannot... due to global variables and other
			// questionable architectural choices... :|
			testCase, flagSet := testCase, flagSet
			t.Run(
				fmt.Sprintf("TestCase#%d_FlagSet#%d", tcNum, fsNum),
				func(t *testing.T) { runTestCase(t, testCase, flagSet, &logHook) },
			)
		}
	}
}
