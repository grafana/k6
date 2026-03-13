package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/cmd"
)

func TestCloudProjectList(t *testing.T) {
	t.Parallel()

	t.Run("lists projects successfully", func(t *testing.T) {
		t.Parallel()

		srv := mockProjectListServer(t)
		defer srv.Close()

		ts := NewGlobalTestState(t)
		ts.CmdArgs = []string{"k6", "cloud", "project", "list"}
		ts.Env["K6_CLOUD_TOKEN"] = validToken
		ts.Env["K6_CLOUD_STACK_ID"] = fmt.Sprintf("%d", validStackID)
		ts.Env["K6_CLOUD_HOST_V6"] = srv.URL

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		assert.Contains(t, stdout, fmt.Sprintf("Projects for stack %d:", validStackID))
		assert.Contains(t, stdout, "ID   NAME              DEFAULT")
		assert.Contains(t, stdout, "1    Default project   yes")
		assert.Contains(t, stdout, "2    My project        no")
	})

	t.Run("fails without token", func(t *testing.T) {
		t.Parallel()

		ts := NewGlobalTestState(t)
		ts.CmdArgs = []string{"k6", "cloud", "project", "list"}
		ts.Env["K6_CLOUD_STACK_ID"] = fmt.Sprintf("%d", validStackID)
		ts.ExpectedExitCode = -1

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stderr := ts.Stderr.String()
		assert.Contains(t, stderr, "authenticate")
	})

	t.Run("fails without stack ID", func(t *testing.T) {
		t.Parallel()

		ts := NewGlobalTestState(t)
		ts.CmdArgs = []string{"k6", "cloud", "project", "list"}
		ts.Env["K6_CLOUD_TOKEN"] = validToken
		ts.ExpectedExitCode = -1

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stderr := ts.Stderr.String()
		assert.Contains(t, stderr, "no stack configured")
	})

	t.Run("empty project list", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, err := fmt.Fprint(w, `{"value": []}`)
			require.NoError(t, err)
		}))
		defer srv.Close()

		ts := NewGlobalTestState(t)
		ts.CmdArgs = []string{"k6", "cloud", "project", "list"}
		ts.Env["K6_CLOUD_TOKEN"] = validToken
		ts.Env["K6_CLOUD_STACK_ID"] = fmt.Sprintf("%d", validStackID)
		ts.Env["K6_CLOUD_HOST_V6"] = srv.URL

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		assert.Contains(t, stdout, fmt.Sprintf("Projects for stack %d:", validStackID))
		assert.Contains(t, stdout, "No projects found.")
		assert.Contains(t, stdout, "https://grafana.com/docs/grafana-cloud/testing/k6/projects/")
	})

	t.Run("--json outputs JSON", func(t *testing.T) {
		t.Parallel()

		srv := mockProjectListServer(t)
		defer srv.Close()

		ts := NewGlobalTestState(t)
		ts.CmdArgs = []string{"k6", "cloud", "project", "list", "--json"}
		ts.Env["K6_CLOUD_TOKEN"] = validToken
		ts.Env["K6_CLOUD_STACK_ID"] = fmt.Sprintf("%d", validStackID)
		ts.Env["K6_CLOUD_HOST_V6"] = srv.URL

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()

		var projects []map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &projects))
		require.Len(t, projects, 2)

		assert.Equal(t, float64(1), projects[0]["id"])
		assert.Equal(t, "Default project", projects[0]["name"])
		assert.Equal(t, true, projects[0]["is_default"])

		assert.Equal(t, float64(2), projects[1]["id"])
		assert.Equal(t, "My project", projects[1]["name"])
		assert.Equal(t, false, projects[1]["is_default"])
	})

	t.Run("--json with empty list outputs empty array", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, err := fmt.Fprint(w, `{"value": []}`)
			require.NoError(t, err)
		}))
		defer srv.Close()

		ts := NewGlobalTestState(t)
		ts.CmdArgs = []string{"k6", "cloud", "project", "list", "--json"}
		ts.Env["K6_CLOUD_TOKEN"] = validToken
		ts.Env["K6_CLOUD_STACK_ID"] = fmt.Sprintf("%d", validStackID)
		ts.Env["K6_CLOUD_HOST_V6"] = srv.URL

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()

		var projects []map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &projects))
		assert.Empty(t, projects)
	})
}

func mockProjectListServer(t *testing.T) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cloud/v6/projects" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader != fmt.Sprintf("Bearer %s", validToken) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, err := fmt.Fprint(w, `{
			"value": [
				{
					"id": 1,
					"name": "Default project",
					"is_default": true,
					"grafana_folder_uid": null,
					"created": "2025-01-01T00:00:00Z",
					"updated": "2025-01-01T00:00:00Z"
				},
				{
					"id": 2,
					"name": "My project",
					"is_default": false,
					"grafana_folder_uid": null,
					"created": "2025-01-02T00:00:00Z",
					"updated": "2025-01-02T00:00:00Z"
				}
			]
		}`)
		require.NoError(t, err)
	}))
}
