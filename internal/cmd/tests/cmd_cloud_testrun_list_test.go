package tests

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v6cloudapi "go.k6.io/k6/v2/internal/cloudapi/v6"
	"go.k6.io/k6/v2/internal/cloudapi/v6/v6test"
	"go.k6.io/k6/v2/internal/cmd"
)

const loadTestIDForTests = 8410

func TestCloudTestRunList(t *testing.T) {
	t.Parallel()

	t.Run("lists runs successfully", func(t *testing.T) {
		t.Parallel()

		ts := newCloudTestRunListTestState(t, testTestRuns(),
			"--test-id", fmt.Sprintf("%d", loadTestIDForTests))

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		assert.Contains(t, stdout, fmt.Sprintf("Runs of test %d:", loadTestIDForTests))
		assert.Contains(t, stdout, "ID")
		assert.Contains(t, stdout, "STATUS")
		assert.Contains(t, stdout, "STARTED")
		assert.Contains(t, stdout, "DURATION")
		assert.Contains(t, stdout, "VUS")
		assert.Contains(t, stdout, "RESULT")
		assert.Contains(t, stdout, "1009852")
		assert.Contains(t, stdout, "finished")
		assert.Contains(t, stdout, "passed")
	})

	t.Run("fails without --test-id", func(t *testing.T) {
		t.Parallel()

		ts := NewGlobalTestState(t)
		ts.CmdArgs = []string{"k6", "cloud", "test-run", "list"}
		ts.Env["K6_CLOUD_TOKEN"] = validToken
		ts.Env["K6_CLOUD_STACK_ID"] = fmt.Sprintf("%d", validStackID)
		ts.ExpectedExitCode = -1

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stderr := ts.Stderr.String()
		assert.Contains(t, stderr, "test ID not specified")
	})

	t.Run("missing auth uses list-specific wording", func(t *testing.T) {
		t.Parallel()

		ts := NewGlobalTestState(t)
		ts.CmdArgs = []string{
			"k6", "cloud", "test-run", "list",
			"--test-id", fmt.Sprintf("%d", loadTestIDForTests),
		}
		ts.ExpectedExitCode = -1

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stderr := ts.Stderr.String()
		assert.Contains(t, stderr, "Listing cloud test runs requires auth settings")
		assert.NotContains(t, stderr, "Running cloud tests requires auth settings")
	})

	t.Run("empty run list", func(t *testing.T) {
		t.Parallel()

		ts := newCloudTestRunListTestState(t, nil,
			"--test-id", fmt.Sprintf("%d", loadTestIDForTests))

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		assert.Contains(t, stdout, fmt.Sprintf("Runs of test %d:", loadTestIDForTests))
		assert.Contains(t, stdout, "No runs found.")
	})

	t.Run("--help includes usage", func(t *testing.T) {
		t.Parallel()

		ts := NewGlobalTestState(t)
		ts.CmdArgs = []string{"k6", "cloud", "test-run", "list", "--help"}

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		assert.Contains(t, stdout, "Usage:\n  k6 cloud test-run list [flags]")
		assert.Contains(t, stdout, "--test-id")
		assert.Contains(t, stdout, "--limit")
		assert.Contains(t, stdout, "--all")
		assert.Contains(t, stdout, "--since")
		assert.Contains(t, stdout, "--json")
		assert.NotContains(t, stdout, "Global Flags:")
		assert.NotContains(t, stdout, "--config")
	})

	t.Run("--json outputs JSON", func(t *testing.T) {
		t.Parallel()

		ts := newCloudTestRunListTestState(t, testTestRuns(),
			"--test-id", fmt.Sprintf("%d", loadTestIDForTests),
			"--json",
		)

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()

		var runs []v6cloudapi.TestRun
		require.NoError(t, json.Unmarshal([]byte(stdout), &runs))
		assert.Equal(t, testTestRuns(), runs)
	})

	t.Run("--json with empty list outputs empty array", func(t *testing.T) {
		t.Parallel()

		ts := newCloudTestRunListTestState(t, nil,
			"--test-id", fmt.Sprintf("%d", loadTestIDForTests),
			"--json",
		)

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()

		var runs []v6cloudapi.TestRun
		require.NoError(t, json.Unmarshal([]byte(stdout), &runs))
		assert.JSONEq(t, `[]`, stdout)
		assert.Empty(t, runs)
	})
}

func testTestRuns() []v6cloudapi.TestRun {
	maxVUs := int32(50)
	return []v6cloudapi.TestRun{
		{
			ID:                1009852,
			LoadTestID:        loadTestIDForTests,
			ProjectID:         7,
			Status:            "finished",
			Result:            "passed",
			Created:           time.Date(2026, 4, 28, 14, 3, 0, 0, time.UTC),
			MaxVUs:            &maxVUs,
			ExecutionDuration: 302,
		},
		{
			ID:                1009711,
			LoadTestID:        loadTestIDForTests,
			ProjectID:         7,
			Status:            "finished",
			Result:            "failed",
			Created:           time.Date(2026, 4, 27, 14, 3, 0, 0, time.UTC),
			MaxVUs:            &maxVUs,
			ExecutionDuration: 250,
		},
	}
}

func newCloudTestRunListTestState(
	t *testing.T, runs []v6cloudapi.TestRun, args ...string,
) *GlobalTestState {
	t.Helper()

	srv := v6test.NewServer(t, v6test.Config{TestRuns: runs})

	ts := NewGlobalTestState(t)
	ts.CmdArgs = append([]string{"k6", "cloud", "test-run", "list"}, args...)
	ts.Env["K6_CLOUD_TOKEN"] = validToken
	ts.Env["K6_CLOUD_STACK_ID"] = fmt.Sprintf("%d", validStackID)
	ts.Env["K6_CLOUD_HOST_V6"] = srv.URL

	return ts
}
