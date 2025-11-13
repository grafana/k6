package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/internal/cmd/tests"
)

func TestExtensionSubcommands(t *testing.T) {
	t.Parallel()

	registerTestSubcommandExtensions(t)

	t.Run("returns all extension subcommands", func(t *testing.T) {
		t.Parallel()

		ts := tests.NewGlobalTestState(t)
		defined := []*cobra.Command{}

		var commands []*cobra.Command
		for cmd := range extensionSubcommands(ts.GlobalState, defined) {
			commands = append(commands, cmd)
		}

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

	t.Run("filters out already defined commands", func(t *testing.T) {
		t.Parallel()

		ts := tests.NewGlobalTestState(t)

		// Create a command with the same name as one of our extensions
		defined := []*cobra.Command{
			{
				Use:   "test-cmd-1",
				Short: "Already defined command",
				Run:   func(_ *cobra.Command, _ []string) {},
			},
		}

		var commands []*cobra.Command
		for cmd := range extensionSubcommands(ts.GlobalState, defined) {
			commands = append(commands, cmd)
		}

		// Check that test-cmd-1 is NOT in the results
		for _, cmd := range commands {
			require.NotEqual(t, "test-cmd-1", cmd.Name(), "test-cmd-1 should be filtered out")
		}

		// But test-cmd-2 and test-cmd-3 should still be present
		commandNames := make(map[string]bool)
		for _, cmd := range commands {
			commandNames[cmd.Name()] = true
		}

		require.True(t, commandNames["test-cmd-2"], "test-cmd-2 should be present")
		require.True(t, commandNames["test-cmd-3"], "test-cmd-3 should be present")
	})

	t.Run("prevents duplicate extensions", func(t *testing.T) {
		t.Parallel()

		ts := tests.NewGlobalTestState(t)
		defined := []*cobra.Command{}

		// Collect all commands
		var commands []*cobra.Command
		for cmd := range extensionSubcommands(ts.GlobalState, defined) {
			commands = append(commands, cmd)
		}

		// Check for duplicates
		seen := make(map[string]bool)
		for _, cmd := range commands {
			require.False(t, seen[cmd.Name()], "command %s should not appear twice", cmd.Name())
			seen[cmd.Name()] = true
		}
	})

	t.Run("returns commands with correct properties", func(t *testing.T) {
		t.Parallel()

		ts := tests.NewGlobalTestState(t)
		defined := []*cobra.Command{}

		for cmd := range extensionSubcommands(ts.GlobalState, defined) {
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
