package cmd

import (
	"sync"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/internal/cmd/tests"
	"go.k6.io/k6/subcommand"
)

func TestRootCommandHelpDisplayCommands(t *testing.T) {
	t.Parallel()

	registerTestSubcommandExtensionsOnce.Do(func() {
		registerTestSubcommandExtensions(t)
	})

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
		{
			name:               "should have test-cmd-1 command",
			wantStdoutContains: "  test-cmd-1  Test command 1",
		},
		{
			name:               "should have test-cmd-2 command",
			wantStdoutContains: "  test-cmd-2  Test command 2",
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

var registerTestSubcommandExtensionsOnce sync.Once //nolint:gochecknoglobals

func registerTestSubcommandExtensions(t *testing.T) {
	t.Helper()

	subcommand.RegisterExtension("test-cmd-1", func(_ *state.GlobalState) *cobra.Command {
		return &cobra.Command{
			Use:   "test-cmd-1",
			Short: "Test command 1",
			Run:   func(_ *cobra.Command, _ []string) {},
		}
	})

	subcommand.RegisterExtension("test-cmd-2", func(_ *state.GlobalState) *cobra.Command {
		return &cobra.Command{
			Use:   "test-cmd-2",
			Short: "Test command 2",
			Run:   func(_ *cobra.Command, _ []string) {},
		}
	})
}
