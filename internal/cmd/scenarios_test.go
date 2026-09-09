package cmd

import (
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/v2/errext"
	"go.k6.io/k6/v2/errext/exitcodes"
	"go.k6.io/k6/v2/internal/lib/testutils"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/lib/executor"
	"go.k6.io/k6/v2/lib/types"
	"go.k6.io/k6/v2/metrics"
)

func TestSelectScenarios(t *testing.T) {
	t.Parallel()

	ui := executor.NewConstantVUsConfig("ui")
	ui.VUs = null.IntFrom(7)
	ui.Duration = types.NullDurationFrom(30 * time.Second)
	ui.Exec = null.StringFrom("uiFn")
	ui.Env = map[string]string{"GREETING": "hi"}
	ui.Tags = map[string]string{"team": "core"}
	ui.Options = &lib.ScenarioOptions{Browser: map[string]any{"type": "chromium"}}

	api := executor.NewConstantVUsConfig("api")

	all := lib.ScenarioConfigs{"ui": ui, "api": api, "db": executor.NewConstantVUsConfig("db")}

	for _, tt := range []struct {
		name    string
		names   []string
		want    lib.ScenarioConfigs
		wantErr string
	}{
		{
			name:  "selects and preserves",
			names: []string{"ui", "api"},
			want:  lib.ScenarioConfigs{"ui": ui, "api": api},
		},
		{
			name:    "unknown name lists available",
			names:   []string{"ui", "gone"},
			wantErr: `scenario "gone" not found; available scenarios: api, db, ui`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			res, err := selectScenarios(lib.Options{Scenarios: all}, tt.names)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				var ec errext.HasExitCode
				require.ErrorAs(t, err, &ec)
				assert.Equal(t, exitcodes.InvalidConfig, ec.ExitCode())
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, res.Scenarios)
		})
	}

	_, err := selectScenarios(lib.Options{}, []string{"default"})
	require.EqualError(t, err, `scenario "default" not found; the script does not have any named scenario`)
}

func TestGetOptionsScenarioShortcuts(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		flag, value string
		allowed     bool
	}{
		{"vus", "2", false},
		{"duration", "1s", false},
		{"iterations", "2", false},
		{"stage", "1s:2", false},
		{"execution-segment", "0:1/2", true},
		{"execution-segment-sequence", "0,1/2,1", true},
	} {
		t.Run(tt.flag, func(t *testing.T) {
			t.Parallel()
			flags := optionFlagSet()
			require.NoError(t, flags.Parse([]string{"--scenario", "ui", "--" + tt.flag, tt.value}))
			_, err := getOptions(flags)
			if tt.allowed {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, "--scenario cannot be combined with --"+tt.flag)
			var ec errext.HasExitCode
			require.ErrorAs(t, err, &ec)
			assert.Equal(t, exitcodes.InvalidConfig, ec.ExitCode())
		})
	}
}

func TestDropScenarioThresholds(t *testing.T) {
	t.Parallel()

	kept := []string{
		"iterations", "iterations{name:scenario:api}", "iterations{scenario:ui}",
		"iterations{scenario:unknown}", "iterations{scenario:api,scenario:ui}",
		"iterations{scenario:api",
	}
	dropped := []string{
		"iterations{ 'scenario' : 'api', name:login}",
		"iterations{scenario:api}", "iterations{scenario:ui,scenario:api}",
	}
	thresholds := make(map[string]metrics.Thresholds)
	for _, name := range slices.Concat(kept, dropped) {
		thresholds[name] = metrics.NewThresholds([]string{"count>0"})
	}
	opts := lib.Options{Scenarios: lib.ScenarioConfigs{"ui": nil}, Thresholds: thresholds}
	logger, hook := testutils.NewLoggerWithHook(t, logrus.WarnLevel)
	dropScenarioThresholds(logger, &opts, lib.ScenarioConfigs{"ui": nil, "api": nil})

	assert.ElementsMatch(t, kept, slices.Collect(maps.Keys(opts.Thresholds)))
	assert.Len(t, thresholds, len(kept)+len(dropped))
	assert.Equal(t, []string{"--scenario skipped thresholds for excluded scenarios: " +
		strings.Join(dropped, "; ") + "; these thresholds remain skipped even if another scenario emits matching tags"}, hook.Lines())

	dropScenarioThresholds(logger, &opts, opts.Scenarios)
	assert.Empty(t, hook.Lines())
}
