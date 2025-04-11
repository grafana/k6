package csv

import (
	"testing"
	"time"

	"gopkg.in/guregu/null.v3"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/lib/types"
)

func TestNewConfig(t *testing.T) {
	t.Parallel()

	config := NewConfig()
	assert.Equal(t, "file.csv", config.FileName.String)
	assert.Equal(t, "1s", config.SaveInterval.String())
	assert.Equal(t, "unix", config.TimeFormat.String)
}

func TestApply(t *testing.T) {
	t.Parallel()

	configs := []Config{
		{
			FileName:     null.StringFrom(""),
			SaveInterval: types.NullDurationFrom(2 * time.Second),
			TimeFormat:   null.StringFrom("unix"),
		},
		{
			FileName:     null.StringFrom("newPath"),
			SaveInterval: types.NewNullDuration(time.Duration(1), false),
			TimeFormat:   null.StringFrom("rfc3339"),
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
			TimeFormat:   "rfc3339",
		},
	}

	for i := range configs {
		config := configs[i]
		expected := expected[i]

		t.Run(expected.FileName+"_"+expected.SaveInterval, func(t *testing.T) {
			t.Parallel()

			baseConfig := NewConfig()
			baseConfig = baseConfig.Apply(config)

			assert.Equal(t, expected.FileName, baseConfig.FileName.String)
			assert.Equal(t, expected.SaveInterval, baseConfig.SaveInterval.String())
			assert.Equal(t, expected.TimeFormat, baseConfig.TimeFormat.String)
		})
	}
}

func TestParseArg(t *testing.T) {
	t.Parallel()

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
		"saveInterval=5s": {
			config: Config{
				FileName:     null.NewString("file.csv", false),
				SaveInterval: types.NullDurationFrom(5 * time.Second),
				TimeFormat:   null.NewString("unix", false),
			},
		},
		"filename=test.csv,saveInterval=5s": {
			expectedErr: true,
		},
		"fileName=test.csv,timeFormat=rfc3339": {
			config: Config{
				FileName:     null.StringFrom("test.csv"),
				SaveInterval: types.NewNullDuration(1*time.Second, false),
				TimeFormat:   null.StringFrom("rfc3339"),
			},
		},
	}

	for arg, testCase := range cases {
		arg := arg
		testCase := testCase

		t.Run(arg, func(t *testing.T) {
			t.Parallel()

			config, err := ParseArg(arg)

			if testCase.expectedErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, testCase.config, config)
		})
	}
}
