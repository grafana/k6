package cloud

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/output"
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
			fmt.Fprintf(w, `{
"reference_id": "cloud-create-test",
"config": {
	"metricPushInterval": "10ms",
	"aggregationPeriod": "30ms"
}
}`)
		case "/v1/tests/cloud-create-test":
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
	})
	require.NoError(t, err)
	require.NoError(t, out.Start())

	assert.Equal(t, types.NullDurationFrom(10*time.Millisecond), out.config.MetricPushInterval)
	assert.Equal(t, types.NullDurationFrom(30*time.Millisecond), out.config.AggregationPeriod)

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
			"K6_CLOUD_API_VERSION": "3",
		},
		ScriptPath: &url.URL{Path: "/script.js"},
	})
	require.NoError(t, err)

	o.referenceID = "123"
	err = o.startVersionedOutput()
	require.ErrorContains(t, err, "v3 is an unexpected version")
}
