package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/cmd/tests"
)

func TestFeaturesSubCommand(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	ts.ExpectedExitCode = 0
	ts.CmdArgs = []string{"k6", "features"}

	ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	require.NotEmpty(t, stdout)

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	require.GreaterOrEqual(t, len(lines), 2, "header plus at least one flag row")
	assert.Equal(t, []string{"FEATURE", "LIFECYCLE", "DESCRIPTION"}, strings.Fields(lines[0]))
}

func TestFeaturesJSONSubCommand(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	ts.ExpectedExitCode = 0
	ts.CmdArgs = []string{"k6", "features", "--json"}

	ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	require.NotEmpty(t, stdout)

	var flags []struct {
		Feature     string `json:"feature"`
		Lifecycle   string `json:"lifecycle"`
		Description string `json:"description"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &flags))
	require.NotEmpty(t, flags)

	for _, flag := range flags {
		assert.NotEmpty(t, flag.Feature)
		assert.NotEmpty(t, flag.Lifecycle)
		assert.NotEmpty(t, flag.Description)
	}
}
