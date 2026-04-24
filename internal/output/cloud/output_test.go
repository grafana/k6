package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/cloudapi"
	"go.k6.io/k6/v2/errext"
	v6cloudapi "go.k6.io/k6/v2/internal/cloudapi/v6"
	"go.k6.io/k6/v2/internal/lib/testutils"
	"go.k6.io/k6/v2/internal/usage"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/lib/types"
	"go.k6.io/k6/v2/metrics"
	"go.k6.io/k6/v2/output"
	cloudv2 "go.k6.io/k6/v2/output/cloud/expv2"
	"gopkg.in/guregu/null.v3"
)

func TestNewOutputNameResolution(t *testing.T) {
	t.Parallel()
	mustParse := func(u string) *url.URL {
		result, err := url.Parse(u)
		require.NoError(t, err)
		return result
	}

	cases := []struct {
		url      *url.URL
		expected string
	}{
		{
			url: &url.URL{
				Opaque: "go.k6.io/k6/v2/samples/http_get.js",
			},
			expected: "http_get.js",
		},
		{
			url:      mustParse("http://go.k6.io/k6/v2/samples/http_get.js"),
			expected: "http_get.js",
		},
		{
			url:      mustParse("file://home/user/k6/samples/http_get.js"),
			expected: "http_get.js",
		},
		{
			url:      mustParse("file://C:/home/user/k6/samples/http_get.js"),
			expected: "http_get.js",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.url.String(), func(t *testing.T) {
			t.Parallel()
			out, err := newOutput(output.Params{
				Logger: testutils.NewLogger(t),
				ScriptOptions: lib.Options{
					Duration:   types.NullDurationFrom(1 * time.Second),
					SystemTags: &metrics.DefaultSystemTagSet,
				},
				ScriptPath: testCase.url,
			})
			require.NoError(t, err)
			require.Equal(t, testCase.expected, out.config.Name.String)
		})
	}
}

func TestCloudOutputRequireScriptName(t *testing.T) {
	t.Parallel()
	_, err := New(output.Params{
		Logger: testutils.NewLogger(t),
		ScriptOptions: lib.Options{
			Duration:   types.NullDurationFrom(1 * time.Second),
			SystemTags: &metrics.DefaultSystemTagSet,
		},
		ScriptPath: &url.URL{Path: ""},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "script name not set")
}

// TestOutputStart_UsesProvisioningFlow verifies that Output.Start() calls the v6
// provisioning API (CreateOrFindLoadTest + StartLocalExecution) instead of the old
// v1 CreateTestRun endpoint, and that the returned testRunID / MetricsPushURL /
// TestRunToken are wired correctly.
func TestOutputStart_UsesProvisioningFlow(t *testing.T) {
	t.Parallel()

	const (
		projectID int64 = 7
		testRunID int64 = 55
	)
	pushURL := "" // filled in after server start

	// We need the server URL first to build pushURL, so build a two-phase setup.
	var ts *httptest.Server

	sleResp := func() map[string]any {
		return map[string]any{
			"test_run_id":               testRunID,
			"archive_upload_url":        nil,
			"test_run_details_page_url": fmt.Sprintf("%s/runs/%d", ts.URL, testRunID),
			"runtime_config": map[string]any{
				"test_run_token": "scoped-tok",
				"metrics": map[string]any{
					"push_url": pushURL,
				},
				"traces": map[string]any{"push_url": ""},
				"files":  map[string]any{"push_url": ""},
				"logs":   map[string]any{"push_url": "", "tail_url": ""},
			},
		}
	}

	handler := func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case r.Method == http.MethodPost && path == fmt.Sprintf("/cloud/v6/projects/%d/load_tests", projectID):
			ltResp := map[string]any{
				"id":                   int32(7),
				"project_id":           projectID,
				"name":                 "test",
				"baseline_test_run_id": nil,
				"created":              "2024-01-01T00:00:00Z",
				"updated":              "2024-01-01T00:00:00Z",
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			require.NoError(t, json.NewEncoder(w).Encode(ltResp))
		case r.Method == http.MethodPost && path == "/provisioning/v1/load_tests/7/start_local_execution":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			require.NoError(t, json.NewEncoder(w).Encode(sleResp()))
		case r.Method == http.MethodPost && path == fmt.Sprintf("/provisioning/v1/test_runs/%d/notify", testRunID):
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && path == "/v1/tests":
			t.Error("Output.Start must NOT call v1 /v1/tests")
			http.Error(w, "v1 tests forbidden", http.StatusInternalServerError)
		default:
			// Accept metrics push and other optional endpoints silently.
			w.WriteHeader(http.StatusOK)
		}
	}

	ts = httptest.NewServer(http.HandlerFunc(handler))
	t.Cleanup(ts.Close)

	pushURL = ts.URL + "/v2/metrics/55"

	out, err := newOutput(output.Params{
		Logger: testutils.NewLogger(t),
		Environment: map[string]string{
			"K6_CLOUD_HOST":       ts.URL,
			"K6_CLOUD_HOST_V6":    ts.URL,
			"K6_CLOUD_STACK_ID":   "1",
			"K6_CLOUD_TOKEN":      "test-token",
			"K6_CLOUD_PROJECT_ID": fmt.Sprintf("%d", projectID),
			"K6_CLOUD_NAME":       "test",
		},
		ScriptOptions: lib.Options{
			Duration:   types.NullDurationFrom(1 * time.Second),
			SystemTags: &metrics.DefaultSystemTagSet,
		},
		ScriptPath: &url.URL{Path: "/script.js"},
		Usage:      usage.New(),
	})
	require.NoError(t, err)
	require.NoError(t, out.Start())

	assert.Equal(t, fmt.Sprintf("%d", testRunID), out.testRunID)
	assert.Equal(t, pushURL, out.config.MetricsPushURL.String)
	assert.Equal(t, "scoped-tok", out.config.TestRunToken.String)
	assert.NotNil(t, out.v6Client)

	require.NoError(t, out.StopWithTestError(nil))
}

// TestOutputStart_PushRefIDSkipsProvisioning verifies that when K6_CLOUD_PUSH_REF_ID is
// set, Start() takes the early-return path and does NOT call any provisioning endpoints.
func TestOutputStart_PushRefIDSkipsProvisioning(t *testing.T) {
	t.Parallel()

	handler := func(w http.ResponseWriter, r *http.Request) {
		// Any call to provisioning endpoints is a test failure.
		if r.URL.Path != "/v1/tests/12345" {
			t.Errorf("unexpected request to provisioning endpoint: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}
	ts := httptest.NewServer(http.HandlerFunc(handler))
	t.Cleanup(ts.Close)

	out, err := newOutput(output.Params{
		Logger: testutils.NewLogger(t),
		Environment: map[string]string{
			"K6_CLOUD_HOST":        ts.URL,
			"K6_CLOUD_HOST_V6":     ts.URL,
			"K6_CLOUD_TOKEN":       "test-token",
			"K6_CLOUD_PUSH_REF_ID": "12345",
		},
		ScriptOptions: lib.Options{
			Duration:   types.NullDurationFrom(1 * time.Second),
			SystemTags: &metrics.DefaultSystemTagSet,
		},
		ScriptPath: &url.URL{Path: "/script.js"},
		Usage:      usage.New(),
	})
	require.NoError(t, err)
	require.NoError(t, out.Start())

	assert.Equal(t, "12345", out.testRunID)
	assert.Nil(t, out.v6Client)

	// StopWithTestError should not call TestFinished because PushRefID is set.
	require.NoError(t, out.StopWithTestError(nil))
}

// TestOutputStart_NoV1TestsCall verifies that Output.Start() never calls the v1
// /v1/tests endpoint during normal provisioning flow.
func TestOutputStart_NoV1TestsCall(t *testing.T) {
	t.Parallel()

	const (
		projectID int64 = 7
		testRunID int64 = 55
	)

	var ts *httptest.Server

	handler := func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case r.Method == http.MethodPost && path == "/v1/tests":
			t.Error("Output.Start MUST NOT call /v1/tests — provisioning path must use v6 API")
			http.Error(w, "v1 forbidden", http.StatusInternalServerError)
		case r.Method == http.MethodPost && path == fmt.Sprintf("/cloud/v6/projects/%d/load_tests", projectID):
			ltResp := map[string]any{
				"id":                   int32(7),
				"project_id":           projectID,
				"name":                 "test",
				"baseline_test_run_id": nil,
				"created":              "2024-01-01T00:00:00Z",
				"updated":              "2024-01-01T00:00:00Z",
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			require.NoError(t, json.NewEncoder(w).Encode(ltResp))
		case r.Method == http.MethodPost && path == "/provisioning/v1/load_tests/7/start_local_execution":
			sleResp := map[string]any{
				"test_run_id":               testRunID,
				"archive_upload_url":        nil,
				"test_run_details_page_url": fmt.Sprintf("%s/runs/%d", ts.URL, testRunID),
				"runtime_config": map[string]any{
					"test_run_token": "scoped-tok",
					"metrics": map[string]any{
						"push_url": ts.URL + fmt.Sprintf("/v2/metrics/%d", testRunID),
					},
					"traces": map[string]any{"push_url": ""},
					"files":  map[string]any{"push_url": ""},
					"logs":   map[string]any{"push_url": "", "tail_url": ""},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			require.NoError(t, json.NewEncoder(w).Encode(sleResp))
		case r.Method == http.MethodPost && path == fmt.Sprintf("/provisioning/v1/test_runs/%d/notify", testRunID):
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}

	ts = httptest.NewServer(http.HandlerFunc(handler))
	t.Cleanup(ts.Close)

	out, err := newOutput(output.Params{
		Logger: testutils.NewLogger(t),
		Environment: map[string]string{
			"K6_CLOUD_HOST":       ts.URL,
			"K6_CLOUD_HOST_V6":    ts.URL,
			"K6_CLOUD_STACK_ID":   "1",
			"K6_CLOUD_TOKEN":      "test-token",
			"K6_CLOUD_PROJECT_ID": fmt.Sprintf("%d", projectID),
			"K6_CLOUD_NAME":       "test",
		},
		ScriptOptions: lib.Options{
			Duration:   types.NullDurationFrom(1 * time.Second),
			SystemTags: &metrics.DefaultSystemTagSet,
		},
		ScriptPath: &url.URL{Path: "/script.js"},
		Usage:      usage.New(),
	})
	require.NoError(t, err)
	require.NoError(t, out.Start())

	assert.Equal(t, fmt.Sprintf("%d", testRunID), out.testRunID)
	assert.NotNil(t, out.v6Client)

	require.NoError(t, out.StopWithTestError(nil))
}

func TestOutputStartVersionError(t *testing.T) {
	t.Parallel()
	o, err := newOutput(output.Params{
		Logger: testutils.NewLogger(t),
		ScriptOptions: lib.Options{
			Duration:   types.NullDurationFrom(1 * time.Second),
			SystemTags: &metrics.DefaultSystemTagSet,
		},
		Environment: map[string]string{
			"K6_CLOUD_API_VERSION": "99",
		},
		ScriptPath: &url.URL{Path: "/script.js"},
		Usage:      usage.New(),
	})
	require.NoError(t, err)

	o.testRunID = "123"
	err = o.startVersionedOutput()
	require.ErrorContains(t, err, "v99 is an unexpected version")
}

func TestOutputStartVersionedOutputV2(t *testing.T) {
	t.Parallel()

	o := Output{
		logger:    testutils.NewLogger(t),
		testRunID: "123",
		config: cloudapi.Config{
			APIVersion:            null.IntFrom(2),
			Host:                  null.StringFrom("fake-cloud-url"),
			Token:                 null.StringFrom("fake-token"),
			AggregationWaitPeriod: types.NullDurationFrom(1 * time.Second),
			// Here, we are enabling it but silencing the related async ops
			AggregationPeriod:  types.NullDurationFrom(1 * time.Hour),
			MetricPushInterval: types.NullDurationFrom(1 * time.Hour),
		},
		usage: usage.New(),
	}

	o.client = cloudapi.NewClient(
		nil, o.config.Token.String, o.config.Host.String, "v/tests", o.config.Timeout.TimeDuration())

	err := o.startVersionedOutput()
	require.NoError(t, err)

	_, ok := o.versionedOutput.(*cloudv2.Output)
	assert.True(t, ok)
}

func TestOutputStartVersionedOutputV1Error(t *testing.T) {
	t.Parallel()

	o := Output{
		testRunID: "123",
		config: cloudapi.Config{
			APIVersion: null.IntFrom(1),
		},
		usage: usage.New(),
	}

	err := o.startVersionedOutput()
	assert.ErrorContains(t, err, "not supported anymore")
}

func TestOutputStartWithTestRunID(t *testing.T) {
	t.Parallel()

	handler := func(_ http.ResponseWriter, _ *http.Request) {
		// no calls are expected to the cloud service when
		// the reference ID is passed
		t.Error("got unexpected call")
	}
	ts := httptest.NewServer(http.HandlerFunc(handler))
	defer ts.Close()

	out, err := newOutput(output.Params{
		Logger: testutils.NewLogger(t),
		Environment: map[string]string{
			"K6_CLOUD_HOST":        ts.URL,
			"K6_CLOUD_PUSH_REF_ID": "12345",
		},
		ScriptOptions: lib.Options{
			SystemTags: &metrics.DefaultSystemTagSet,
		},
		ScriptPath: &url.URL{Path: "/script.js"},
		Usage:      usage.New(),
	})
	require.NoError(t, err)
	require.NoError(t, out.Start())
	require.NoError(t, out.Stop())
}

func TestCloudOutputDescription(t *testing.T) {
	t.Parallel()

	t.Run("WithTestRunDetails", func(t *testing.T) {
		t.Parallel()
		o := Output{testRunID: "74"}
		o.config.TestRunDetails = null.StringFrom("my-custom-string")
		assert.Equal(t, "cloud (my-custom-string)", o.Description())
	})
	t.Run("WithWebAppURL", func(t *testing.T) {
		t.Parallel()
		o := Output{testRunID: "74"}
		o.config.WebAppURL = null.StringFrom("mywebappurl.com")
		assert.Equal(t, "cloud (mywebappurl.com/runs/74)", o.Description())
	})
}

func TestOutputStopWithTestError(t *testing.T) {
	t.Parallel()

	done := make(chan struct{})

	handler := func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/tests/1234":
			b, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			// aborted by system status
			expB := `{"result_status":0, "run_status":6, "thresholds":{}}`
			require.JSONEq(t, expB, string(b))

			w.WriteHeader(http.StatusOK)
			close(done)
		default:
			http.Error(w, "not expected path", http.StatusInternalServerError)
		}
	}
	ts := httptest.NewServer(http.HandlerFunc(handler))
	defer ts.Close()

	out, err := newOutput(output.Params{
		Logger: testutils.NewLogger(t),
		Environment: map[string]string{
			"K6_CLOUD_HOST": ts.URL,
		},
		ScriptOptions: lib.Options{
			SystemTags: &metrics.DefaultSystemTagSet,
		},
		ScriptPath: &url.URL{Path: "/script.js"},
	})
	require.NoError(t, err)

	calledStopFn := false
	out.testRunID = "1234"
	out.versionedOutput = versionedOutputMock{
		callback: func(fn string) {
			if fn == "StopWithTestError" {
				calledStopFn = true
			}
		},
	}

	fakeErr := errors.New("this is my error")
	require.NoError(t, out.StopWithTestError(fakeErr))
	assert.True(t, calledStopFn)

	select {
	case <-time.After(1 * time.Second):
		t.Error("timed out")
	case <-done:
	}
}

func TestOutputGetStatusRun(t *testing.T) {
	t.Parallel()

	t.Run("Success", func(t *testing.T) {
		t.Parallel()
		o := Output{}
		assert.Equal(t, cloudapi.RunStatusFinished, o.getRunStatus(nil))
	})
	t.Run("WithErrorNoAbortReason", func(t *testing.T) {
		t.Parallel()
		o := Output{logger: testutils.NewLogger(t)}
		assert.Equal(t, cloudapi.RunStatusAbortedSystem, o.getRunStatus(errors.New("my-error")))
	})
	t.Run("WithAbortReason", func(t *testing.T) {
		t.Parallel()
		o := Output{}
		errWithReason := errext.WithAbortReasonIfNone(
			errors.New("my-original-error"),
			errext.AbortedByOutput,
		)
		assert.Equal(t, cloudapi.RunStatusAbortedSystem, o.getRunStatus(errWithReason))
	})
}

func TestOutputProxyAddMetricSamples(t *testing.T) {
	t.Parallel()

	called := false
	o := &Output{
		versionedOutput: versionedOutputMock{
			callback: func(fn string) {
				if fn != "AddMetricSamples" {
					return
				}
				called = true
			},
		},
	}
	o.AddMetricSamples([]metrics.SampleContainer{})
	assert.True(t, called)
}

type versionedOutputMock struct {
	callback func(name string)
}

func (o versionedOutputMock) Start() error {
	o.callback("Start")
	return nil
}

func (o versionedOutputMock) StopWithTestError(_ error) error {
	o.callback("StopWithTestError")
	return nil
}

func (o versionedOutputMock) SetTestRunStopCallback(_ func(error)) {
	o.callback("SetTestRunStopCallback")
}

func (o versionedOutputMock) SetTestRunID(_ string) {
	o.callback("SetTestRunID")
}

func (o versionedOutputMock) AddMetricSamples(_ []metrics.SampleContainer) {
	o.callback("AddMetricSamples")
}

// TestOutputStart_ContextCancelPropagates verifies that cancelling the context
// passed via output.Params.Ctx causes Start() to return rather than block
// forever when the provisioning server is slow.
func TestOutputStart_ContextCancelPropagates(t *testing.T) {
	t.Parallel()

	blocked := make(chan struct{})
	handler := func(w http.ResponseWriter, _ *http.Request) {
		<-blocked
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	ts := httptest.NewServer(http.HandlerFunc(handler))
	t.Cleanup(func() { close(blocked); ts.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out, err := newOutput(output.Params{
		Ctx:    ctx,
		Logger: testutils.NewLogger(t),
		Environment: map[string]string{
			"K6_CLOUD_HOST":       ts.URL,
			"K6_CLOUD_HOST_V6":    ts.URL,
			"K6_CLOUD_STACK_ID":   "1",
			"K6_CLOUD_TOKEN":      "test-token",
			"K6_CLOUD_PROJECT_ID": "7",
			"K6_CLOUD_NAME":       "test",
		},
		ScriptOptions: lib.Options{
			Duration:   types.NullDurationFrom(1 * time.Second),
			SystemTags: &metrics.DefaultSystemTagSet,
		},
		ScriptPath: &url.URL{Path: "/script.js"},
		Usage:      usage.New(),
	})
	require.NoError(t, err)

	errCh := make(chan error, 1)
	go func() { errCh <- out.Start() }()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		require.Error(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("Start() did not return after context cancellation")
	}
}

// TestOutputStopWithTestError_ContextCancelPropagates verifies that when the
// context from output.Params is already cancelled, StopWithTestError returns an
// error from NotifyTestRunCompleted instead of blocking forever.
func TestOutputStopWithTestError_ContextCancelPropagates(t *testing.T) {
	t.Parallel()

	blocked := make(chan struct{})
	handler := func(w http.ResponseWriter, _ *http.Request) {
		<-blocked
		w.WriteHeader(http.StatusNoContent)
	}
	ts := httptest.NewServer(http.HandlerFunc(handler))
	t.Cleanup(func() { close(blocked); ts.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel so any request made with this context fails immediately

	v6c, err := v6cloudapi.NewClient(
		testutils.NewLogger(t), "test-token", ts.URL, "test", 10*time.Second)
	require.NoError(t, err)
	v6c.SetStackID(1)

	out := &Output{
		logger:    testutils.NewLogger(t),
		testRunID: "55",
		ctx:       ctx,
		v6Client:  v6c,
		config: cloudapi.Config{
			Host:  null.StringFrom(ts.URL),
			Token: null.StringFrom("test-token"),
		},
		versionedOutput: versionedOutputMock{callback: func(string) {}},
	}

	errCh := make(chan error, 1)
	go func() { errCh <- out.StopWithTestError(nil) }()

	select {
	case err := <-errCh:
		require.Error(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("StopWithTestError did not return after context cancellation")
	}
}
