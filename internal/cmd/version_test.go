package cmd

import (
	"encoding/json"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.k6.io/k6/internal/build"
	"go.k6.io/k6/internal/cmd/tests"
)

func TestVersionFlag(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	ts.ExpectedExitCode = 0
	ts.CmdArgs = []string{"k6", "--version"}

	ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	t.Log(stdout)
	assert.NotEmpty(t, stdout)

	// Check that the version/format string is correct
	assert.Contains(t, stdout, "k6 v")
	assert.Contains(t, stdout, build.Version)
	assert.Contains(t, stdout, runtime.Version())
	assert.Contains(t, stdout, runtime.GOOS)
	assert.Contains(t, stdout, runtime.GOARCH)
}

func TestVersionSubCommand(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	ts.ExpectedExitCode = 0
	ts.CmdArgs = []string{"k6", "version"}

	ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	t.Log(stdout)
	assert.NotEmpty(t, stdout)

	// Check that the version/format string is correct
	assert.Contains(t, stdout, "k6 v")
	assert.Contains(t, stdout, build.Version)
	assert.Contains(t, stdout, runtime.Version())
	assert.Contains(t, stdout, runtime.GOOS)
	assert.Contains(t, stdout, runtime.GOARCH)
}

func TestVersionJSONSubCommand(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	ts.ExpectedExitCode = 0
	ts.CmdArgs = []string{"k6", "version", "--json"}

	ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	t.Log(stdout)
	assert.NotEmpty(t, stdout)

	// try to unmarshal the JSON output
	var details map[string]interface{}
	err := json.Unmarshal([]byte(stdout), &details)
	assert.NoError(t, err)

	// Check that details are correct
	assert.Contains(t, details, "version")
	assert.Contains(t, details, "go_version")
	assert.Contains(t, details, "go_os")
	assert.Contains(t, details, "go_arch")
	assert.Equal(t, "v"+build.Version, details["version"])
	assert.Equal(t, runtime.Version(), details["go_version"])
	assert.Equal(t, runtime.GOOS, details["go_os"])
	assert.Equal(t, runtime.GOARCH, details["go_arch"])
}
