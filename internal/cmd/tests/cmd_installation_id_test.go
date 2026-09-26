package tests

import (
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/cmd/state"
	"go.k6.io/k6/v2/internal/cmd"
	"go.k6.io/k6/v2/internal/lib/testutils"
	"go.k6.io/k6/v2/lib/fsext"
)

func TestUsageReportInstallationID(t *testing.T) {
	t.Parallel()

	registerReportTestSubcommand(t)

	tests := []struct {
		name       string
		args       []string
		script     string
		configPath string
		optOut     bool
		failedFS   bool
	}{
		{
			name:   "run reports the installation ID",
			args:   []string{"run", "-"},
			script: `export default function() {};`,
		},
		{
			name: "subcommand reports the installation ID",
			args: []string{"x", "testsub"},
		},
		{
			name:       "a custom config path does not move the installation ID",
			args:       []string{"run", "-"},
			script:     `export default function() {};`,
			configPath: "elsewhere/k6.json",
		},
		{
			name:   "run opt-out reports nothing and stores nothing",
			args:   []string{"run", "-"},
			script: `export default function() {};`,
			optOut: true,
		},
		{
			name:   "subcommand opt-out reports nothing and stores nothing",
			args:   []string{"x", "testsub"},
			optOut: true,
		},
		{
			name:     "the installation ID it cannot store stays out of the report and the output",
			args:     []string{"x", "testsub"},
			failedFS: true,
		},
		{
			name:     "the storage failure surfaces at debug level",
			args:     []string{"run", "-v", "-"},
			script:   `export default function() {};`,
			failedFS: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var reportCount atomic.Int32
			var gotBody atomic.Value
			reportServer := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				assert.NoError(t, err)
				gotBody.Store(body)
				reportCount.Add(1)
			}))
			t.Cleanup(reportServer.Close)

			ts := NewGlobalTestState(t)
			ts.Env["K6_NO_USAGE_REPORT"] = strconv.FormatBool(tt.optOut)
			ts.Env[state.UsageReportURL] = reportServer.URL
			if tt.configPath != "" {
				require.NoError(t, fsext.WriteFile(ts.FS, tt.configPath, []byte("{}"), 0o600))
				ts.Env["K6_CONFIG"] = tt.configPath
			}
			var idAccesses atomic.Int32
			ts.FS = fsext.NewChangePathFs(ts.FS, func(name string) (string, error) {
				if !strings.HasSuffix(name, "installation-id") {
					return name, nil
				}
				idAccesses.Add(1)
				if tt.failedFS {
					return "", fs.ErrPermission
				}
				return name, nil
			})
			ts.CmdArgs = append([]string{"k6"}, tt.args...)
			ts.Stdin = bytes.NewBufferString(tt.script)
			ts.ReparseFlags()

			cmd.ExecuteWithGlobalState(ts.GlobalState)

			if tt.optOut {
				require.Zero(t, reportCount.Load())
				require.Zero(t, idAccesses.Load())
				return
			}

			require.Equal(t, int32(1), reportCount.Load())
			raw, ok := gotBody.Load().([]byte)
			require.True(t, ok)
			var report map[string]any
			require.NoError(t, json.Unmarshal(raw, &report))
			require.NotEmpty(t, report["k6_version"])

			if tt.failedFS {
				require.NotContains(t, report, "installation_id")

				entries := ts.LoggerHook.Drain()
				if slices.Contains(tt.args, "-v") {
					require.True(t, testutils.LogContains(entries, logrus.DebugLevel, "Omitting the installation ID"))
					return
				}
				require.Empty(t, entries)
				require.Empty(t, ts.Stdout.String())
				return
			}

			id, ok := report["installation_id"].(string)
			require.True(t, ok)
			require.NoError(t, uuid.Validate(id))
			saved, err := fsext.ReadFile(ts.FS, filepath.Join(".config", "k6", "installation-id"))
			require.NoError(t, err)
			require.Equal(t, id, string(saved))
		})
	}
}
