package cloudapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	k6cloud "github.com/grafana/k6-cloud-openapi-client-go/k6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/internal/build"
	"go.k6.io/k6/v2/internal/lib/testutils"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/lib/fsext"
	"go.k6.io/k6/v2/lib/types"
	"gopkg.in/guregu/null.v3"
)

func TestValidateToken(t *testing.T) {
	t.Parallel()

	t.Run("successful token validation", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify the authorization header
			authHeader := r.Header.Get("Authorization")
			assert.Equal(t, "Bearer test-token", authHeader)

			// Verify the stack URL
			stackURL := r.Header.Get("X-Stack-Url")
			assert.Equal(t, stackURL, "https://stack.grafana.net")

			w.Header().Add("Content-Type", "application/json")
			fprint(t, w, `{
				"stack_id": 123,
				"default_project_id": 456
			}`)
		}))
		defer server.Close()

		client, err := NewClient(testutils.NewLogger(t), "test-token", server.URL, "1.0", 1*time.Second)
		require.NoError(t, err)

		resp, err := client.ValidateToken(t.Context(), "https://stack.grafana.net")
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, int32(123), resp.StackId)
		assert.Equal(t, int32(456), resp.DefaultProjectId)
	})

	t.Run("unauthorized token should fail", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			writeError(t, w, http.StatusUnauthorized, "error", "Invalid token")
		}))
		defer server.Close()

		client, err := NewClient(testutils.NewLogger(t), "invalid-token", server.URL, "1.0", 1*time.Second)
		require.NoError(t, err)

		resp, err := client.ValidateToken(t.Context(), "https://stack.grafana.net")
		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "(401/error) Invalid token")
	})

	t.Run("network error should fail", func(t *testing.T) {
		t.Parallel()
		// Use an invalid URL to simulate network error
		client, err := NewClient(testutils.NewLogger(t), "test-token", "http://invalid-url-that-does-not-exist", "1.0", 1*time.Second)
		require.NoError(t, err)

		resp, err := client.ValidateToken(t.Context(), "https://stack.grafana.net")
		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("missing stack URL should fail", func(t *testing.T) {
		t.Parallel()
		client, err := NewClient(testutils.NewLogger(t), "test-token", "http://example.com", "1.0", 1*time.Second)
		require.NoError(t, err)

		resp, err := client.ValidateToken(t.Context(), "")
		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, "stack URL is required to validate token", err.Error())
	})

	t.Run("invalid stack URL should fail", func(t *testing.T) {
		t.Parallel()
		client, err := NewClient(testutils.NewLogger(t), "test-token", "http://example.com", "1.0", 1*time.Second)
		require.NoError(t, err)

		resp, err := client.ValidateToken(t.Context(), "://invalid-url")
		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "invalid stack URL")
	})
}

func TestListProjects(t *testing.T) {
	t.Parallel()

	project := func(id int32, name string, isDefault bool) map[string]any {
		return map[string]any{
			"id":                 id,
			"name":               name,
			"is_default":         isDefault,
			"grafana_folder_uid": nil,
			"created":            "2025-01-01T00:00:00Z",
			"updated":            "2025-01-01T00:00:00Z",
		}
	}

	var requests atomic.Int32
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "1000", r.URL.Query().Get("$top"))

		switch requests.Add(1) {
		case 1:
			assert.Equal(t, "0", r.URL.Query().Get("$skip"))
			writeJSON(t, w, http.StatusOK, map[string]any{
				"value":     []any{project(1, "Default project", true)},
				"@nextLink": "https://api.k6.io/cloud/v6/projects?$skip=1000&$top=1000",
			})
		case 2:
			assert.Equal(t, "1000", r.URL.Query().Get("$skip"))
			writeJSON(t, w, http.StatusOK, map[string]any{
				"value": []any{project(2, "My project", false)},
			})
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))

	projects, err := client.ListProjects(t.Context())
	require.NoError(t, err)
	require.Len(t, projects, 2)
	assert.Equal(t, int32(2), requests.Load())
	assert.Equal(t, Project{ID: 1, Name: "Default project", IsDefault: true}, projects[0])
	assert.Equal(t, Project{ID: 2, Name: "My project", IsDefault: false}, projects[1])
}

func TestRetryWithConnectionClose(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32
	var body []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) == 1 {
			w.Header().Set("Connection", "close")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		var err error
		body, err = io.ReadAll(r.Body)
		assert.NoError(t, err)
		writeJSON(t, w, http.StatusOK, map[string]any{})
	}))
	defer srv.Close()

	client, err := NewClient(testutils.NewLogger(t), "test-token", srv.URL, "1.0", 5*time.Second)
	require.NoError(t, err)
	client.SetStackID(1)

	opts := lib.Options{VUs: null.IntFrom(10)}
	require.NoError(t, client.ValidateOptions(t.Context(), 1, opts))

	assert.Equal(t, int32(2), attempts.Load(), "expected exactly 2 attempts")

	var got struct {
		Options json.RawMessage `json:"options"`
	}
	require.NoError(t, json.Unmarshal(body, &got))
	want, err := json.Marshal(opts)
	require.NoError(t, err)
	assert.JSONEq(t, string(want), string(got.Options))
}

func TestValidateOptions(t *testing.T) {
	t.Parallel()

	var body struct {
		Options   json.RawMessage `json:"options"`
		ProjectID int32           `json:"project_id"`
	}
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		writeJSON(t, w, http.StatusOK, map[string]any{}) // the actual response body is not relevant
	}))

	opts := lib.Options{
		VUs:      null.IntFrom(5),
		Duration: types.NullDurationFrom(10 * time.Second),
		Stages:   []lib.Stage{{Duration: types.NullDurationFrom(30 * time.Second), Target: null.IntFrom(20)}},
		RunTags:  map[string]string{"env": "staging"},
	}
	require.NoError(t, client.ValidateOptions(t.Context(), 42, opts))

	want, err := json.Marshal(opts)
	require.NoError(t, err)
	assert.JSONEq(t, string(want), string(body.Options))
	assert.Equal(t, int32(42), body.ProjectID)
}

func TestUploadTest(t *testing.T) {
	t.Parallel()

	newTestArchive := func(t *testing.T) *lib.Archive {
		t.Helper()

		fs := fsext.NewMemMapFs()
		require.NoError(t, fsext.WriteFile(fs, "/a.js", []byte(`// c`), 0o644))
		return &lib.Archive{
			Type:        "js",
			K6Version:   build.Version,
			FilenameURL: &url.URL{Scheme: "file", Path: "/a.js"},
			PwdURL:      &url.URL{Scheme: "file", Path: "/"},
			Data:        []byte(`// c`),
			Filesystems: map[string]fsext.Fs{"file": fs},
		}
	}

	loadTest := k6cloud.NewLoadTestApiModelWithDefaults()
	loadTest.SetId(42)

	t.Run("creates new on success", func(t *testing.T) {
		t.Parallel()

		var script []byte
		client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !assert.NoError(t, r.ParseMultipartForm(1<<20)) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			f, _, err := r.FormFile("script")
			if !assert.NoError(t, err) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			script, err = io.ReadAll(f)
			if !assert.NoError(t, err) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			writeJSON(t, w, http.StatusCreated, loadTest)
		}))
		arc := newTestArchive(t)
		got, err := client.UploadTest(t.Context(), "test", 1, arc)
		require.NoError(t, err)
		assertJSONEqual(t, loadTest, got)
		assertArchiveEqual(t, arc, script)
	})

	t.Run("updates existing on 409", func(t *testing.T) {
		t.Parallel()

		var script []byte
		client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodPost:
				writeError(t, w, http.StatusConflict, "conflict", "already exists")
			case http.MethodGet:
				writeJSON(t, w, http.StatusOK, map[string]any{"value": []any{loadTest}})
			case http.MethodPut:
				var err error
				script, err = io.ReadAll(r.Body)
				if !assert.NoError(t, err) {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				w.WriteHeader(http.StatusNoContent)
			}
		}))
		arc := newTestArchive(t)
		got, err := client.UploadTest(t.Context(), "test", 1, arc)
		require.NoError(t, err)
		assertJSONEqual(t, loadTest, got)
		assertArchiveEqual(t, arc, script)
	})

	t.Run("fails when not found", func(t *testing.T) {
		t.Parallel()

		client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodPost:
				writeError(t, w, http.StatusConflict, "conflict", "already exists")
			case http.MethodGet:
				writeJSON(t, w, http.StatusOK, map[string]any{"value": []any{}})
			}
		}))
		_, err := client.UploadTest(t.Context(), "test", 1, newTestArchive(t))
		assert.ErrorIs(t, err, errTestNotExists)
	})
}

func TestStartTest(t *testing.T) {
	t.Parallel()

	res := k6cloud.NewStartLoadTestResponseWithDefaults()
	res.SetId(7)
	res.SetDistribution([]k6cloud.DistributionZoneApiModel{})
	res.SetResultDetails(map[string]any{})
	res.SetOptions(map[string]any{})

	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/42/start")
		assert.Regexpf(t, `^[0-9a-f]{16}$`, r.Header.Get("K6-Idempotency-Key"),
			"missing or invalid K6-Idempotency-Key header")
		writeJSON(t, w, http.StatusOK, res)
	}))

	got, err := client.StartTest(t.Context(), 42)
	require.NoError(t, err)
	assertJSONEqual(t, res, got)
}

func TestStopTest(t *testing.T) {
	t.Parallel()

	t.Run("no content succeeds", func(t *testing.T) {
		t.Parallel()
		client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Contains(t, r.URL.Path, "/42/abort")
			w.WriteHeader(http.StatusNoContent)
		}))
		require.NoError(t, client.StopTest(t.Context(), 42))
	})
	t.Run("already stopped is swallowed", func(t *testing.T) {
		t.Parallel()
		client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Contains(t, r.URL.Path, "/42/abort")
			writeError(t, w, http.StatusConflict, "conflict", "already stopped")
		}))
		require.NoError(t, client.StopTest(t.Context(), 42))
	})
}

func TestResponseErrorWithNullTargets(t *testing.T) {
	t.Parallel()

	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		fprint(t, w, `{
			"error": {
				"code": "bad_request",
				"message": "invalid request",
				"target": null,
				"details": [{
					"code": "invalid",
					"message": "invalid field",
					"target": null
				}]
			}
		}`)
	}))

	err := client.ValidateOptions(t.Context(), 1, lib.Options{})
	require.Error(t, err)
	assert.Equal(t, "(400/bad_request) invalid request\ninvalid field", err.Error())
}

func TestFetchTest(t *testing.T) {
	t.Parallel()

	want := &TestProgress{
		Status:            StatusRunning,
		Result:            ResultFailed,
		EstimatedDuration: 120,
		ExecutionDuration: 60,
		StatusHistory: []StatusEvent{{
			Status:  StatusCreated,
			Entered: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			ByUser:  "u@e.com",
			Code:    3,
			Message: "boom",
		}},
	}

	res := k6cloud.NewTestRunApiModelWithDefaults()
	res.SetStatus(want.Status.String())
	res.SetResult(want.Result.String())
	res.SetEstimatedDuration(want.EstimatedDuration)
	res.SetExecutionDuration(want.ExecutionDuration)
	res.SetStatusHistory(ToStatusModel(want.StatusHistory))
	res.SetDistribution([]k6cloud.DistributionZoneApiModel{*k6cloud.NewDistributionZoneApiModelWithDefaults()})
	res.SetResultDetails(map[string]any{"foo": "bar"})
	res.SetOptions(map[string]any{"vus": 10})

	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/test_runs/42")
		writeJSON(t, w, http.StatusOK, res)
	}))

	got, err := client.FetchTest(t.Context(), 42)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func newTestClient(t *testing.T, handler http.Handler) *Client {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	client, err := NewClient(testutils.NewLogger(t), "test-token", srv.URL, "1.0", 5*time.Second)
	require.NoError(t, err)
	client.SetStackID(1)

	return client
}

func writeJSON(t *testing.T, w http.ResponseWriter, status int, v any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	assert.NoError(t, json.NewEncoder(w).Encode(v))
}

func writeError(t *testing.T, w http.ResponseWriter, status int, code, msg string) {
	t.Helper()

	writeJSON(t, w, status, map[string]any{
		"error": map[string]string{"code": code, "message": msg},
	})
}

func fprint(t *testing.T, w io.Writer, format string) int {
	n, err := fmt.Fprint(w, format)
	assert.NoError(t, err)
	return n
}

func assertJSONEqual(t *testing.T, want, got any) {
	t.Helper()
	w, err := json.Marshal(want)
	require.NoError(t, err)
	g, err := json.Marshal(got)
	require.NoError(t, err)
	assert.JSONEq(t, string(w), string(g))
}

func assertArchiveEqual(t *testing.T, want *lib.Archive, got []byte) {
	t.Helper()
	var b bytes.Buffer
	require.NoError(t, want.Write(&b))
	assert.Equal(t, b.Bytes(), got)
}
