package cmd

import (
	"encoding/json"
	"io/fs"
	"testing"
	"time"

	"github.com/mstoykov/envconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/errext"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/executor"
	"go.k6.io/k6/lib/fsext"
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
		data := data
		t.Run(data.Name, func(t *testing.T) {
			t.Parallel()
			for _, test := range data.Tests {
				test := test
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
		{"WebDashboard", "K6_WEB_DASHBOARD"}: {
			"":      func(c Config) { assert.Equal(t, null.Bool{}, c.WebDashboard) },
			"true":  func(c Config) { assert.Equal(t, null.BoolFrom(true), c.WebDashboard) },
			"false": func(c Config) { assert.Equal(t, null.BoolFrom(false), c.WebDashboard) },
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

func TestReadDiskConfigWithDefaultFlags(t *testing.T) {
	t.Parallel()
	memfs := fsext.NewMemMapFs()

	conf := []byte(`{"iterations":1028,"cloud":{"field1":"testvalue"}}`)
	defaultConfigPath := ".config/loadimpact/k6/config.json"
	require.NoError(t, fsext.WriteFile(memfs, defaultConfigPath, conf, 0o644))

	defaultFlags := state.GetDefaultFlags(".config")
	gs := &state.GlobalState{
		FS:           memfs,
		Flags:        defaultFlags,
		DefaultFlags: defaultFlags,
	}
	c, err := readDiskConfig(gs)
	require.NoError(t, err)

	assert.Equal(t, c.Iterations.Int64, int64(1028))
	assert.JSONEq(t, `{"field1":"testvalue"}`, string(c.Cloud))
}

func TestReadDiskConfigCustomFilePath(t *testing.T) {
	t.Parallel()
	memfs := fsext.NewMemMapFs()

	conf := []byte(`{"iterations":1028,"cloud":{"field1":"testvalue"}}`)
	require.NoError(t, fsext.WriteFile(memfs, "custom-path/config.json", conf, 0o644))

	defaultFlags := state.GetDefaultFlags(".config")
	gs := &state.GlobalState{
		FS:           memfs,
		Flags:        defaultFlags,
		DefaultFlags: defaultFlags,
	}
	gs.Flags.ConfigFilePath = "custom-path/config.json"

	c, err := readDiskConfig(gs)
	require.NoError(t, err)

	assert.Equal(t, c.Iterations.Int64, int64(1028))
	assert.JSONEq(t, `{"field1":"testvalue"}`, string(c.Cloud))
}

func TestReadDiskConfigNotFoundSilenced(t *testing.T) {
	t.Parallel()
	memfs := fsext.NewMemMapFs()

	// Put the file into a different and unexpected directory
	conf := []byte(`{"iterations":1028,"cloud":{"field1":"testvalue"}}`)
	defaultConfigPath := ".config/unknown-folder/k6/config.json"
	require.NoError(t, fsext.WriteFile(memfs, defaultConfigPath, conf, 0o644))

	defaultFlags := state.GetDefaultFlags(".config")
	gs := &state.GlobalState{
		FS:           memfs,
		Flags:        defaultFlags,
		DefaultFlags: defaultFlags,
	}
	c, err := readDiskConfig(gs)
	assert.NoError(t, err)
	assert.Empty(t, c)
}

func TestReadDiskConfigNotJSONExtension(t *testing.T) {
	t.Parallel()
	memfs := fsext.NewMemMapFs()

	conf := []byte(`{"iterations":1028,"cloud":{"field1":"testvalue"}}`)
	require.NoError(t, fsext.WriteFile(memfs, "custom-path/config.txt", conf, 0o644))

	defaultFlags := state.GetDefaultFlags(".config")
	gs := &state.GlobalState{
		FS:           memfs,
		DefaultFlags: defaultFlags,
		Flags:        defaultFlags,
	}
	gs.Flags.ConfigFilePath = "custom-path/config.txt"

	c, err := readDiskConfig(gs)
	require.NoError(t, err)

	assert.Equal(t, c.Iterations.Int64, int64(1028))
	assert.JSONEq(t, `{"field1":"testvalue"}`, string(c.Cloud))
}

func TestReadDiskConfigNotJSONContentError(t *testing.T) {
	t.Parallel()
	memfs := fsext.NewMemMapFs()

	conf := []byte(`bad json format`)
	defaultConfigPath := ".config/loadimpact/k6/config.json"
	require.NoError(t, fsext.WriteFile(memfs, defaultConfigPath, conf, 0o644))

	gs := &state.GlobalState{
		FS:    memfs,
		Flags: state.GetDefaultFlags(".config"),
	}
	_, err := readDiskConfig(gs)
	var serr *json.SyntaxError
	assert.ErrorAs(t, err, &serr)
}

func TestReadDiskConfigNotFoundErrorWithCustomPath(t *testing.T) {
	t.Parallel()
	memfs := fsext.NewMemMapFs()

	defaultFlags := state.GetDefaultFlags(".config")
	gs := &state.GlobalState{
		FS:           memfs,
		Flags:        defaultFlags,
		DefaultFlags: defaultFlags,
	}
	gs.Flags.ConfigFilePath = ".config/my-custom-path/k6/config.json"

	c, err := readDiskConfig(gs)
	assert.ErrorIs(t, err, fs.ErrNotExist)
	assert.Empty(t, c)
}

func TestWriteDiskConfigWithDefaultFlags(t *testing.T) {
	t.Parallel()
	memfs := fsext.NewMemMapFs()

	defaultFlags := state.GetDefaultFlags(".config")
	gs := &state.GlobalState{
		FS:           memfs,
		Flags:        defaultFlags,
		DefaultFlags: defaultFlags,
	}

	c := Config{WebDashboard: null.BoolFrom(true)}
	err := writeDiskConfig(gs, c)
	require.NoError(t, err)

	finfo, err := memfs.Stat(".config/loadimpact/k6/config.json")
	require.NoError(t, err)
	assert.NotEmpty(t, finfo.Size())
}

func TestWriteDiskConfigOverwrite(t *testing.T) {
	t.Parallel()
	memfs := fsext.NewMemMapFs()

	conf := []byte(`{"iterations":1028,"cloud":{"field1":"testvalue"}}`)
	defaultConfigPath := ".config/loadimpact/k6/config.json"
	require.NoError(t, fsext.WriteFile(memfs, defaultConfigPath, conf, 0o644))

	defaultFlags := state.GetDefaultFlags(".config")
	gs := &state.GlobalState{
		FS:           memfs,
		Flags:        defaultFlags,
		DefaultFlags: defaultFlags,
	}

	c := Config{WebDashboard: null.BoolFrom(true)}
	err := writeDiskConfig(gs, c)
	require.NoError(t, err)
}

func TestWriteDiskConfigCustomPath(t *testing.T) {
	t.Parallel()
	memfs := fsext.NewMemMapFs()

	defaultFlags := state.GetDefaultFlags(".config")
	gs := &state.GlobalState{
		FS:           memfs,
		Flags:        defaultFlags,
		DefaultFlags: defaultFlags,
	}
	gs.Flags.ConfigFilePath = "my-custom-path/config.json"

	c := Config{WebDashboard: null.BoolFrom(true)}
	err := writeDiskConfig(gs, c)
	require.NoError(t, err)
}

func TestWriteDiskConfigNoJSONContentError(t *testing.T) {
	t.Parallel()
	memfs := fsext.NewMemMapFs()

	defaultFlags := state.GetDefaultFlags(".config")
	gs := &state.GlobalState{
		FS:           memfs,
		Flags:        defaultFlags,
		DefaultFlags: defaultFlags,
	}

	c := Config{
		WebDashboard: null.BoolFrom(true),
		Options: lib.Options{
			Cloud: []byte(`invalid-json`),
		},
	}
	err := writeDiskConfig(gs, c)
	var serr *json.SyntaxError
	assert.ErrorAs(t, err, &serr)
}
