package tests

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cloudapiv6 "go.k6.io/k6/v2/internal/cloudapi/v6"
	"go.k6.io/k6/v2/internal/cloudapi/v6/v6test"
	"go.k6.io/k6/v2/internal/cmd"
)

func TestCloudProjectList(t *testing.T) {
	t.Parallel()

	t.Run("lists projects successfully", func(t *testing.T) {
		t.Parallel()

		ts := newCloudProjectListTestState(t, testProjects())

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		assert.Contains(t, stdout, fmt.Sprintf("Projects for stack-%d:", validStackID))
		assert.Contains(t, stdout, "ID   NAME              DEFAULT")
		assert.Contains(t, stdout, "1    Default project   yes")
		assert.Contains(t, stdout, "2    My project        no")
	})

	t.Run("empty project list", func(t *testing.T) {
		t.Parallel()

		ts := newCloudProjectListTestState(t, nil)

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		assert.Contains(t, stdout, fmt.Sprintf("Projects for stack-%d:", validStackID))
		assert.Contains(t, stdout, "No projects found.")
		assert.Contains(t, stdout, "https://grafana.com/docs/grafana-cloud/testing/k6/projects-and-users/projects/")
	})

	t.Run("--help includes usage", func(t *testing.T) {
		t.Parallel()

		ts := NewGlobalTestState(t)
		ts.CmdArgs = []string{"k6", "cloud", "project", "list", "--help"}

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		assert.Contains(t, stdout, "Usage:\n  k6 cloud project list [flags]")
		assert.Contains(t, stdout, "--json")
		assert.NotContains(t, stdout, "Global Flags:")
		assert.NotContains(t, stdout, "--config")
		assert.NotContains(t, stdout, "\n\n\nExamples:")
	})

	t.Run("--json outputs JSON", func(t *testing.T) {
		t.Parallel()

		ts := newCloudProjectListTestState(t, testProjects(), "--json")

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()

		var projects []cloudapiv6.Project
		require.NoError(t, json.Unmarshal([]byte(stdout), &projects))
		assert.Equal(t, testProjects(), projects)
	})

	t.Run("--json with empty list outputs empty array", func(t *testing.T) {
		t.Parallel()

		ts := newCloudProjectListTestState(t, nil, "--json")

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()

		var projects []cloudapiv6.Project
		require.NoError(t, json.Unmarshal([]byte(stdout), &projects))
		assert.Empty(t, projects)
	})
}

func testProjects() []cloudapiv6.Project {
	return []cloudapiv6.Project{
		{ID: 1, Name: "Default project", IsDefault: true},
		{ID: 2, Name: "My project", IsDefault: false},
	}
}

func newCloudProjectListTestState(
	t *testing.T, projects []cloudapiv6.Project, args ...string,
) *GlobalTestState {
	t.Helper()

	srv := v6test.NewServer(t, v6test.Config{Projects: projects})

	ts := NewGlobalTestState(t)
	ts.CmdArgs = append([]string{"k6", "cloud", "project", "list"}, args...)
	ts.Env["K6_CLOUD_TOKEN"] = validToken
	ts.Env["K6_CLOUD_STACK_ID"] = fmt.Sprintf("%d", validStackID)
	ts.Env["K6_CLOUD_HOST_V6"] = srv.URL

	return ts
}
