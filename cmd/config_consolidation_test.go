package cmd

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/executor"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/metrics"
)

func verifyOneIterPerOneVU(t *testing.T, c Config) {
	// No config anywhere should result in a 1 VU with a 1 iteration config
	exec := c.Scenarios[lib.DefaultScenarioName]
	require.NotEmpty(t, exec)
	require.IsType(t, executor.PerVUIterationsConfig{}, exec)
	perVuIters, ok := exec.(executor.PerVUIterationsConfig)
	require.True(t, ok)
	assert.Equal(t, null.NewInt(1, false), perVuIters.Iterations)
	assert.Equal(t, null.NewInt(1, false), perVuIters.VUs)
}

func verifySharedIters(vus, iters null.Int) func(t *testing.T, c Config) {
	return func(t *testing.T, c Config) {
		exec := c.Scenarios[lib.DefaultScenarioName]
		require.NotEmpty(t, exec)
		require.IsType(t, executor.SharedIterationsConfig{}, exec)
		sharedIterConfig, ok := exec.(executor.SharedIterationsConfig)
		require.True(t, ok)
		assert.Equal(t, vus, sharedIterConfig.VUs)
		assert.Equal(t, iters, sharedIterConfig.Iterations)
		assert.Equal(t, vus, c.VUs)
		assert.Equal(t, iters, c.Iterations)
	}
}

func verifyConstLoopingVUs(vus null.Int, duration time.Duration) func(t *testing.T, c Config) {
	return func(t *testing.T, c Config) {
		exec := c.Scenarios[lib.DefaultScenarioName]
		require.NotEmpty(t, exec)
		require.IsType(t, executor.ConstantVUsConfig{}, exec)
		clvc, ok := exec.(executor.ConstantVUsConfig)
		require.True(t, ok)
		assert.Equal(t, vus, clvc.VUs)
		assert.Equal(t, types.NullDurationFrom(duration), clvc.Duration)
		assert.Equal(t, vus, c.VUs)
		assert.Equal(t, types.NullDurationFrom(duration), c.Duration)
	}
}

func verifyExternallyExecuted(scenarioName string, vus null.Int, duration time.Duration) func(t *testing.T, c Config) {
	return func(t *testing.T, c Config) {
		exec := c.Scenarios[scenarioName]
		require.NotEmpty(t, exec)
		require.IsType(t, executor.ExternallyControlledConfig{}, exec)
		ecc, ok := exec.(executor.ExternallyControlledConfig)
		require.True(t, ok)
		assert.Equal(t, vus, ecc.VUs)
		assert.Equal(t, types.NullDurationFrom(duration), ecc.Duration)
		assert.Equal(t, vus, ecc.MaxVUs) // MaxVUs defaults to VUs unless specified
	}
}

func verifyRampingVUs(startVus null.Int, stages []executor.Stage) func(t *testing.T, c Config) {
	return func(t *testing.T, c Config) {
		exec := c.Scenarios[lib.DefaultScenarioName]
		require.NotEmpty(t, exec)
		require.IsType(t, executor.RampingVUsConfig{}, exec)
		clvc, ok := exec.(executor.RampingVUsConfig)
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
func buildStages(durationsAndVUs ...int64) []executor.Stage {
	l := len(durationsAndVUs)
	if l%2 != 0 {
		panic("wrong len")
	}
	result := make([]executor.Stage, 0, l/2)
	for i := 0; i < l; i += 2 {
		result = append(result, executor.Stage{
			Duration: types.NullDurationFrom(time.Duration(durationsAndVUs[i]) * time.Second),
			Target:   null.IntFrom(durationsAndVUs[i+1]),
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
		must(afero.WriteFile(fs, f.filepath, []byte(f.contents), 0o644)) // modes don't matter in the afero.MemMapFs
	}
	return fs
}

type opts struct {
	cli    []string
	env    []string
	runner *lib.Options
	fs     afero.Fs
	cmds   []string
}

// exp contains the different events or errors we expect our test case to trigger.
// for space and clarity, we use the fact that by default, all of the struct values are false
type exp struct {
	cliParseError      bool
	cliReadError       bool
	consolidationError bool // Note: consolidationError includes validation errors from envconfig.Process()
	derivationError    bool
	validationErrors   bool
	logWarning         bool
}

// A hell of a complicated test case, that still doesn't test things fully...
type configConsolidationTestCase struct {
	options         opts
	expected        exp
	customValidator func(t *testing.T, c Config)
}

func getConfigConsolidationTestCases() []configConsolidationTestCase {
	defaultConfig := func(jsonConfig string) afero.Fs {
		return getFS([]file{{
			filepath.Join(".config", "loadimpact", "k6", defaultConfigFileName), // TODO: improve
			jsonConfig,
		}})
	}
	I := null.IntFrom // shortcut for "Valid" (i.e. user-specified) ints
	// This is a function, because some of these test cases actually need for the init() functions
	// to be executed, since they depend on defaultConfigFilePath
	return []configConsolidationTestCase{
		// Check that no options will result in 1 VU 1 iter value for execution
		{opts{}, exp{}, verifyOneIterPerOneVU},
		// Verify some CLI errors
		{opts{cli: []string{"--blah", "blah"}}, exp{cliParseError: true}, nil},
		{opts{cli: []string{"--duration", "blah"}}, exp{cliParseError: true}, nil},
		{opts{cli: []string{"--duration", "1000"}}, exp{cliParseError: true}, nil}, // intentionally unsupported
		{opts{cli: []string{"--iterations", "blah"}}, exp{cliParseError: true}, nil},
		{opts{cli: []string{"--execution", ""}}, exp{cliParseError: true}, nil},
		{opts{cli: []string{"--stage", "10:20s"}}, exp{cliReadError: true}, nil},
		{opts{cli: []string{"--stage", "1000:20"}}, exp{cliReadError: true}, nil}, // intentionally unsupported
		// Check if CLI shortcuts generate correct execution values
		{opts{cli: []string{"--vus", "1", "--iterations", "5"}}, exp{}, verifySharedIters(I(1), I(5))},
		{opts{cli: []string{"-u", "2", "-i", "6"}}, exp{}, verifySharedIters(I(2), I(6))},
		{opts{cli: []string{"-d", "123s"}}, exp{}, verifyConstLoopingVUs(null.NewInt(1, false), 123*time.Second)},
		{opts{cli: []string{"-u", "3", "-d", "30s"}}, exp{}, verifyConstLoopingVUs(I(3), 30*time.Second)},
		{opts{cli: []string{"-u", "4", "--duration", "60s"}}, exp{}, verifyConstLoopingVUs(I(4), 1*time.Minute)},
		{
			opts{cli: []string{"--stage", "20s:10", "-s", "3m:5"}},
			exp{},
			verifyRampingVUs(null.NewInt(1, false), buildStages(20, 10, 180, 5)),
		},
		{
			opts{cli: []string{"-s", "1m6s:5", "--vus", "10"}},
			exp{},
			verifyRampingVUs(null.NewInt(10, true), buildStages(66, 5)),
		},
		{opts{cli: []string{"-u", "1", "-i", "6", "-d", "10s"}}, exp{}, func(t *testing.T, c Config) {
			verifySharedIters(I(1), I(6))(t, c)
			sharedIterConfig, ok := c.Scenarios[lib.DefaultScenarioName].(executor.SharedIterationsConfig)
			require.True(t, ok)
			assert.Equal(t, sharedIterConfig.MaxDuration.TimeDuration(), 10*time.Second)
		}},
		// This should get a validation error since VUs are more than the shared iterations
		{opts{cli: []string{"--vus", "10", "-i", "6"}}, exp{validationErrors: true}, verifySharedIters(I(10), I(6))},
		{opts{cli: []string{"-s", "10s:5", "-s", "10s:"}}, exp{validationErrors: true}, nil},
		{opts{fs: defaultConfig(`{"stages": [{"duration": "20s"}], "vus": 10}`)}, exp{validationErrors: true}, nil},
		// These should emit a derivation error
		{opts{cli: []string{"-u", "2", "-d", "10s", "-s", "10s:20"}}, exp{derivationError: true}, nil},
		{opts{cli: []string{"-u", "3", "-i", "5", "-s", "10s:20"}}, exp{derivationError: true}, nil},
		{opts{cli: []string{"-u", "3", "-d", "0"}}, exp{derivationError: true}, nil},
		{
			opts{runner: &lib.Options{
				VUs:      null.IntFrom(5),
				Duration: types.NullDurationFrom(44 * time.Second),
				Stages: []lib.Stage{
					{Duration: types.NullDurationFrom(3 * time.Second), Target: I(20)},
				},
			}}, exp{derivationError: true}, nil,
		},
		{opts{fs: defaultConfig(`{"scenarios": {}}`)}, exp{logWarning: true}, verifyOneIterPerOneVU},
		// Test if environment variable shortcuts are working as expected
		{opts{env: []string{"K6_VUS=5", "K6_ITERATIONS=15"}}, exp{}, verifySharedIters(I(5), I(15))},
		{opts{env: []string{"K6_VUS=10", "K6_DURATION=20s"}}, exp{}, verifyConstLoopingVUs(I(10), 20*time.Second)},
		{opts{env: []string{"K6_VUS=10", "K6_DURATION=10000"}}, exp{}, verifyConstLoopingVUs(I(10), 10*time.Second)},
		{
			opts{env: []string{"K6_STAGES=2m30s:11,1h1m:100"}},
			exp{},
			verifyRampingVUs(null.NewInt(1, false), buildStages(150, 11, 3660, 100)),
		},
		{
			opts{env: []string{"K6_STAGES=100s:100,0m30s:0", "K6_VUS=0"}},
			exp{},
			verifyRampingVUs(null.NewInt(0, true), buildStages(100, 100, 30, 0)),
		},
		{opts{env: []string{"K6_STAGES=1000:100"}}, exp{consolidationError: true}, nil}, // intentionally unsupported
		// Test if JSON configs work as expected
		{opts{fs: defaultConfig(`{"iterations": 77, "vus": 7}`)}, exp{}, verifySharedIters(I(7), I(77))},
		{opts{fs: defaultConfig(`wrong-json`)}, exp{consolidationError: true}, nil},
		{opts{fs: getFS(nil), cli: []string{"--config", "/my/config.file"}}, exp{consolidationError: true}, nil},

		// Test combinations between options and levels
		{opts{cli: []string{"--vus", "1"}}, exp{}, verifyOneIterPerOneVU},
		{opts{cli: []string{"--vus", "10"}}, exp{logWarning: true}, verifyOneIterPerOneVU},
		{
			opts{
				fs:  getFS([]file{{"/my/config.file", `{"vus": 8, "duration": "2m"}`}}),
				cli: []string{"--config", "/my/config.file"},
			}, exp{}, verifyConstLoopingVUs(I(8), 120*time.Second),
		},
		{
			opts{
				fs:  getFS([]file{{"/my/config.file", `{"duration": 20000}`}}),
				cli: []string{"--config", "/my/config.file"},
			}, exp{}, verifyConstLoopingVUs(null.NewInt(1, false), 20*time.Second),
		},
		{
			opts{
				fs:  defaultConfig(`{"stages": [{"duration": "20s", "target": 20}], "vus": 10}`),
				env: []string{"K6_DURATION=15s"},
				cli: []string{"--stage", ""},
			},
			exp{logWarning: true},
			verifyOneIterPerOneVU,
		},
		{
			opts{
				runner: &lib.Options{VUs: null.IntFrom(5), Duration: types.NullDurationFrom(50 * time.Second)},
				cli:    []string{"--stage", "5s:5"},
			},
			exp{},
			verifyRampingVUs(I(5), buildStages(5, 5)),
		},
		{
			opts{
				fs:     defaultConfig(`{"stages": [{"duration": "20s", "target": 10}]}`),
				runner: &lib.Options{VUs: null.IntFrom(5)},
			},
			exp{},
			verifyRampingVUs(I(5), buildStages(20, 10)),
		},
		{
			opts{
				fs:     defaultConfig(`{"stages": [{"duration": "20s", "target": 10}]}`),
				runner: &lib.Options{VUs: null.IntFrom(5)},
				env:    []string{"K6_VUS=15", "K6_ITERATIONS=17"},
			},
			exp{},
			verifySharedIters(I(15), I(17)),
		},
		{
			opts{
				fs:     defaultConfig(`{"stages": [{"duration": "11s", "target": 11}]}`),
				runner: &lib.Options{VUs: null.IntFrom(22)},
				env:    []string{"K6_VUS=33"},
				cli:    []string{"--stage", "44s:44", "-s", "55s:55"},
			},
			exp{},
			verifyRampingVUs(null.NewInt(33, true), buildStages(44, 44, 55, 55)),
		},

		// TODO: test the future full overwriting of the duration/iterations/stages/execution options
		{
			opts{
				fs: defaultConfig(`{
					"scenarios": { "someKey": {
						"executor": "constant-vus", "vus": 10, "duration": "60s", "gracefulStop": "10s",
						"startTime": "70s", "env": {"test": "mest"}, "exec": "someFunc"
					}}}`),
				env: []string{"K6_ITERATIONS=25"},
				cli: []string{"--vus", "12"},
			},
			exp{},
			verifySharedIters(I(12), I(25)),
		},
		{
			opts{
				fs: defaultConfig(`{"scenarios": { "foo": {
					"executor": "constant-vus", "vus": 2, "duration": "1d",
					"gracefulStop": "10000", "startTime": 1000.5
				}}}`),
			}, exp{}, func(t *testing.T, c Config) {
				exec := c.Scenarios["foo"]
				require.NotEmpty(t, exec)
				require.IsType(t, executor.ConstantVUsConfig{}, exec)
				clvc, ok := exec.(executor.ConstantVUsConfig)
				require.True(t, ok)
				assert.Equal(t, null.IntFrom(2), clvc.VUs)
				assert.Equal(t, types.NullDurationFrom(24*time.Hour), clvc.Duration)
				assert.Equal(t, types.NullDurationFrom(time.Second+500*time.Microsecond), clvc.StartTime)
				assert.Equal(t, types.NullDurationFrom(10*time.Second), clvc.GracefulStop)
			},
		},
		{
			opts{
				fs: defaultConfig(`{"scenarios": { "def": {
					"executor": "externally-controlled", "vus": 15, "duration": "2h"
				}}}`),
			},
			exp{},
			verifyExternallyExecuted("def", I(15), 2*time.Hour),
		},
		// TODO: test execution-segment

		// Just in case, verify that no options will result in the same 1 vu 1 iter config
		{opts{}, exp{}, verifyOneIterPerOneVU},

		// Test system tags
		{opts{}, exp{}, func(t *testing.T, c Config) {
			assert.Equal(t, &metrics.DefaultSystemTagSet, c.Options.SystemTags)
		}},
		{opts{cli: []string{"--system-tags", `""`}}, exp{}, func(t *testing.T, c Config) {
			assert.Equal(t, metrics.SystemTagSet(0), *c.Options.SystemTags)
		}},
		{
			opts{
				runner: &lib.Options{
					SystemTags: metrics.NewSystemTagSet(metrics.TagSubproto, metrics.TagURL),
				},
			},
			exp{},
			func(t *testing.T, c Config) {
				assert.Equal(
					t,
					*metrics.NewSystemTagSet(metrics.TagSubproto, metrics.TagURL),
					*c.Options.SystemTags,
				)
			},
		},

		// Test-wide Tags
		{
			opts{
				fs:  defaultConfig(`{"tags": { "codeTagKey": "codeTagValue"}}`),
				cli: []string{"--tag", "clitagkey=clitagvalue"},
			},
			exp{},
			func(t *testing.T, c Config) {
				exp := map[string]string{"clitagkey": "clitagvalue"}
				assert.Equal(t, exp, c.RunTags)
			},
		},

		// Test summary trend stats
		{opts{}, exp{}, func(t *testing.T, c Config) {
			assert.Equal(t, lib.DefaultSummaryTrendStats, c.Options.SummaryTrendStats)
		}},
		{opts{cli: []string{"--summary-trend-stats", ""}}, exp{}, func(t *testing.T, c Config) {
			assert.Equal(t, []string{}, c.Options.SummaryTrendStats)
		}},
		{opts{cli: []string{"--summary-trend-stats", "coun"}}, exp{consolidationError: true}, nil},
		{opts{cli: []string{"--summary-trend-stats", "med,avg,p("}}, exp{consolidationError: true}, nil},
		{opts{cli: []string{"--summary-trend-stats", "med,avg,p(-1)"}}, exp{consolidationError: true}, nil},
		{opts{cli: []string{"--summary-trend-stats", "med,avg,p(101)"}}, exp{consolidationError: true}, nil},
		{opts{cli: []string{"--summary-trend-stats", "med,avg,p(99.999)"}}, exp{}, func(t *testing.T, c Config) {
			assert.Equal(t, []string{"med", "avg", "p(99.999)"}, c.Options.SummaryTrendStats)
		}},
		{
			opts{runner: &lib.Options{SummaryTrendStats: []string{"avg", "p(90)", "count"}}},
			exp{},
			func(t *testing.T, c Config) {
				assert.Equal(t, []string{"avg", "p(90)", "count"}, c.Options.SummaryTrendStats)
			},
		},
		{opts{cli: []string{}}, exp{}, func(t *testing.T, c Config) {
			assert.Equal(t, types.DNSConfig{
				TTL:    null.NewString("5m", false),
				Select: types.NullDNSSelect{DNSSelect: types.DNSrandom, Valid: false},
				Policy: types.NullDNSPolicy{DNSPolicy: types.DNSpreferIPv4, Valid: false},
			}, c.Options.DNS)
		}},
		{opts{env: []string{"K6_DNS=ttl=5,select=roundRobin"}}, exp{}, func(t *testing.T, c Config) {
			assert.Equal(t, types.DNSConfig{
				TTL:    null.StringFrom("5"),
				Select: types.NullDNSSelect{DNSSelect: types.DNSroundRobin, Valid: true},
				Policy: types.NullDNSPolicy{DNSPolicy: types.DNSpreferIPv4, Valid: false},
			}, c.Options.DNS)
		}},
		{opts{env: []string{"K6_DNS=ttl=inf,select=random,policy=preferIPv6"}}, exp{}, func(t *testing.T, c Config) {
			assert.Equal(t, types.DNSConfig{
				TTL:    null.StringFrom("inf"),
				Select: types.NullDNSSelect{DNSSelect: types.DNSrandom, Valid: true},
				Policy: types.NullDNSPolicy{DNSPolicy: types.DNSpreferIPv6, Valid: true},
			}, c.Options.DNS)
		}},
		// This is functionally invalid, but will error out in validation done in js.parseTTL().
		{opts{cli: []string{"--dns", "ttl=-1"}}, exp{}, func(t *testing.T, c Config) {
			assert.Equal(t, types.DNSConfig{
				TTL:    null.StringFrom("-1"),
				Select: types.NullDNSSelect{DNSSelect: types.DNSrandom, Valid: false},
				Policy: types.NullDNSPolicy{DNSPolicy: types.DNSpreferIPv4, Valid: false},
			}, c.Options.DNS)
		}},
		{opts{cli: []string{"--dns", "ttl=0,blah=nope"}}, exp{cliReadError: true}, nil},
		{opts{cli: []string{"--dns", "ttl=0"}}, exp{}, func(t *testing.T, c Config) {
			assert.Equal(t, types.DNSConfig{
				TTL:    null.StringFrom("0"),
				Select: types.NullDNSSelect{DNSSelect: types.DNSrandom, Valid: false},
				Policy: types.NullDNSPolicy{DNSPolicy: types.DNSpreferIPv4, Valid: false},
			}, c.Options.DNS)
		}},
		{opts{cli: []string{"--dns", "ttl=5s,select="}}, exp{cliReadError: true}, nil},
		{
			opts{fs: defaultConfig(`{"dns": {"ttl": "0", "select": "roundRobin", "policy": "onlyIPv4"}}`)},
			exp{},
			func(t *testing.T, c Config) {
				assert.Equal(t, types.DNSConfig{
					TTL:    null.StringFrom("0"),
					Select: types.NullDNSSelect{DNSSelect: types.DNSroundRobin, Valid: true},
					Policy: types.NullDNSPolicy{DNSPolicy: types.DNSonlyIPv4, Valid: true},
				}, c.Options.DNS)
			},
		},
		{
			opts{
				fs:  defaultConfig(`{"dns": {"ttl": "0"}}`),
				env: []string{"K6_DNS=ttl=30,policy=any"},
			},
			exp{},
			func(t *testing.T, c Config) {
				assert.Equal(t, types.DNSConfig{
					TTL:    null.StringFrom("30"),
					Select: types.NullDNSSelect{DNSSelect: types.DNSrandom, Valid: false},
					Policy: types.NullDNSPolicy{DNSPolicy: types.DNSany, Valid: true},
				}, c.Options.DNS)
			},
		},
		{
			// CLI overrides all, falling back to env
			opts{
				fs:  defaultConfig(`{"dns": {"ttl": "60", "select": "first"}}`),
				env: []string{"K6_DNS=ttl=30,select=random,policy=any"},
				cli: []string{"--dns", "ttl=5"},
			},
			exp{},
			func(t *testing.T, c Config) {
				assert.Equal(t, types.DNSConfig{
					TTL:    null.StringFrom("5"),
					Select: types.NullDNSSelect{DNSSelect: types.DNSrandom, Valid: true},
					Policy: types.NullDNSPolicy{DNSPolicy: types.DNSany, Valid: true},
				}, c.Options.DNS)
			},
		},
		{
			opts{env: []string{"K6_NO_SETUP=true", "K6_NO_TEARDOWN=false"}},
			exp{},
			func(t *testing.T, c Config) {
				assert.Equal(t, null.BoolFrom(true), c.Options.NoSetup)
				assert.Equal(t, null.BoolFrom(false), c.Options.NoTeardown)
			},
		},
		{
			opts{env: []string{"K6_NO_SETUP=false", "K6_NO_TEARDOWN=bool"}},
			exp{
				consolidationError: true,
			},
			nil,
		},
		// TODO: test for differences between flagsets
		// TODO: more tests in general, especially ones not related to execution parameters...
	}
}

func runTestCase(t *testing.T, testCase configConsolidationTestCase, subCmd string) {
	t.Logf("Test for `k6 %s` with opts=%#v and exp=%#v\n", subCmd, testCase.options, testCase.expected)

	ts := newGlobalTestState(t)
	ts.args = append([]string{"k6", subCmd}, testCase.options.cli...)
	ts.envVars = buildEnvMap(testCase.options.env)
	if testCase.options.fs != nil {
		ts.globalState.fs = testCase.options.fs
	}

	rootCmd := newRootCommand(ts.globalState)
	cmd, args, err := rootCmd.cmd.Find(ts.args[1:])
	require.NoError(t, err)

	err = cmd.ParseFlags(args)
	if testCase.expected.cliParseError {
		require.Error(t, err)
		return
	}
	require.NoError(t, err)

	flagSet := cmd.Flags()

	// TODO: remove these hacks when we improve the configuration...
	var cliConf Config
	if flagSet.Lookup("out") != nil {
		cliConf, err = getConfig(flagSet)
	} else {
		opts, errOpts := getOptions(flagSet)
		cliConf, err = Config{Options: opts}, errOpts
	}
	if testCase.expected.cliReadError {
		require.Error(t, err)
		return
	}
	require.NoError(t, err)

	var opts lib.Options
	if testCase.options.runner != nil {
		opts = *testCase.options.runner
	}
	consolidatedConfig, err := getConsolidatedConfig(ts.globalState, cliConf, opts)
	if testCase.expected.consolidationError {
		require.Error(t, err)
		return
	}
	require.NoError(t, err)

	derivedConfig := consolidatedConfig
	derivedConfig.Options, err = executor.DeriveScenariosFromShortcuts(consolidatedConfig.Options, ts.logger)
	if testCase.expected.derivationError {
		require.Error(t, err)
		return
	}
	require.NoError(t, err)

	if warnings := ts.loggerHook.Drain(); testCase.expected.logWarning {
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
	t.Parallel()

	for tcNum, testCase := range getConfigConsolidationTestCases() {
		tcNum, testCase := tcNum, testCase
		subCommands := testCase.options.cmds
		if subCommands == nil { // handle the most common case
			subCommands = []string{"run", "archive", "cloud"}
		}
		for fsNum, subCmd := range subCommands {
			fsNum, subCmd := fsNum, subCmd
			t.Run(
				fmt.Sprintf("TestCase#%d_FlagSet#%d", tcNum, fsNum),
				func(t *testing.T) {
					t.Parallel()
					runTestCase(t, testCase, subCmd)
				},
			)
		}
	}
}
