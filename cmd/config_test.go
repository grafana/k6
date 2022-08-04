package cmd

import (
	"testing"
	"time"

	"github.com/mstoykov/envconfig"
	"github.com/stretchr/testify/assert"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/errext"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/executor"
	"go.k6.io/k6/lib/types"
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
	t.Parallel()
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
			t.Parallel()
			for _, test := range data.Tests {
				t.Run(`"`+test.Name+`"`, func(t *testing.T) {
					t.Parallel()
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
	t.Parallel()
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
			t.Parallel()
			for value, fn := range data {
				value, fn := value, fn
				t.Run(`"`+value+`"`, func(t *testing.T) {
					t.Parallel()
					var config Config
					assert.NoError(t, envconfig.Process("", &config, func(key string) (string, bool) {
						if key == field.Key {
							return value, true
						}
						return "", false
					}))
					fn(config)
				})
			}
		})
	}
}

func TestConfigApply(t *testing.T) {
	t.Parallel()
	t.Run("Linger", func(t *testing.T) {
		t.Parallel()
		conf := Config{}.Apply(Config{Linger: null.BoolFrom(true)})
		assert.Equal(t, null.BoolFrom(true), conf.Linger)
	})
	t.Run("NoUsageReport", func(t *testing.T) {
		t.Parallel()
		conf := Config{}.Apply(Config{NoUsageReport: null.BoolFrom(true)})
		assert.Equal(t, null.BoolFrom(true), conf.NoUsageReport)
	})
	t.Run("Out", func(t *testing.T) {
		t.Parallel()
		conf := Config{}.Apply(Config{Out: []string{"influxdb"}})
		assert.Equal(t, []string{"influxdb"}, conf.Out)

		conf = Config{}.Apply(Config{Out: []string{"influxdb", "json"}})
		assert.Equal(t, []string{"influxdb", "json"}, conf.Out)
	})
}

func TestDeriveAndValidateConfig(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		conf   Config
		isExec bool
		err    string
	}{
		{"defaultOK", Config{}, true, ""},
		{
			"defaultErr",
			Config{},
			false,
			"executor default: function 'default' not found in exports",
		},
		{
			"nonDefaultOK", Config{Options: lib.Options{Scenarios: lib.ScenarioConfigs{
				"per_vu_iters": executor.PerVUIterationsConfig{
					BaseConfig: executor.BaseConfig{
						Name: "per_vu_iters", Type: "per-vu-iterations", Exec: null.StringFrom("nonDefault"),
					},
					VUs:         null.IntFrom(1),
					Iterations:  null.IntFrom(1),
					MaxDuration: types.NullDurationFrom(time.Second),
				},
			}}}, true, "",
		},
		{
			"nonDefaultErr",
			Config{Options: lib.Options{Scenarios: lib.ScenarioConfigs{
				"per_vu_iters": executor.PerVUIterationsConfig{
					BaseConfig: executor.BaseConfig{
						Name: "per_vu_iters", Type: "per-vu-iterations", Exec: null.StringFrom("nonDefaultErr"),
					},
					VUs:         null.IntFrom(1),
					Iterations:  null.IntFrom(1),
					MaxDuration: types.NullDurationFrom(time.Second),
				},
			}}},
			false,
			"executor per_vu_iters: function 'nonDefaultErr' not found in exports",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := deriveAndValidateConfig(tc.conf,
				func(_ string) bool { return tc.isExec }, nil)
			if tc.err != "" {
				var ecerr errext.HasExitCode
				assert.ErrorAs(t, err, &ecerr)
				assert.Equal(t, exitcodes.InvalidConfig, ecerr.ExitCode())
				assert.Contains(t, err.Error(), tc.err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
