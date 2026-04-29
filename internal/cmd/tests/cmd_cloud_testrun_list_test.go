package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/cmd"
)

const loadTestIDForTests = 8410

func TestCloudTestRunList(t *testing.T) {
	t.Parallel()

	t.Run("lists runs successfully", func(t *testing.T) {
		t.Parallel()

		srv := mockTestRunListServer(t, nil)

		ts := NewGlobalTestState(t)
		ts.CmdArgs = []string{
			"k6", "cloud", "test-run", "list",
			"--test-id", fmt.Sprintf("%d", loadTestIDForTests),
		}
		ts.Env["K6_CLOUD_TOKEN"] = validToken
		ts.Env["K6_CLOUD_STACK_ID"] = fmt.Sprintf("%d", validStackID)
		ts.Env["K6_CLOUD_HOST_V6"] = srv.URL

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

	t.Run("fails without token", func(t *testing.T) {
		t.Parallel()

		ts := NewGlobalTestState(t)
		ts.CmdArgs = []string{
			"k6", "cloud", "test-run", "list",
			"--test-id", fmt.Sprintf("%d", loadTestIDForTests),
		}
		ts.Env["K6_CLOUD_STACK_ID"] = fmt.Sprintf("%d", validStackID)
		ts.ExpectedExitCode = -1

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stderr := ts.Stderr.String()
		assert.Contains(t, stderr, "authenticate")
	})

	t.Run("fails without stack ID", func(t *testing.T) {
		t.Parallel()

		ts := NewGlobalTestState(t)
		ts.CmdArgs = []string{
			"k6", "cloud", "test-run", "list",
			"--test-id", fmt.Sprintf("%d", loadTestIDForTests),
		}
		ts.Env["K6_CLOUD_TOKEN"] = validToken
		ts.ExpectedExitCode = -1

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stderr := ts.Stderr.String()
		assert.Contains(t, stderr, "no stack configured")
	})

	t.Run("--limit caps results without paging", func(t *testing.T) {
		t.Parallel()

		var requests atomic.Int32
		srv := mockTestRunListServer(t, &mockOptions{
			onRequest: func(_ *http.Request) {
				requests.Add(1)
			},
			expectedTop: "5",
		})

		ts := NewGlobalTestState(t)
		ts.CmdArgs = []string{
			"k6", "cloud", "test-run", "list",
			"--test-id", fmt.Sprintf("%d", loadTestIDForTests),
			"--limit", "5",
		}
		ts.Env["K6_CLOUD_TOKEN"] = validToken
		ts.Env["K6_CLOUD_STACK_ID"] = fmt.Sprintf("%d", validStackID)
		ts.Env["K6_CLOUD_HOST_V6"] = srv.URL

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		assert.Equal(t, int32(1), requests.Load(), "expected exactly one page request")
	})

	t.Run("--all paginates", func(t *testing.T) {
		t.Parallel()

		var requests atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "1000", r.URL.Query().Get("$top"))

			switch requests.Add(1) {
			case 1:
				w.Header().Set("Content-Type", "application/json")
				_, err := fmt.Fprintf(w, `{
					"value": [%s],
					"@nextLink": "%s/cloud/v6/load_tests/%d/test_runs?$skip=1000&$top=1000"
				}`, sampleTestRunJSON(1009852, "finished", "passed"), r.Host, loadTestIDForTests)
				require.NoError(t, err)
			case 2:
				w.Header().Set("Content-Type", "application/json")
				_, err := fmt.Fprintf(w, `{"value": [%s]}`,
					sampleTestRunJSON(1009711, "finished", "failed"))
				require.NoError(t, err)
			default:
				w.WriteHeader(http.StatusInternalServerError)
			}
		}))
		t.Cleanup(srv.Close)

		ts := NewGlobalTestState(t)
		ts.CmdArgs = []string{
			"k6", "cloud", "test-run", "list",
			"--test-id", fmt.Sprintf("%d", loadTestIDForTests),
			"--all",
		}
		ts.Env["K6_CLOUD_TOKEN"] = validToken
		ts.Env["K6_CLOUD_STACK_ID"] = fmt.Sprintf("%d", validStackID)
		ts.Env["K6_CLOUD_HOST_V6"] = srv.URL

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		assert.Contains(t, stdout, "1009852")
		assert.Contains(t, stdout, "1009711")
		assert.Equal(t, int32(2), requests.Load())
	})

	t.Run("--since forwards created_after", func(t *testing.T) {
		t.Parallel()

		var seenCreatedAfter atomic.Value
		srv := mockTestRunListServer(t, &mockOptions{
			onRequest: func(r *http.Request) {
				seenCreatedAfter.Store(r.URL.Query().Get("created_after"))
			},
		})

		ts := NewGlobalTestState(t)
		ts.CmdArgs = []string{
			"k6", "cloud", "test-run", "list",
			"--test-id", fmt.Sprintf("%d", loadTestIDForTests),
			"--since", "1h",
		}
		ts.Env["K6_CLOUD_TOKEN"] = validToken
		ts.Env["K6_CLOUD_STACK_ID"] = fmt.Sprintf("%d", validStackID)
		ts.Env["K6_CLOUD_HOST_V6"] = srv.URL

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		got, _ := seenCreatedAfter.Load().(string)
		assert.NotEmpty(t, got, "expected created_after to be set")
	})

	t.Run("empty list", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, err := fmt.Fprint(w, `{"value": []}`)
			require.NoError(t, err)
		}))
		defer srv.Close()

		ts := NewGlobalTestState(t)
		ts.CmdArgs = []string{
			"k6", "cloud", "test-run", "list",
			"--test-id", fmt.Sprintf("%d", loadTestIDForTests),
		}
		ts.Env["K6_CLOUD_TOKEN"] = validToken
		ts.Env["K6_CLOUD_STACK_ID"] = fmt.Sprintf("%d", validStackID)
		ts.Env["K6_CLOUD_HOST_V6"] = srv.URL

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
	})

	t.Run("--json outputs JSON", func(t *testing.T) {
		t.Parallel()

		srv := mockTestRunListServer(t, nil)

		ts := NewGlobalTestState(t)
		ts.CmdArgs = []string{
			"k6", "cloud", "test-run", "list",
			"--test-id", fmt.Sprintf("%d", loadTestIDForTests),
			"--json",
		}
		ts.Env["K6_CLOUD_TOKEN"] = validToken
		ts.Env["K6_CLOUD_STACK_ID"] = fmt.Sprintf("%d", validStackID)
		ts.Env["K6_CLOUD_HOST_V6"] = srv.URL

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()

		var runs []map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &runs))
		require.Len(t, runs, 2)

		assert.Equal(t, float64(1009852), runs[0]["id"])
		assert.Equal(t, "finished", runs[0]["status"])
		assert.Equal(t, "passed", runs[0]["result"])
	})
}

type mockOptions struct {
	onRequest   func(*http.Request)
	expectedTop string
}

func mockTestRunListServer(t *testing.T, opts *mockOptions) *httptest.Server {
	t.Helper()

	expectedPath := fmt.Sprintf("/cloud/v6/load_tests/%d/test_runs", loadTestIDForTests)

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != expectedPath {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader != fmt.Sprintf("Bearer %s", validToken) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if opts != nil {
			if opts.onRequest != nil {
				opts.onRequest(r)
			}
			if opts.expectedTop != "" {
				assert.Equal(t, opts.expectedTop, r.URL.Query().Get("$top"))
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_, err := fmt.Fprintf(w, `{"value": [%s, %s]}`,
			sampleTestRunJSON(1009852, "finished", "passed"),
			sampleTestRunJSON(1009711, "finished", "failed"),
		)
		require.NoError(t, err)
	}))

	t.Cleanup(mockServer.Close)

	return mockServer
}

func sampleTestRunJSON(id int, status, result string) string {
	return fmt.Sprintf(`{
		"id": %[1]d,
		"test_id": %[2]d,
		"project_id": 7,
		"started_by": null,
		"created": "2026-04-28T14:03:00Z",
		"ended": "2026-04-28T14:08:02Z",
		"note": "",
		"retention_expiry": null,
		"cost": null,
		"status": "%[3]s",
		"status_details": {"type": "%[3]s", "entered": "2026-04-28T14:03:00Z"},
		"status_history": [],
		"distribution": [],
		"result": "%[4]s",
		"result_details": {},
		"options": {},
		"k6_dependencies": {},
		"k6_versions": {},
		"max_vus": 50,
		"max_browser_vus": null,
		"estimated_duration": 300,
		"execution_duration": 302
	}`, id, loadTestIDForTests, status, result)
}
