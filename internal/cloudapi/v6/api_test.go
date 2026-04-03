package cloudapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	k6cloud "github.com/grafana/k6-cloud-openapi-client-go/k6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/internal/build"
	"go.k6.io/k6/internal/lib/testutils"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/fsext"
	"go.k6.io/k6/lib/types"
	"gopkg.in/guregu/null.v3"
)

func TestValidateToken(t *testing.T) {
	t.Parallel()

	t.Run("successful", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
			assert.Equal(t, "https://stack.grafana.net", r.Header.Get("X-Stack-Url"))

			w.Header().Add("Content-Type", "application/json")
			fprint(t, w, `{"stack_id": 123, "default_project_id": 456}`)
		}))
		defer srv.Close()

		client := newTestClient(t, srv)
		resp, err := client.ValidateToken(t.Context(), "https://stack.grafana.net")
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, int32(123), resp.StackId)
		assert.Equal(t, int32(456), resp.DefaultProjectId)
	})

	t.Run("unauthorized", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Add("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			fprint(t, w, `{"error": {"code": "error", "message": "Invalid token"}}`)
		}))
		defer srv.Close()

		client := newTestClient(t, srv)
		resp, err := client.ValidateToken(t.Context(), "https://stack.grafana.net")
		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "(401/error) Invalid token")
	})

	t.Run("network error", func(t *testing.T) {
		t.Parallel()
		client, err := NewClient(testutils.NewLogger(t), "test-token", "http://invalid-url-that-does-not-exist", "1.0", 1*time.Second)
		require.NoError(t, err)
		client.retryInterval = time.Millisecond

		resp, err := client.ValidateToken(t.Context(), "https://stack.grafana.net")
		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("missing stack URL", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		defer srv.Close()

		client := newTestClient(t, srv)
		resp, err := client.ValidateToken(t.Context(), "")
		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, "stack URL is required to validate token", err.Error())
	})

	t.Run("invalid stack URL", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		defer srv.Close()

		client := newTestClient(t, srv)
		resp, err := client.ValidateToken(t.Context(), "://invalid-url")
		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "invalid stack URL")
	})
}

func TestValidateOptions(t *testing.T) {
	t.Parallel()

	t.Run("successful", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
			assert.Equal(t, "123", r.Header.Get("X-Stack-Id"))

			b, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			var opts k6cloud.ValidateOptionsRequest
			require.NoError(t, json.Unmarshal(b, &opts))

			duration := opts.Options.AdditionalProperties["duration"]
			assert.Equal(t, "1m0s", duration)

			w.Header().Set("Content-Type", "application/json")
			fprint(t, w, `{}`)
		}))
		defer srv.Close()

		client := newTestClient(t, srv)
		err := client.ValidateOptions(t.Context(), lib.Options{
			Duration: types.NullDurationFrom(60 * time.Second),
		})
		require.NoError(t, err)
	})

	t.Run("validation error", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			fprint(t, w, `{"error": {"code": "error", "message": "Invalid VUs number"}}`)
		}))
		defer srv.Close()

		client := newTestClient(t, srv)
		err := client.ValidateOptions(t.Context(), lib.Options{VUs: null.IntFrom(-1)})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Invalid VUs number")
	})
}

func TestCreateCloudTest(t *testing.T) {
	t.Parallel()

	t.Run("successful", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
			assert.Equal(t, "123", r.Header.Get("X-Stack-Id"))

			formData := parseFormData(t, r)
			assert.Contains(t, r.URL.Path, "456") // projectID from client

			w.Header().Add("Content-Type", "application/json")
			fprint(t, w, fmt.Sprintf(`{
				"id": 789,
				"name": "%s",
				"project_id": 456,
				"baseline_test_run_id": null,
				"created": "2024-01-01T00:00:00Z",
				"updated": "2024-01-01T00:00:00Z"
			}`, formData["name"]))
		}))
		defer srv.Close()

		client := newTestClient(t, srv)
		arcData := createTestArchiveBytes(t)
		result, err := client.CreateCloudTest(t.Context(), "test-name", arcData)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, int32(789), result.Id)
		assert.Equal(t, "test-name", result.Name)
	})
}

func TestFetchCloudTestByName(t *testing.T) {
	t.Parallel()

	t.Run("found", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "test-name", r.URL.Query().Get("name"))
			w.Header().Set("Content-Type", "application/json")
			fprint(t, w, `{
				"value": [{
					"id": 789, "name": "test-name", "project_id": 456,
					"baseline_test_run_id": null,
					"created": "2024-01-01T00:00:00Z", "updated": "2024-01-01T00:00:00Z"
				}]
			}`)
		}))
		defer srv.Close()

		client := newTestClient(t, srv)
		result, err := client.FetchCloudTestByName(t.Context(), "test-name")
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, int32(789), result.Id)
	})

	t.Run("empty result", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fprint(t, w, `{"value": []}`)
		}))
		defer srv.Close()

		client := newTestClient(t, srv)
		_, err := client.FetchCloudTestByName(t.Context(), "my-test")
		require.Error(t, err)
		assert.Contains(t, err.Error(), `"my-test" not found in project`)
	})
}

func TestCreateOrUpdateCloudTest(t *testing.T) {
	t.Parallel()

	t.Run("creates new test", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Add("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			fprint(t, w, `{
				"id": 789, "name": "test-name", "project_id": 456,
				"baseline_test_run_id": null,
				"created": "2024-01-01T00:00:00Z", "updated": "2024-01-01T00:00:00Z"
			}`)
		}))
		defer srv.Close()

		client := newTestClient(t, srv)
		arc := createTestArchive(t)
		result, err := client.CreateOrUpdateCloudTest(t.Context(), "test-name", arc)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, int32(789), result.Id)
	})

	t.Run("updates on conflict", func(t *testing.T) {
		t.Parallel()
		getCalled := false
		updateCalled := false

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.Method {
			case http.MethodPost:
				w.WriteHeader(http.StatusConflict)
				fprint(t, w, `{"error": {"code": "error", "message": "conflict"}}`)
			case http.MethodGet:
				getCalled = true
				fprint(t, w, `{
					"value": [{
						"id": 789, "name": "test-name", "project_id": 456,
						"baseline_test_run_id": null,
						"created": "2024-01-01T00:00:00Z", "updated": "2024-01-01T00:00:00Z"
					}]
				}`)
			case http.MethodPut:
				updateCalled = true
				w.WriteHeader(http.StatusNoContent)
			}
		}))
		defer srv.Close()

		client := newTestClient(t, srv)
		arc := createTestArchive(t)
		result, err := client.CreateOrUpdateCloudTest(t.Context(), "test-name", arc)
		require.NoError(t, err)
		assert.Equal(t, int32(789), result.Id)
		assert.True(t, getCalled)
		assert.True(t, updateCalled)
	})
}

func TestStartCloudTestRun(t *testing.T) {
	t.Parallel()

	t.Run("successful", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
			assert.Equal(t, "123", r.Header.Get("X-Stack-Id"))
			assert.NotEmpty(t, r.Header.Get("K6-Idempotency-Key"))

			w.Header().Set("Content-Type", "application/json")
			fprint(t, w, testRunJSON(t, 999, "created", nil, "https://app.grafana.com/runs/999"))
		}))
		defer srv.Close()

		client := newTestClient(t, srv)
		result, err := client.StartCloudTestRun(t.Context(), 789)
		require.NoError(t, err)
		assert.Equal(t, int32(999), result.Id)
		assert.Equal(t, int32(789), result.TestId)
		assert.Equal(t, "https://app.grafana.com/runs/999", result.TestRunDetailsPageUrl)
	})

	t.Run("idempotency key stable across retries", func(t *testing.T) {
		t.Parallel()
		var keys []string
		attempts := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			keys = append(keys, r.Header.Get("K6-Idempotency-Key"))
			attempts++
			if attempts < 3 {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			fprint(t, w, testRunJSON(t, 999, "created", nil, "https://app.grafana.com/runs/999"))
		}))
		defer srv.Close()

		client := newTestClient(t, srv)
		client.retryInterval = time.Millisecond

		_, err := client.StartCloudTestRun(t.Context(), 789)
		require.NoError(t, err)
		require.Equal(t, 3, attempts)
		require.Len(t, keys, 3)
		assert.Equal(t, keys[0], keys[1], "key must be the same across retries")
		assert.Equal(t, keys[0], keys[2], "key must be the same across retries")
		assert.NotEmpty(t, keys[0])
	})
}

func TestStopCloudTestRun(t *testing.T) {
	t.Parallel()

	t.Run("successful", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Contains(t, r.URL.Path, "999")
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
			assert.Equal(t, "123", r.Header.Get("X-Stack-Id"))
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		client := newTestClient(t, srv)
		err := client.StopCloudTestRun(t.Context(), 999)
		require.NoError(t, err)
	})
}

func TestFetchTestRun(t *testing.T) {
	t.Parallel()

	t.Run("all fields", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
			assert.Equal(t, "123", r.Header.Get("X-Stack-Id"))
			assert.Contains(t, r.URL.Path, "999")

			w.Header().Set("Content-Type", "application/json")
			fprint(t, w, `{
				"id": 999, "test_id": 789, "project_id": 456,
				"started_by": "user@example.com",
				"created": "2024-06-01T19:00:00Z", "ended": null,
				"cost": null, "note": "", "retention_expiry": null,
				"distribution": null, "options": null,
				"result": "passed", "result_details": null,
				"status": "running",
				"status_details": {"type": "running", "entered": "2024-06-01T19:00:00Z"},
				"status_history": [], "k6_dependencies": {}, "k6_versions": {},
				"max_vus": null, "max_browser_vus": null,
				"estimated_duration": 60, "execution_duration": 30
			}`)
		}))
		defer srv.Close()

		client := newTestClient(t, srv)
		progress, err := client.FetchTestRun(t.Context(), 999)
		require.NoError(t, err)
		assert.Equal(t, StatusRunning, progress.Status)
		assert.Equal(t, "passed", progress.Result)
		assert.Equal(t, int32(60), progress.EstimatedDuration)
		assert.Equal(t, int32(30), progress.ExecutionDuration)
		assert.InDelta(t, 0.5, progress.Progress(), 0.01)
	})

	t.Run("retries on 502", func(t *testing.T) {
		t.Parallel()
		attempts := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			attempts++
			if attempts < 3 {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			fprint(t, w, `{
				"id": 999, "test_id": 789, "project_id": 456,
				"started_by": null, "created": "2024-06-01T19:00:00Z",
				"ended": null, "cost": null, "note": "", "retention_expiry": null,
				"distribution": null, "options": null, "result": null,
				"result_details": null, "status": "running",
				"status_details": {"type": "running", "entered": "2024-06-01T19:00:00Z"},
				"status_history": [], "k6_dependencies": {}, "k6_versions": {},
				"max_vus": null, "max_browser_vus": null,
				"estimated_duration": null, "execution_duration": 0
			}`)
		}))
		defer srv.Close()

		client := newTestClient(t, srv)
		client.retryInterval = time.Millisecond

		progress, err := client.FetchTestRun(t.Context(), 999)
		require.NoError(t, err)
		assert.Equal(t, StatusRunning, progress.Status)
		assert.Equal(t, 3, attempts)
	})

	t.Run("context cancelled during retry", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			cancel()
			w.WriteHeader(http.StatusBadGateway)
		}))
		defer srv.Close()

		client := newTestClient(t, srv)
		client.retryInterval = time.Hour

		_, err := client.FetchTestRun(ctx, 999)
		require.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("non-retryable error", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			fprint(t, w, `{"error": {"code": "error", "message": "not found"}}`)
		}))
		defer srv.Close()

		client := newTestClient(t, srv)
		_, err := client.FetchTestRun(t.Context(), 999)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("terminal status", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fprint(t, w, `{
				"id": 999, "test_id": 789, "project_id": 456,
				"started_by": null, "created": "2024-06-01T19:00:00Z",
				"ended": "2024-06-01T19:01:00Z", "cost": null, "note": "",
				"retention_expiry": null, "distribution": null, "options": null,
				"result": "failed", "result_details": null, "status": "completed",
				"status_details": {"type": "completed", "entered": "2024-06-01T19:01:00Z"},
				"status_history": [], "k6_dependencies": {}, "k6_versions": {},
				"max_vus": null, "max_browser_vus": null,
				"estimated_duration": null, "execution_duration": 0
			}`)
		}))
		defer srv.Close()

		client := newTestClient(t, srv)
		progress, err := client.FetchTestRun(t.Context(), 999)
		require.NoError(t, err)
		assert.Equal(t, StatusCompleted, progress.Status)
		assert.Equal(t, ResultFailed, progress.Result)
		assert.True(t, progress.IsTerminal())
	})
}

func TestTestRunProgress(t *testing.T) {
	t.Parallel()

	t.Run("Progress", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name      string
			est, exec int32
			expected  float64
		}{
			{"half done", 60, 30, 0.5},
			{"exceeds estimate", 60, 70, 1.0},
			{"zero estimated", 0, 30, 0.0},
			{"negative execution", 60, -1, 0.0},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				p := TestRunProgress{EstimatedDuration: tt.est, ExecutionDuration: tt.exec}
				assert.InDelta(t, tt.expected, p.Progress(), 0.01)
			})
		}
	})

	t.Run("IsTerminal", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			status   string
			terminal bool
		}{
			{StatusCompleted, true},
			{StatusAborted, true},
			{StatusCreated, false},
			{StatusQueued, false},
			{StatusInitializing, false},
			{StatusRunning, false},
			{StatusProcessingMetrics, false},
		}
		for _, tt := range tests {
			t.Run(tt.status, func(t *testing.T) {
				t.Parallel()
				p := TestRunProgress{Status: tt.status}
				assert.Equal(t, tt.terminal, p.IsTerminal())
			})
		}
	})
}

func TestFormatStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status, display string
	}{
		{StatusCreated, "Created"},
		{StatusQueued, "Queued"},
		{StatusInitializing, "Initializing"},
		{StatusRunning, "Running"},
		{StatusProcessingMetrics, "Processing Metrics"},
		{StatusCompleted, "Completed"},
		{StatusAborted, "Aborted"},
		{"some_unknown_status", "some_unknown_status"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.display, FormatStatus(tt.status))
		})
	}
}

func fprint(t *testing.T, w io.Writer, s string) int {
	t.Helper()
	n, err := fmt.Fprint(w, s)
	require.NoError(t, err)
	return n
}

func createTestArchive(t *testing.T) *lib.Archive {
	t.Helper()
	fs := fsext.NewMemMapFs()
	require.NoError(t, fsext.WriteFile(fs, "/path/to/a.js", []byte(`// a contents`), 0o644))
	return &lib.Archive{
		Type:        "js",
		K6Version:   build.Version,
		Options:     lib.Options{},
		FilenameURL: &url.URL{Scheme: "file", Path: "/path/to/a.js"},
		Data:        []byte(`// a contents`),
		PwdURL:      &url.URL{Scheme: "file", Path: "/path/to"},
		Filesystems: map[string]fsext.Fs{"file": fs},
	}
}

func createTestArchiveBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, createTestArchive(t).Write(&buf))
	return buf.Bytes()
}

var testEpoch = time.Date(2024, 6, 1, 19, 0, 0, 0, time.UTC) //nolint:gochecknoglobals

// testRunJSON builds a StartLoadTestResponse JSON string using the generated
// model constructor so fixtures break at compile time if the spec changes.
func testRunJSON(t *testing.T, id int32, status string, result *string, webAppURL string) string {
	t.Helper()
	m := k6cloud.NewStartLoadTestResponse(
		id, 789, 456,
		*k6cloud.NewNullableString(nil), // started_by
		testEpoch,                       // created
		*k6cloud.NewNullableTime(nil),   // ended
		"",                              // note
		*k6cloud.NewNullableTime(nil),   // retention_expiry
		*k6cloud.NewNullableTestCostApiModel(nil), // cost
		status, // status
		*k6cloud.NewStatusApiModel("created", testEpoch), // status_details
		[]k6cloud.StatusApiModel{},                       // status_history
		[]k6cloud.DistributionZoneApiModel{},             // distribution
		*k6cloud.NewNullableString(result),               // result
		map[string]any{},                                 // result_details
		map[string]any{},                                 // options
		map[string]string{},                              // k6_dependencies
		map[string]string{},                              // k6_versions
		*k6cloud.NewNullableInt32(nil),                   // max_vus
		*k6cloud.NewNullableInt32(nil),                   // max_browser_vus
		*k6cloud.NewNullableInt32(nil),                   // estimated_duration
		0,                                                // execution_duration
		webAppURL,                                        // test_run_details_page_url
	)
	b, err := json.Marshal(m)
	require.NoError(t, err)
	return string(b)
}

func parseFormData(t *testing.T, r *http.Request) map[string]string {
	t.Helper()
	require.NoError(t, r.ParseMultipartForm(32<<20))
	formData := make(map[string]string)
	for key, values := range r.MultipartForm.Value {
		if len(values) > 0 {
			formData[key] = values[0]
		}
	}
	return formData
}
