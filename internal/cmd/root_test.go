package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/cmd/state"
	"go.k6.io/k6/v2/errext/exitcodes"
	"go.k6.io/k6/v2/internal/cmd/tests"
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

func TestAddressGlobalOption(t *testing.T) {
	t.Parallel()

	t.Run("default value", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "", state.GetDefaultFlags(".config", ".cache").Address)
	})

	t.Run("set via --address flag", func(t *testing.T) {
		t.Parallel()
		ts := tests.NewGlobalTestState(t)
		flagSet := rootCmdPersistentFlagSet(ts.GlobalState)
		require.NoError(t, flagSet.Parse([]string{"--address", "localhost:9090"}))
		assert.Equal(t, "localhost:9090", ts.Flags.Address)
	})

	t.Run("set via -a shorthand flag", func(t *testing.T) {
		t.Parallel()
		ts := tests.NewGlobalTestState(t)
		flagSet := rootCmdPersistentFlagSet(ts.GlobalState)
		require.NoError(t, flagSet.Parse([]string{"-a", "localhost:9090"}))
		assert.Equal(t, "localhost:9090", ts.Flags.Address)
	})

	t.Run("set via K6_ADDRESS env var", func(t *testing.T) {
		t.Parallel()
		ts := tests.NewGlobalTestState(t)
		ts.Env["K6_ADDRESS"] = "localhost:9090"
		ts.ReparseFlags()
		assert.Equal(t, "localhost:9090", ts.Flags.Address)
	})
}

// TestAddressOptionPrecedence verifies that the CLI flag takes precedence over the K6_ADDRESS
// env var by going through the real cobra command path, reflecting the actual implementation.
func TestAddressGlobalOptionPrecedence(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	ts.Env["K6_ADDRESS"] = "localhost:9090"
	ts.CmdArgs = []string{"k6", "run", "--address", "localhost:9091", "script.js"}

	rootCmd := newRootCommand(ts.GlobalState)
	require.NoError(t, rootCmd.cmd.ParseFlags(ts.CmdArgs[1:]))

	assert.Equal(t, "localhost:9091", ts.Flags.Address)
}
