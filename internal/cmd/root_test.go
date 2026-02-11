package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/internal/cmd/tests"
)

func TestRootCommandHelpDisplayCommands(t *testing.T) {
	t.Parallel()

	registerTestSubcommandExtensions(t)

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
			wantStdoutContains: "  cloud       Run and manage Grafana Cloud tests",
		},
		{
			name:               "should have completion command",
			wantStdoutContains: "  completion  Generate the autocompletion script for the specified shell",
		},
		{
			name:               "should have inspect command",
			wantStdoutContains: "  inspect     Inspect a script or archive",
		},
		{
			name:               "should have new command",
			wantStdoutContains: "  new         Create a test",
		},
		{
			name:               "should have run command",
			wantStdoutContains: "  run         Run a test",
		},
		{
			name:               "should have x command",
			wantStdoutContains: "  x           Extension subcommands",
		},
	}

	for _, tc := range testCases {
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
