package cloudapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	k6cloud "github.com/grafana/k6-cloud-openapi-client-go/k6"
	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/errext"
	"go.k6.io/k6/v2/internal/build"
	"go.k6.io/k6/v2/internal/lib/testutils"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/lib/fsext"
	"go.k6.io/k6/v2/lib/types"
	"gopkg.in/guregu/null.v3"
)

// testAbortError is a test helper implementing errext.HasAbortReason.
type testAbortError struct {
	reason errext.AbortReason
	msg    string
}

func (e testAbortError) Error() string                   { return e.msg }
func (e testAbortError) AbortReason() errext.AbortReason { return e.reason }

var _ errext.HasAbortReason = testAbortError{}

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

func newTestArchive(t *testing.T) *lib.Archive {
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
	// lib.Archive.Write uses time.Now() for tar entry ModTimes, so two calls to
	// Write never produce identical bytes.  Parse the received bytes back into an
	// Archive and compare the stable content fields instead.
	parsed, err := lib.ReadArchive(bytes.NewReader(got))
	require.NoError(t, err, "got bytes must be a valid k6 archive")
	assert.Equal(t, want.Type, parsed.Type, "archive type must match")
	assert.Equal(t, want.K6Version, parsed.K6Version, "archive k6 version must match")
	assert.Equal(t, want.Data, parsed.Data, "archive script content must match")
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
			name: "response_struct_fields",
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
			name: "request_body_fields",
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
			name: "idempotency_key_header",
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
			name: "bearer_auth",
			assert: func(t *testing.T, req capturedRequest, _ *StartLocalExecutionResponse) {
				t.Helper()
				assert.Equal(t, "Bearer test-token", req.headers.Get("Authorization"),
					"Authorization header must use Bearer scheme, not Token scheme")
			},
		},
		{
			name: "x_stack_id_header",
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
			name:         "put_with_correct_content_type_and_body",
			serverStatus: http.StatusOK,
			checkReq: func(t *testing.T, req capturedUpload) {
				t.Helper()
				assert.Equal(t, http.MethodPut, req.method)
				assert.Equal(t, "application/x-tar", req.contentType)
				assert.Equal(t, expectedBody, req.body)
			},
		},
		{
			name:         "no_authorization_header_on_s3_presigned_put",
			serverStatus: http.StatusOK,
			checkReq: func(t *testing.T, req capturedUpload) {
				t.Helper()
				assert.Empty(t, req.auth, "Authorization header must NOT be set on S3 presigned PUT")
			},
		},
		{
			name:            "error_propagation_on_non_2xx",
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

// newTestClientWithLogger creates a test client using the provided logger (for log capture).
func newTestClientWithLogger(t *testing.T, logger logrus.FieldLogger, handler http.Handler) *Client {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	client, err := NewClient(logger, "test-token", srv.URL, "1.0", 5*time.Second)
	require.NoError(t, err)
	client.SetStackID(1)

	return client
}

// makeTestRunResponse builds a minimal k6cloud.TestRunApiModel JSON-encodable value
// for the given status.
func makeTestRunResponse(status Status, statusHistory []StatusEvent) *k6cloud.TestRunApiModel {
	res := k6cloud.NewTestRunApiModelWithDefaults()
	res.SetStatus(status.String())
	res.SetResult(ResultPassed.String())
	res.SetDistribution([]k6cloud.DistributionZoneApiModel{})
	res.SetResultDetails(map[string]any{})
	res.SetOptions(map[string]any{})
	res.SetStatusHistory(ToStatusModel(statusHistory))
	return res
}

// sequentialHandler builds an httptest handler that returns responses[i] for the i-th call.
// After the last response is exhausted it repeats the last one.
func sequentialHandler(t *testing.T, responses []*k6cloud.TestRunApiModel) http.Handler {
	t.Helper()
	var mu sync.Mutex
	i := 0
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		res := responses[i]
		if i < len(responses)-1 {
			i++
		}
		mu.Unlock()
		writeJSON(t, w, http.StatusOK, res)
	})
}

func TestWaitForTestRunReady(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		responses       []*k6cloud.TestRunApiModel
		wantErrContains string // "" ⇒ expect nil error
		wantInfoLogs    int    // -1 = don't check; ≥0 = assert exact count of Info entries with "status" field
		wantWarnLogs    int    // -1 = don't check; ≥0 = assert exact count of Warn entries
	}{
		{
			name: "proceeds_on_first_initializing",
			responses: []*k6cloud.TestRunApiModel{
				makeTestRunResponse(StatusInitializing, nil),
			},
			wantInfoLogs: -1,
			wantWarnLogs: -1,
		},
		{
			name: "created_then_queued_then_initializing",
			responses: []*k6cloud.TestRunApiModel{
				makeTestRunResponse(StatusCreated, nil),
				makeTestRunResponse(StatusCreated, nil),
				makeTestRunResponse(StatusQueued, nil),
				makeTestRunResponse(StatusQueued, nil),
				makeTestRunResponse(StatusInitializing, nil),
			},
			wantInfoLogs: -1,
			wantWarnLogs: -1,
		},
		{
			name: "aborted_fails_with_message",
			responses: []*k6cloud.TestRunApiModel{
				makeTestRunResponse(StatusAborted, []StatusEvent{
					{Status: StatusAborted, Message: "backend rejected archive"},
				}),
			},
			wantErrContains: "backend rejected archive",
			wantInfoLogs:    -1,
			wantWarnLogs:    -1,
		},
		{
			name: "queued_then_initializing_logs_one_info_entry",
			responses: func() []*k6cloud.TestRunApiModel {
				r := make([]*k6cloud.TestRunApiModel, 0, 6)
				for range 5 {
					r = append(r, makeTestRunResponse(StatusQueued, nil))
				}
				r = append(r, makeTestRunResponse(StatusInitializing, nil))
				return r
			}(),
			wantInfoLogs: 1, // only queued logs; initializing returns immediately without logging
			wantWarnLogs: -1,
		},
		{
			name: "processing_metrics_keeps_polling_until_initializing",
			responses: []*k6cloud.TestRunApiModel{
				makeTestRunResponse(StatusProcessingMetrics, nil),
				makeTestRunResponse(StatusInitializing, nil),
			},
			wantInfoLogs: -1,
			wantWarnLogs: 0,
		},
		{
			name: "completed_errors_before_k6_starts",
			responses: []*k6cloud.TestRunApiModel{
				makeTestRunResponse(StatusCompleted, nil),
			},
			wantErrContains: "completed",
			wantInfoLogs:    -1,
			wantWarnLogs:    -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			logger, hook := logrustest.NewNullLogger()
			logger.SetLevel(logrus.InfoLevel)
			client := newTestClientWithLogger(t, logger, sequentialHandler(t, tt.responses))

			err := client.WaitForTestRunReady(t.Context(), 42, 1*time.Millisecond)

			if tt.wantErrContains != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrContains)
			} else {
				require.NoError(t, err)
			}

			if tt.wantInfoLogs >= 0 {
				var infoStatusEntries []*logrus.Entry
				for _, e := range hook.AllEntries() {
					if e.Level == logrus.InfoLevel {
						if _, ok := e.Data["status"]; ok {
							infoStatusEntries = append(infoStatusEntries, e)
						}
					}
				}
				assert.Len(t, infoStatusEntries, tt.wantInfoLogs,
					"expected exactly %d status-change log lines at Info", tt.wantInfoLogs)
			}

			if tt.wantWarnLogs >= 0 {
				var warnEntries []*logrus.Entry
				for _, e := range hook.AllEntries() {
					if e.Level == logrus.WarnLevel {
						warnEntries = append(warnEntries, e)
					}
				}
				require.Len(t, warnEntries, tt.wantWarnLogs)
				if tt.wantWarnLogs > 0 {
					assert.Contains(t, warnEntries[0].Message, "unexpected pre-run test run status")
				}
			}
		})
	}
}

// TestWaitForTestRunReady_ContextCancelMidPoll: context is cancelled after first FetchTest call.
// Verifies the function returns context.Canceled or context.DeadlineExceeded.
func TestWaitForTestRunReady_ContextCancelMidPoll(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())

	callCount := 0
	var mu sync.Mutex

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		callCount++
		mu.Unlock()
		// Cancel after first call returns
		cancel()
		writeJSON(t, w, http.StatusOK, makeTestRunResponse(StatusInitializing, nil))
	})

	client := newTestClient(t, handler)
	err := client.WaitForTestRunReady(ctx, 42, 1*time.Millisecond)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

// TestWaitForTestRunReady_LongQueueWait: mock returns queued×35, initializing×1, running×1.
// Verifies no timeout, no spurious error, and queued is logged exactly once.
func TestWaitForTestRunReady_LongQueueWait(t *testing.T) {
	t.Parallel()

	responses := make([]*k6cloud.TestRunApiModel, 0, 37)
	for range 35 {
		responses = append(responses, makeTestRunResponse(StatusQueued, nil))
	}
	responses = append(responses, makeTestRunResponse(StatusInitializing, nil))

	logger, hook := logrustest.NewNullLogger()
	logger.SetLevel(logrus.InfoLevel)
	client := newTestClientWithLogger(t, logger, sequentialHandler(t, responses))

	err := client.WaitForTestRunReady(t.Context(), 42, 1*time.Millisecond)
	require.NoError(t, err)

	// Count Info-level entries with "status" field — queued should appear exactly once.
	var queuedEntries int
	for _, e := range hook.AllEntries() {
		if e.Level == logrus.InfoLevel {
			if statusVal, ok := e.Data["status"]; ok {
				if fmt.Sprintf("%v", statusVal) == "Queued" {
					queuedEntries++
				}
			}
		}
	}
	assert.Equal(t, 1, queuedEntries, "queued status must be logged exactly once")
}

// notifyBody is the decoded notify request body for test assertions.
type notifyBody struct {
	EventType string       `json:"event_type"`
	Error     *notifyError `json:"error"`
}

func TestNotifyTestRunCompleted_ErrorMapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		testRunID      int32
		testErr        error
		wantErrorField *notifyError // nil ⇒ expect JSON null
	}{
		{
			name:           "nil_error_sends_null",
			testRunID:      42,
			testErr:        nil,
			wantErrorField: nil,
		},
		{
			name:           "aborted_by_user_maps_to_code_5",
			testRunID:      42,
			testErr:        testAbortError{reason: errext.AbortedByUser, msg: "aborted by user"},
			wantErrorField: &notifyError{Code: 5, Reason: "aborted by user"},
		},
		{
			name:           "aborted_by_threshold_maps_to_code_8",
			testRunID:      42,
			testErr:        testAbortError{reason: errext.AbortedByThreshold, msg: "threshold reached"},
			wantErrorField: &notifyError{Code: 8},
		},
		{
			name:           "aborted_by_script_error_maps_to_code_7",
			testRunID:      42,
			testErr:        testAbortError{reason: errext.AbortedByScriptError, msg: "script error"},
			wantErrorField: &notifyError{Code: 7},
		},
		{
			name:           "ctrl_c_during_queue_wait_still_maps_to_code_5",
			testRunID:      99,
			testErr:        testAbortError{reason: errext.AbortedByUser, msg: "aborted by user"},
			wantErrorField: &notifyError{Code: 5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var captured notifyBody
			var capturedPath string
			var capturedAuth string
			var capturedStackID string

			client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedPath = r.URL.Path
				capturedAuth = r.Header.Get("Authorization")
				capturedStackID = r.Header.Get("X-Stack-Id")
				require.NoError(t, json.NewDecoder(r.Body).Decode(&captured))
				w.WriteHeader(http.StatusNoContent)
			}))

			err := client.NotifyTestRunCompleted(t.Context(), tt.testRunID, tt.testErr)
			require.NoError(t, err)

			assert.Equal(t, fmt.Sprintf("/provisioning/v1/test_runs/%d/notify", tt.testRunID), capturedPath)
			assert.Equal(t, "Bearer test-token", capturedAuth)
			assert.Equal(t, "1", capturedStackID, "X-Stack-Id header must be set to the configured stack ID")
			assert.Equal(t, "script_execution_completed", captured.EventType)

			if tt.wantErrorField == nil {
				assert.Nil(t, captured.Error, "error field must be JSON null when testErr is nil")
			} else {
				require.NotNil(t, captured.Error)
				assert.Equal(t, tt.wantErrorField.Code, captured.Error.Code)
				if tt.wantErrorField.Reason != "" {
					assert.Equal(t, tt.wantErrorField.Reason, captured.Error.Reason)
				}
			}
		})
	}
}

// TestNotifyTestRunCompleted_5xxRetriesViaDo verifies that the notify call retries on 5xx
// responses: server returns 503 twice then 204 on the third attempt.
func TestNotifyTestRunCompleted_5xxRetriesViaDo(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Drain the body so the client can proceed.
		_, _ = io.Copy(io.Discard, r.Body)

		mu.Lock()
		callCount++
		n := callCount
		mu.Unlock()

		if n < MaxRetries {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := NewClient(testutils.NewLogger(t), "test-token", server.URL, "1.0", 5*time.Second)
	require.NoError(t, err)

	err = client.NotifyTestRunCompleted(t.Context(), 42, nil)
	require.NoError(t, err)

	mu.Lock()
	got := callCount
	mu.Unlock()
	assert.Equal(t, MaxRetries, got, "server must receive exactly MaxRetries calls")
}

// TestWaitForTestRunReady_ErrorHandling verifies error-handling in the poll loop:
//   - transient 5xx responses are retried by the SDK until success (AC-404)
//   - 4xx responses propagate as errors immediately without retry (AC-405)
func TestWaitForTestRunReady_ErrorHandling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		handler         func(t *testing.T) http.Handler
		wantErr         bool
		wantErrContains string
	}{
		{
			// The openapi SDK is configured with MaxRetries=3 and RetryInterval=500ms;
			// a single 503 triggers one internal retry — WaitForTestRunReady sees the
			// eventual success and returns nil.
			name: "transient_5xx_retried_by_sdk_succeeds_on_eventual_200",
			handler: func(t *testing.T) http.Handler {
				t.Helper()
				var calls atomic.Int32
				return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					if calls.Add(1) == 1 {
						w.WriteHeader(http.StatusServiceUnavailable)
						return
					}
					writeJSON(t, w, http.StatusOK, makeTestRunResponse(StatusInitializing, nil))
				})
			},
			wantErr: false,
		},
		{
			// 4xx responses are not retried by the SDK; the error propagates immediately.
			name: "4xx_from_poll_endpoint_propagates_as_error",
			handler: func(t *testing.T) http.Handler {
				t.Helper()
				return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					writeError(t, w, http.StatusForbidden, "forbidden", "not authorized")
				})
			},
			wantErr:         true,
			wantErrContains: "403",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := newTestClient(t, tt.handler(t))
			err := client.WaitForTestRunReady(t.Context(), 42, 1*time.Millisecond)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrContains != "" {
					assert.Contains(t, err.Error(), tt.wantErrContains)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestNotifyTestRunCompleted_ContextCancelledMidRetry verifies that a context cancellation
// mid-request propagates back as context.Canceled.
func TestNotifyTestRunCompleted_ContextCancelledMidRetry(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// unblockServer is closed by the test after it cancels the client context,
	// allowing the handler goroutine to return and the httptest.Server to close.
	started := make(chan struct{})
	unblockServer := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		select {
		case started <- struct{}{}:
		default:
		}
		<-unblockServer
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer func() {
		close(unblockServer)
		server.Close()
	}()

	client, err := NewClient(testutils.NewLogger(t), "test-token", server.URL, "1.0", 5*time.Second)
	require.NoError(t, err)

	errCh := make(chan error, 1)
	go func() {
		errCh <- client.NotifyTestRunCompleted(ctx, 42, nil)
	}()

	// Wait for the server to receive the request, then cancel the client context.
	<-started
	cancel()

	err = <-errCh
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled), "expected context.Canceled, got: %v", err)
}
