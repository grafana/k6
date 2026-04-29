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

const projectIDForTests = 42

func TestCloudTestList(t *testing.T) {
	t.Parallel()

	t.Run("lists tests successfully using --project-id", func(t *testing.T) {
		t.Parallel()

		ts := newCloudTestListTestState(t, testLoadTests(),
			"--project-id", fmt.Sprintf("%d", projectIDForTests))

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		assert.Contains(t, stdout, fmt.Sprintf("Tests in project %d:", projectIDForTests))
		assert.Contains(t, stdout, "ID")
		assert.Contains(t, stdout, "NAME")
		assert.Contains(t, stdout, "CREATED")
		assert.Contains(t, stdout, "UPDATED")
		assert.Contains(t, stdout, "checkout-flow")
		assert.Contains(t, stdout, "api-smoke")
	})

	t.Run("uses K6_CLOUD_PROJECT_ID when --project-id is omitted", func(t *testing.T) {
		t.Parallel()

		ts := newCloudTestListTestState(t, testLoadTests())
		ts.Env["K6_CLOUD_PROJECT_ID"] = fmt.Sprintf("%d", projectIDForTests)

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		assert.Contains(t, stdout, fmt.Sprintf("Tests in project %d:", projectIDForTests))
		assert.Contains(t, stdout, "checkout-flow")
	})

	t.Run("fails without project", func(t *testing.T) {
		t.Parallel()

		ts := NewGlobalTestState(t)
		ts.CmdArgs = []string{"k6", "cloud", "test", "list"}
		ts.Env["K6_CLOUD_TOKEN"] = validToken
		ts.Env["K6_CLOUD_STACK_ID"] = fmt.Sprintf("%d", validStackID)
		ts.ExpectedExitCode = -1

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stderr := ts.Stderr.String()
		assert.Contains(t, stderr, "no project specified")
	})

	t.Run("missing auth uses list-specific wording", func(t *testing.T) {
		t.Parallel()

		ts := NewGlobalTestState(t)
		ts.CmdArgs = []string{"k6", "cloud", "test", "list", "--project-id", fmt.Sprintf("%d", projectIDForTests)}
		ts.ExpectedExitCode = -1

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stderr := ts.Stderr.String()
		assert.Contains(t, stderr, "Listing cloud tests requires auth settings")
		assert.NotContains(t, stderr, "Running cloud tests requires auth settings")
	})

	t.Run("empty test list", func(t *testing.T) {
		t.Parallel()

		ts := newCloudTestListTestState(t, nil,
			"--project-id", fmt.Sprintf("%d", projectIDForTests))

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		assert.Contains(t, stdout, fmt.Sprintf("Tests in project %d:", projectIDForTests))
		assert.Contains(t, stdout, "No tests found.")
	})

	t.Run("--help includes usage", func(t *testing.T) {
		t.Parallel()

		ts := NewGlobalTestState(t)
		ts.CmdArgs = []string{"k6", "cloud", "test", "list", "--help"}

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		assert.Contains(t, stdout, "Usage:\n  k6 cloud test list [flags]")
		assert.Contains(t, stdout, "--project-id")
		assert.Contains(t, stdout, "--json")
		assert.NotContains(t, stdout, "Global Flags:")
		assert.NotContains(t, stdout, "--config")
	})

	t.Run("--json outputs JSON", func(t *testing.T) {
		t.Parallel()

		ts := newCloudTestListTestState(t, testLoadTests(),
			"--project-id", fmt.Sprintf("%d", projectIDForTests),
			"--json",
		)

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()

		var tests []v6cloudapi.LoadTest
		require.NoError(t, json.Unmarshal([]byte(stdout), &tests))
		assert.Equal(t, testLoadTests(), tests)
	})

	t.Run("--json with empty list outputs empty array", func(t *testing.T) {
		t.Parallel()

		ts := newCloudTestListTestState(t, nil,
			"--project-id", fmt.Sprintf("%d", projectIDForTests),
			"--json",
		)

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()

		var tests []v6cloudapi.LoadTest
		require.NoError(t, json.Unmarshal([]byte(stdout), &tests))
		assert.JSONEq(t, `[]`, stdout)
		assert.Empty(t, tests)
	})
}

func testLoadTests() []v6cloudapi.LoadTest {
	return []v6cloudapi.LoadTest{
		{
			ID:        8410,
			ProjectID: projectIDForTests,
			Name:      "checkout-flow",
			Created:   time.Date(2026, 4, 1, 9, 12, 0, 0, time.UTC),
			Updated:   time.Date(2026, 4, 25, 14, 3, 0, 0, time.UTC),
		},
		{
			ID:        8421,
			ProjectID: projectIDForTests,
			Name:      "api-smoke",
			Created:   time.Date(2026, 4, 2, 10, 0, 0, 0, time.UTC),
			Updated:   time.Date(2026, 4, 28, 9, 15, 0, 0, time.UTC),
		},
	}
}

func newCloudTestListTestState(
	t *testing.T, loadTests []v6cloudapi.LoadTest, args ...string,
) *GlobalTestState {
	t.Helper()

	srv := v6test.NewServer(t, v6test.Config{LoadTests: loadTests})

	ts := NewGlobalTestState(t)
	ts.CmdArgs = append([]string{"k6", "cloud", "test", "list"}, args...)
	ts.Env["K6_CLOUD_TOKEN"] = validToken
	ts.Env["K6_CLOUD_STACK_ID"] = fmt.Sprintf("%d", validStackID)
	ts.Env["K6_CLOUD_HOST_V6"] = srv.URL

	return ts
}
