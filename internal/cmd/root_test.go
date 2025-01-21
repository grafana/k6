package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/internal/cmd/tests"
)

func TestRootCommandHelpDisplayCommands(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name                  string
		extraArgs             []string
		wantExitCode          exitcodes.ExitCode
		wantStdoutContains    string
		wantStdoutNotContains string
	}{
		{
			name:               "should have archive command",
			wantStdoutContains: "  archive     Create an archive",
		},
		{
			name:               "should have cloud command",
			wantStdoutContains: "  cloud       Run a test on the cloud",
		},
		{
			name:               "should have completion command",
			wantStdoutContains: "  completion  Generate the autocompletion script for the specified shell",
		},
		{
			name:               "should have help command",
			wantStdoutContains: "  help        Help about any command",
		},
		{
			name:               "should have inspect command",
			wantStdoutContains: "  inspect     Inspect a script or archive",
		},
		{
			name:               "should have new command",
			wantStdoutContains: "  new         Create and initialize a new k6 script",
		},
		{
			name:               "should have pause command",
			wantStdoutContains: "  pause       Pause a running test",
		},
		{
			name:               "should have resume command",
			wantStdoutContains: "  resume      Resume a paused test",
		},
		{
			name:               "should have run command",
			wantStdoutContains: "  run         Start a test",
		},
		{
			name:               "should have scale command",
			wantStdoutContains: "  scale       Scale a running test",
		},
		{
			name:               "should have stats command",
			wantStdoutContains: "  stats       Show test metrics",
		},
		{
			name:               "should have status command",
			wantStdoutContains: "  status      Show test status",
		},
		{
			name:               "should have version command",
			wantStdoutContains: "  version     Show application version",
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ts := tests.NewGlobalTestState(t)
			ts.CmdArgs = []string{"k6", "help"}
			newRootCommand(ts.GlobalState).execute()

			if tc.wantStdoutContains != "" {
				assert.Contains(t, ts.Stdout.String(), tc.wantStdoutContains)
			}
		})
	}
}
