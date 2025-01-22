package cloud

import (
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
	"go.k6.io/k6/cloudapi"
	"go.k6.io/k6/errext"
	"go.k6.io/k6/internal/lib/testutils"
	"go.k6.io/k6/internal/usage"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/output"
	cloudv2 "go.k6.io/k6/output/cloud/expv2"
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
				Opaque: "go.k6.io/k6/samples/http_get.js",
			},
			expected: "http_get.js",
		},
		{
			url:      mustParse("http://go.k6.io/k6/samples/http_get.js"),
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
		testCase := testCase

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
			require.Equal(t, out.config.Name.String, testCase.expected)
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
