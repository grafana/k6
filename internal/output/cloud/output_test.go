package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/cloudapi"
	"go.k6.io/k6/v2/errext"
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

func TestOutputCreateTestWithConfigOverwrite(t *testing.T) {
	t.Parallel()

	handler := func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/tests":
			_, err := fmt.Fprintf(w, `{
"reference_id": "12345",
"config": {
	"metricPushInterval": "10ms",
	"aggregationPeriod": "40s"
}
}`)
			require.NoError(t, err)
		case "/v1/tests/12345":
			w.WriteHeader(http.StatusOK)
		default:
			http.Error(w, "not expected path", http.StatusInternalServerError)
		}
	}
	ts := httptest.NewServer(http.HandlerFunc(handler))
	defer ts.Close()

	out, err := newOutput(output.Params{
		Logger: testutils.NewLogger(t),
		Environment: map[string]string{
			"K6_CLOUD_HOST":               ts.URL,
			"K6_CLOUD_AGGREGATION_PERIOD": "30s",
		},
		ScriptOptions: lib.Options{
			SystemTags: &metrics.DefaultSystemTagSet,
		},
		ScriptPath: &url.URL{Path: "/script.js"},
		Usage:      usage.New(),
	})
	require.NoError(t, err)
	require.NoError(t, out.Start())

	assert.Equal(t, types.NullDurationFrom(10*time.Millisecond), out.config.MetricPushInterval)
	assert.Equal(t, types.NullDurationFrom(40*time.Second), out.config.AggregationPeriod)

	// Assert that it overwrites only the provided values
	expTimeout := types.NewNullDuration(60*time.Second, false)
	assert.Equal(t, expTimeout, out.config.Timeout)

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
	synctest.Test(t, func(t *testing.T) {
		mock := &cloudClientMock{}
		out := &Output{
			logger:    testutils.NewLogger(t),
			testRunID: "1234",
			client:    mock,
		}

		calledStopFn := false
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

		require.True(t, mock.testFinishedCalled)
		assert.Equal(t, "1234", mock.testFinishedRefID)
		assert.Equal(t, cloudapi.ThresholdResult{}, mock.testFinishedThresholds)
		assert.False(t, mock.testFinishedTainted)
		assert.Equal(t, cloudapi.RunStatusAbortedSystem, mock.testFinishedRunStatus)
	})
}

// Locks in the symmetric-gating property: the stop-time decision must read
// out.provisioningMode (set by lazyInitProvisioning), never the Config fields
// directly. Otherwise a future divergence (Config fields populated without
// lazyInit running) would nil-deref out.provisioningNotifier.
func TestOutputStopWithTestError_ConfigFieldsAloneDoNotInferProvisioningMode(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		mock := &cloudClientMock{}
		out := &Output{
			logger:    testutils.NewLogger(t),
			testRunID: "1234",
			client:    mock,
			// Config fields populated; lazyInit did not run; deps nil.
			config: cloudapi.Config{
				MetricsPushURL: null.StringFrom("https://ingest.example/metrics/abc"),
				TestRunToken:   null.StringFrom("scoped-token"),
			},
		}
		out.versionedOutput = versionedOutputMock{callback: func(string) {}}

		require.NoError(t, out.StopWithTestError(nil))

		require.True(t, mock.testFinishedCalled,
			"legacy TestFinished must be called when provisioningMode is false")
	})
}

// cloudClientMock implements cloudClient for tests without starting a real HTTP server.
type cloudClientMock struct {
	createTestRunFn func(*cloudapi.TestRun) (*cloudapi.CreateTestRunResponse, error)

	testFinishedCalled     bool
	testFinishedRefID      string
	testFinishedThresholds cloudapi.ThresholdResult
	testFinishedTainted    bool
	testFinishedRunStatus  cloudapi.RunStatus
}

func (m *cloudClientMock) CreateTestRun(tr *cloudapi.TestRun) (*cloudapi.CreateTestRunResponse, error) {
	if m.createTestRunFn != nil {
		return m.createTestRunFn(tr)
	}
	return &cloudapi.CreateTestRunResponse{}, nil
}

func (m *cloudClientMock) TestFinished(referenceID string, thresholds cloudapi.ThresholdResult, tainted bool, runStatus cloudapi.RunStatus) error {
	m.testFinishedCalled = true
	m.testFinishedRefID = referenceID
	m.testFinishedThresholds = thresholds
	m.testFinishedTainted = tainted
	m.testFinishedRunStatus = runStatus
	return nil
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

// metricsPusherMock implements metricsPusher for tests.
type metricsPusherMock struct {
	doCalls atomic.Int32
}

func (m *metricsPusherMock) Do(_ *http.Request, _ any) error {
	m.doCalls.Add(1)
	return nil
}

// provisioningNotifierMock implements provisioningNotifier for tests.
type provisioningNotifierMock struct {
	called    atomic.Bool
	callCount atomic.Int32

	// Captured arguments from the most recent call.
	testRunID int64
	token     string
	testErr   error
}

func (m *provisioningNotifierMock) NotifyTestRunCompleted(_ context.Context, testRunID int64, token string, testErr error) error {
	m.called.Store(true)
	m.callCount.Add(1)
	m.testRunID = testRunID
	m.token = token
	m.testErr = testErr
	return nil
}

func TestOutputStart_ProvisioningMode(t *testing.T) {
	t.Parallel()

	// Build the JSON config containing MetricsPushURL and TestRunToken
	// (the provisioning-mode fields on cloudapi.Config).
	cfgJSON, err := json.Marshal(cloudapi.Config{
		Host:                  null.StringFrom("https://fake-cloud"),
		Token:                 null.StringFrom("fake-long-lived-token"),
		MetricsPushURL:        null.StringFrom("https://metrics.example.com/v2/metrics/123"),
		TestRunToken:          null.StringFrom("scoped-test-run-token"),
		APIVersion:            null.IntFrom(2),
		AggregationPeriod:     types.NullDurationFrom(1 * time.Hour),
		MetricPushInterval:    types.NullDurationFrom(1 * time.Hour),
		AggregationWaitPeriod: types.NullDurationFrom(1 * time.Second),
		MaxTimeSeriesInBatch:  null.IntFrom(1000),
		MetricPushConcurrency: null.IntFrom(1),
	})
	require.NoError(t, err)

	out, err := newOutput(output.Params{
		Logger:     testutils.NewLogger(t),
		JSONConfig: cfgJSON,
		ScriptOptions: lib.Options{
			Duration:   types.NullDurationFrom(1 * time.Second),
			SystemTags: &metrics.DefaultSystemTagSet,
		},
		ScriptPath: &url.URL{Path: "/script.js"},
		RuntimeOptions: lib.RuntimeOptions{
			Env: map[string]string{
				"K6_CLOUDRUN_TEST_RUN_ID": "12345",
			},
		},
		Usage: usage.New(),
	})
	require.NoError(t, err)

	// Inject a cloudClient mock whose CreateTestRun would fail the test
	// if called — regression assertion that the provisioning path never
	// falls through to the CreateTestRun legacy branch.
	out.client = &cloudClientMock{
		createTestRunFn: func(_ *cloudapi.TestRun) (*cloudapi.CreateTestRunResponse, error) {
			t.Fatal("CreateTestRun must not be called in provisioning mode")
			return nil, errors.New("unreachable")
		},
	}

	// Inject mocks for the provisioning-mode dependencies.
	pusherMock := &metricsPusherMock{}
	notifierMock := &provisioningNotifierMock{}
	out.metricsPusher = pusherMock
	out.provisioningNotifier = notifierMock

	// Start should succeed.
	require.NoError(t, out.Start())

	// The versionedOutput must be an *expv2.Output (api version 2).
	expv2Out, ok := out.versionedOutput.(*cloudv2.Output)
	require.True(t, ok, "expected versionedOutput to be *cloudv2.Output")
	assert.NotNil(t, expv2Out)

	// Feed a sample so the stop-time flush exercises the metrics-push path,
	// and confirm it goes through the injected metricsPusher rather than a
	// real HTTP client.
	r := metrics.NewRegistry()
	m := r.MustNewMetric("test_metric", metrics.Counter)
	out.AddMetricSamples([]metrics.SampleContainer{
		metrics.Sample{
			TimeSeries: metrics.TimeSeries{Metric: m, Tags: r.RootTagSet()},
			Time:       time.Now(),
			Value:      1.0,
		},
	})

	// Stop force-flushes the queued samples.
	require.NoError(t, out.StopWithTestError(nil))

	assert.Positive(t, pusherMock.doCalls.Load(),
		"queued metrics should be pushed through the injected metricsPusher")
}

// TestOutputStart_PushRefIDStillTakesPrecedence locks in cloud-Output
// behaviour even if cmd ever populated both PushRefID and MetricsPushURL.
// In practice, cmd/outputs_cloud.go:96-100 short-circuits on PushRefID
// before MetricsPushURL is ever set — so this combination shouldn't
// occur. This test is defensive against future cmd-layer regressions.
func TestOutputStart_PushRefIDStillTakesPrecedence(t *testing.T) {
	t.Parallel()

	cfgJSON, err := json.Marshal(cloudapi.Config{
		Host:                  null.StringFrom("https://fake-cloud"),
		Token:                 null.StringFrom("fake-token"),
		PushRefID:             null.StringFrom("99999"),
		MetricsPushURL:        null.StringFrom("https://metrics.example.com/v2/metrics/123"),
		TestRunToken:          null.StringFrom("scoped-test-run-token"),
		APIVersion:            null.IntFrom(2),
		AggregationPeriod:     types.NullDurationFrom(1 * time.Hour),
		MetricPushInterval:    types.NullDurationFrom(1 * time.Hour),
		AggregationWaitPeriod: types.NullDurationFrom(1 * time.Second),
		MaxTimeSeriesInBatch:  null.IntFrom(1000),
		MetricPushConcurrency: null.IntFrom(1),
	})
	require.NoError(t, err)

	out, err := newOutput(output.Params{
		Logger:     testutils.NewLogger(t),
		JSONConfig: cfgJSON,
		ScriptOptions: lib.Options{
			Duration:   types.NullDurationFrom(1 * time.Second),
			SystemTags: &metrics.DefaultSystemTagSet,
		},
		ScriptPath: &url.URL{Path: "/script.js"},
		Usage:      usage.New(),
	})
	require.NoError(t, err)

	require.NoError(t, out.Start())

	// PushRefID should win: testRunID should be set to the PushRefID value,
	// not the testRunID from env.
	assert.Equal(t, "99999", out.testRunID)

	// The metricsPusher field should remain nil — provisioning mode was NOT
	// entered because PushRefID took precedence.
	assert.Nil(t, out.metricsPusher, "metricsPusher should be nil when PushRefID takes precedence")
	assert.Nil(t, out.provisioningNotifier, "provisioningNotifier should be nil when PushRefID takes precedence")

	require.NoError(t, out.StopWithTestError(nil))
}

func TestOutputStopWithTestError_ProvisioningMode_NoError(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		notifierMock := &provisioningNotifierMock{}
		clientMock := &cloudClientMock{}

		out := &Output{
			logger:    testutils.NewLogger(t),
			testRunID: "12345",
			config: cloudapi.Config{
				MetricsPushURL: null.StringFrom("https://metrics.example.com/v2/metrics/123"),
				TestRunToken:   null.StringFrom("scoped-test-run-token"),
			},
			client:               clientMock,
			provisioningNotifier: notifierMock,
			provisioningMode:     true,
		}

		out.versionedOutput = versionedOutputMock{
			callback: func(_ string) {},
		}

		require.NoError(t, out.StopWithTestError(nil))

		// Notify was called exactly once with the right arguments.
		assert.True(t, notifierMock.called.Load())
		assert.Equal(t, int32(1), notifierMock.callCount.Load())
		assert.Equal(t, int64(12345), notifierMock.testRunID)
		assert.Equal(t, "scoped-test-run-token", notifierMock.token)
		assert.Nil(t, notifierMock.testErr)

		// TestFinished was NOT called (regression assertion).
		assert.False(t, clientMock.testFinishedCalled)
	})
}

func TestOutputStopWithTestError_ProvisioningMode_WithTestError(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		notifierMock := &provisioningNotifierMock{}
		clientMock := &cloudClientMock{}

		scriptErr := errext.WithAbortReasonIfNone(
			errors.New("script error"),
			errext.AbortedByScriptError,
		)

		out := &Output{
			logger:    testutils.NewLogger(t),
			testRunID: "12345",
			config: cloudapi.Config{
				MetricsPushURL: null.StringFrom("https://metrics.example.com/v2/metrics/123"),
				TestRunToken:   null.StringFrom("scoped-test-run-token"),
			},
			client:               clientMock,
			provisioningNotifier: notifierMock,
			provisioningMode:     true,
		}

		out.versionedOutput = versionedOutputMock{
			callback: func(_ string) {},
		}

		require.NoError(t, out.StopWithTestError(scriptErr))

		// Notify was called with the error verbatim (mapping to code 8035
		// happens inside the real notifier, which is mocked here).
		assert.True(t, notifierMock.called.Load())
		assert.Equal(t, int64(12345), notifierMock.testRunID)
		assert.Equal(t, "scoped-test-run-token", notifierMock.token)
		assert.Equal(t, scriptErr, notifierMock.testErr)

		// TestFinished was NOT called.
		assert.False(t, clientMock.testFinishedCalled)
	})
}

func TestOutputStopWithTestError_PushRefID_NoNotifyNoTestFinished(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		notifierMock := &provisioningNotifierMock{}
		clientMock := &cloudClientMock{}

		out := &Output{
			logger:    testutils.NewLogger(t),
			testRunID: "12345",
			config: cloudapi.Config{
				PushRefID:      null.StringFrom("99999"),
				MetricsPushURL: null.StringFrom("https://metrics.example.com/v2/metrics/123"),
				TestRunToken:   null.StringFrom("scoped-test-run-token"),
			},
			client:               clientMock,
			provisioningNotifier: notifierMock,
		}

		out.versionedOutput = versionedOutputMock{
			callback: func(_ string) {},
		}

		require.NoError(t, out.StopWithTestError(nil))

		// PushRefID short-circuits: NEITHER notify NOR TestFinished called.
		assert.False(t, notifierMock.called.Load())
		assert.False(t, clientMock.testFinishedCalled)
	})
}
