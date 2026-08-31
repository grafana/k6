package cmd

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/v2/errext"
	"go.k6.io/k6/v2/errext/exitcodes"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/lib/executor"
	"go.k6.io/k6/v2/lib/types"
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
}
