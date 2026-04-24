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

func TestCreateOrFindLoadTest_CreateSuccess(t *testing.T) {
	t.Parallel()

	lt := k6cloud.NewLoadTestApiModelWithDefaults()
	lt.SetId(42)
	lt.SetProjectId(1)
	lt.SetName("my-test")

	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		assert.Equal(t, "1", r.Header.Get("X-Stack-Id"))

		require.NoError(t, r.ParseMultipartForm(1<<20))
		assert.Equal(t, "my-test", r.FormValue("name"))
		assert.Empty(t, r.MultipartForm.File["script"], "script field must not be present")

		writeJSON(t, w, http.StatusCreated, lt)
	}))

	id, err := client.CreateOrFindLoadTest(t.Context(), 1, "my-test")
	require.NoError(t, err)
	assert.Equal(t, int32(42), id)
}

func TestCreateOrFindLoadTest_Conflict409FallbackFound(t *testing.T) {
	t.Parallel()

	lt := k6cloud.NewLoadTestApiModelWithDefaults()
	lt.SetId(99)
	lt.SetProjectId(1)
	lt.SetName("my-test")

	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			writeError(t, w, http.StatusConflict, "conflict", "already exists")
		case http.MethodGet:
			writeJSON(t, w, http.StatusOK, map[string]any{"value": []any{lt}})
		}
	}))

	id, err := client.CreateOrFindLoadTest(t.Context(), 1, "my-test")
	require.NoError(t, err)
	assert.Equal(t, int32(99), id)
}

// capturedRequest holds the HTTP request fields captured by a mock handler.
type capturedRequest struct {
	method  string
	path    string
	headers http.Header
	body    []byte
}

func TestStartLocalExecution(t *testing.T) {
	t.Parallel()

	// Shared happy-path mock response, covering all fields asserted by any row.
	archiveUploadURL := "https://storage.example.com/upload/abc123"
	fullResponse := map[string]any{
		"test_run_id":               int64(999),
		"archive_upload_url":        archiveUploadURL,
		"test_run_details_page_url": "https://app.grafana.com/test-runs/999",
		"runtime_config": map[string]any{
			"test_run_token": "tok-abc",
			"metrics": map[string]any{
				"push_url":                "https://metrics.example.com/push",
				"push_interval":           "5s",
				"push_concurrency":        int64(2),
				"aggregation_period":      "30s",
				"aggregation_wait_period": "5s",
				"aggregation_min_samples": int64(100),
				"max_samples_per_package": int64(1000),
			},
			"traces": map[string]any{"push_url": "https://traces.example.com/push"},
			"files":  map[string]any{"push_url": "https://files.example.com/push"},
			"logs":   map[string]any{"push_url": "https://logs.example.com/push", "tail_url": "https://logs.example.com/tail"},
		},
	}

	tests := []struct {
		name   string
		assert func(t *testing.T, req capturedRequest, resp *StartLocalExecutionResponse)
	}{
		{
			name: "response struct fields",
			assert: func(t *testing.T, _ capturedRequest, resp *StartLocalExecutionResponse) {
				t.Helper()
				assert.Equal(t, int64(999), resp.TestRunID)
				require.NotNil(t, resp.ArchiveUploadURL)
				assert.Equal(t, archiveUploadURL, *resp.ArchiveUploadURL)
				assert.Equal(t, "https://app.grafana.com/test-runs/999", resp.TestRunDetailsPageURL)
				assert.Equal(t, "tok-abc", resp.RuntimeConfig.TestRunToken)
				assert.Equal(t, "https://metrics.example.com/push", resp.RuntimeConfig.Metrics.PushURL)
				require.NotNil(t, resp.RuntimeConfig.Metrics.PushInterval)
				assert.Equal(t, "5s", *resp.RuntimeConfig.Metrics.PushInterval)
				require.NotNil(t, resp.RuntimeConfig.Metrics.PushConcurrency)
				assert.Equal(t, int64(2), *resp.RuntimeConfig.Metrics.PushConcurrency)
				assert.Equal(t, "https://traces.example.com/push", resp.RuntimeConfig.Traces.PushURL)
				assert.Equal(t, "https://files.example.com/push", resp.RuntimeConfig.Files.PushURL)
				assert.Equal(t, "https://logs.example.com/push", resp.RuntimeConfig.Logs.PushURL)
				assert.Equal(t, "https://logs.example.com/tail", resp.RuntimeConfig.Logs.TailURL)
			},
		},
		{
			name: "request body fields",
			assert: func(t *testing.T, req capturedRequest, _ *StartLocalExecutionResponse) {
				t.Helper()
				var body map[string]any
				require.NoError(t, json.Unmarshal(req.body, &body))
				opts, ok := body["options"].(map[string]any)
				require.True(t, ok, "options should be a map")
				assert.Equal(t, float64(5), opts["vus"])
				assert.Equal(t, "30s", opts["duration"])
				assert.Equal(t, float64(5), body["max_vus"])
				assert.Equal(t, float64(30000), body["total_duration"])
				assert.Equal(t, float64(4096), body["archive_size"], "archive_size should be present and non-null")
			},
		},
		{
			name: "idempotency key header",
			assert: func(t *testing.T, req capturedRequest, _ *StartLocalExecutionResponse) {
				t.Helper()
				key := req.headers.Get("K6-Idempotency-Key")
				assert.NotEmpty(t, key, "K6-Idempotency-Key header must be present")
				assert.GreaterOrEqual(t, len(key), 1)
				assert.LessOrEqual(t, len(key), 36)
				assert.Regexp(t, `^[0-9a-f]+$`, key, "K6-Idempotency-Key must be hex")
			},
		},
		{
			name: "bearer auth",
			assert: func(t *testing.T, req capturedRequest, _ *StartLocalExecutionResponse) {
				t.Helper()
				assert.Equal(t, "Bearer test-token", req.headers.Get("Authorization"),
					"Authorization header must use Bearer scheme, not Token scheme")
			},
		},
		{
			name: "x-stack-id header",
			assert: func(t *testing.T, req capturedRequest, _ *StartLocalExecutionResponse) {
				t.Helper()
				assert.Equal(t, "1", req.headers.Get("X-Stack-Id"),
					"X-Stack-Id header must be set to the configured stack ID")
			},
		},
	}

	archiveSize := int64(4096)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var captured capturedRequest
			client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				captured.method = r.Method
				captured.path = r.URL.Path
				captured.headers = r.Header.Clone()
				var err error
				captured.body, err = io.ReadAll(r.Body)
				require.NoError(t, err)
				writeJSON(t, w, http.StatusOK, fullResponse)
			}))

			req := StartLocalExecutionRequest{
				Options:       map[string]any{"vus": 5, "duration": "30s"},
				MaxVUs:        5,
				TotalDuration: 30000,
				ArchiveSize:   &archiveSize,
			}

			resp, err := client.StartLocalExecution(t.Context(), 42, req)
			require.NoError(t, err)
			require.NotNil(t, resp)

			tt.assert(t, captured, resp)
		})
	}
}

func TestUploadArchive(t *testing.T) {
	t.Parallel()

	arc := newTestArchive(t)
	var arcBuf bytes.Buffer
	require.NoError(t, arc.Write(&arcBuf))
	expectedBody := arcBuf.Bytes()

	type capturedUpload struct {
		method      string
		contentType string
		auth        string
		body        []byte
	}

	tests := []struct {
		name            string
		serverStatus    int
		wantErr         bool
		wantErrContains string
		checkReq        func(t *testing.T, req capturedUpload)
	}{
		{
			name:         "PUT with correct content-type and body",
			serverStatus: http.StatusOK,
			checkReq: func(t *testing.T, req capturedUpload) {
				t.Helper()
				assert.Equal(t, http.MethodPut, req.method)
				assert.Equal(t, "application/x-tar", req.contentType)
				assert.Equal(t, expectedBody, req.body)
			},
		},
		{
			name:         "no authorization header on S3 presigned PUT",
			serverStatus: http.StatusOK,
			checkReq: func(t *testing.T, req capturedUpload) {
				t.Helper()
				assert.Empty(t, req.auth, "Authorization header must NOT be set on S3 presigned PUT")
			},
		},
		{
			name:            "error propagation on non-2xx",
			serverStatus:    http.StatusForbidden,
			wantErr:         true,
			wantErrContains: "403",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var captured capturedUpload
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				captured.method = r.Method
				captured.contentType = r.Header.Get("Content-Type")
				captured.auth = r.Header.Get("Authorization")
				var err error
				captured.body, err = io.ReadAll(r.Body)
				require.NoError(t, err)
				w.WriteHeader(tt.serverStatus)
			}))
			defer server.Close()

			client, err := NewClient(testutils.NewLogger(t), "test-token", server.URL, "1.0", 5*time.Second)
			require.NoError(t, err)

			bodyBytes := make([]byte, len(expectedBody))
			copy(bodyBytes, expectedBody)

			err = client.UploadArchive(t.Context(), server.URL, bodyBytes)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrContains != "" {
					assert.Contains(t, err.Error(), tt.wantErrContains)
				}
			} else {
				require.NoError(t, err)
			}

			if tt.checkReq != nil {
				tt.checkReq(t, captured)
			}
		})
	}
}
