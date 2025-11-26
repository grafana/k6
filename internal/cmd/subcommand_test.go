package cmd

import (
	"sync"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/internal/cmd/tests"
	"go.k6.io/k6/subcommand"
)

func TestExtensionSubcommands(t *testing.T) {
	t.Parallel()

	registerTestSubcommandExtensions(t)

	t.Run("returns all extension subcommands", func(t *testing.T) {
		t.Parallel()

		ts := tests.NewGlobalTestState(t)
		commands := extensionSubcommands(ts.GlobalState)

		// Should have at least the 3 test extensions we registered
		require.GreaterOrEqual(t, len(commands), 2)

		// Check that our test commands are present
		commandNames := make(map[string]bool)
		for _, cmd := range commands {
			commandNames[cmd.Name()] = true
		}

		require.True(t, commandNames["test-cmd-1"], "test-cmd-1 should be present")
		require.True(t, commandNames["test-cmd-2"], "test-cmd-2 should be present")
		require.True(t, commandNames["test-cmd-3"], "test-cmd-3 should be present")
	})

	t.Run("returns commands with correct properties", func(t *testing.T) {
		t.Parallel()

		ts := tests.NewGlobalTestState(t)
		commands := extensionSubcommands(ts.GlobalState)

		for _, cmd := range commands {
			require.NotEmpty(t, cmd.Use, "command should have a Use field")

			switch cmd.Use {
			case "test-cmd-1":
				require.Equal(t, "Test command 1", cmd.Short)
			case "test-cmd-2":
				require.Equal(t, "Test command 2", cmd.Short)
			case "test-cmd-3":
				require.Equal(t, "Test command 3", cmd.Short)
			}
		}
	})
}

func TestXCommandHelpDisplayCommands(t *testing.T) {
	t.Parallel()

	registerTestSubcommandExtensions(t)

	testCases := []struct {
		name               string
		wantStdoutContains string
	}{
		{
			name:               "should have test-cmd-1 command",
			wantStdoutContains: "  test-cmd-1  Test command 1",
		},
		{
			name:               "should have test-cmd-2 command",
			wantStdoutContains: "  test-cmd-2  Test command 2",
		},
		{
			name:               "should have test-cmd-3 command",
			wantStdoutContains: "  test-cmd-3  Test command 3",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ts := tests.NewGlobalTestState(t)
			ts.CmdArgs = []string{"k6", "x", "help"}
			newRootCommand(ts.GlobalState).execute()

			require.Contains(t, ts.Stdout.String(), tc.wantStdoutContains)
		})
	}
}

var registerTestSubcommandExtensionsOnce sync.Once //nolint:gochecknoglobals

func registerTestSubcommandExtensions(t *testing.T) {
	t.Helper()

	registerTestSubcommandExtensionsOnce.Do(func() {
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

		subcommand.RegisterExtension("test-cmd-3", func(_ *state.GlobalState) *cobra.Command {
			return &cobra.Command{
				Use:   "test-cmd-3",
				Short: "Test command 3",
				Run:   func(_ *cobra.Command, _ []string) {},
			}
		})
	})
}
