package http

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
)

func TestExpectedStatuses(t *testing.T) {
	t.Parallel()
	runtime, _ := getTestModuleInstance(t)
	rt := runtime.VU.RuntimeField

	cases := map[string]struct {
		code, err string
		expected  expectedStatuses
	}{
		"good example": {
			expected: expectedStatuses{exact: []int{200, 300}, minmax: [][2]int{{200, 300}}},
			code:     `(http.expectedStatuses(200, 300, {min: 200, max:300}))`,
		},

		"strange example": {
			expected: expectedStatuses{exact: []int{200, 300}, minmax: [][2]int{{200, 300}}},
			code:     `(http.expectedStatuses(200, 300, {min: 200, max:300, other: "attribute"}))`,
		},

		"string status code": {
			code: `(http.expectedStatuses(200, "300", {min: 200, max:300}))`,
			err:  "argument number 2 to expectedStatuses was neither an integer nor an object like {min:100, max:329}",
		},

		"string max status code": {
			code: `(http.expectedStatuses(200, 300, {min: 200, max:"300"}))`,
			err:  "both min and max need to be integers for argument number 3",
		},
		"float status code": {
			err:  "argument number 2 to expectedStatuses was neither an integer nor an object like {min:100, max:329}",
			code: `(http.expectedStatuses(200, 300.5, {min: 200, max:300}))`,
		},

		"float max status code": {
			err:  "both min and max need to be integers for argument number 3",
			code: `(http.expectedStatuses(200, 300, {min: 200, max:300.5}))`,
		},
		"no arguments": {
			code: `(http.expectedStatuses())`,
			err:  "no arguments",
		},
	}

	for name, testCase := range cases {
		name, testCase := name, testCase
		t.Run(name, func(t *testing.T) {
			val, err := rt.RunString(testCase.code)
			if testCase.err == "" {
				require.NoError(t, err)
				got := new(expectedStatuses)
				err = rt.ExportTo(val, &got)
				require.NoError(t, err)
				require.Equal(t, testCase.expected, *got)
				return // the t.Run
			}

			require.Error(t, err)
			exc := err.(*goja.Exception)
			require.Contains(t, exc.Error(), testCase.err)
		})
	}
}

type expectedSample struct {
	tags    map[string]string
	metrics []string
}

func TestResponseCallbackInAction(t *testing.T) {
	t.Parallel()
	ts := newTestCase(t)
	tb := ts.tb
	samples := ts.samples

	sr := tb.Replacer.Replace

	HTTPMetricsWithoutFailed := []string{
		metrics.HTTPReqsName,
		metrics.HTTPReqBlockedName,
		metrics.HTTPReqConnectingName,
		metrics.HTTPReqDurationName,
		metrics.HTTPReqReceivingName,
		metrics.HTTPReqWaitingName,
		metrics.HTTPReqSendingName,
		metrics.HTTPReqTLSHandshakingName,
	}

	allHTTPMetrics := append(HTTPMetricsWithoutFailed, metrics.HTTPReqFailedName)

	testCases := map[string]struct {
		code            string
		expectedSamples []expectedSample
	}{
		"basic": {
			code: `http.request("GET", "HTTPBIN_URL/redirect/1");`,
			expectedSamples: []expectedSample{
				{
					tags: map[string]string{
						"method":            "GET",
						"url":               sr("HTTPBIN_URL/redirect/1"),
						"name":              sr("HTTPBIN_URL/redirect/1"),
						"status":            "302",
						"group":             "",
						"expected_response": "true",
						"proto":             "HTTP/1.1",
					},
					metrics: allHTTPMetrics,
				},
				{
					tags: map[string]string{
						"method":            "GET",
						"url":               sr("HTTPBIN_URL/get"),
						"name":              sr("HTTPBIN_URL/get"),
						"status":            "200",
						"group":             "",
						"expected_response": "true",
						"proto":             "HTTP/1.1",
					},
					metrics: allHTTPMetrics,
				},
			},
		},
		"overwrite per request": {
			code: `
			http.setResponseCallback(http.expectedStatuses(200));
			http.request("GET", "HTTPBIN_URL/redirect/1");
			`,
			expectedSamples: []expectedSample{
				{
					tags: map[string]string{
						"method":            "GET",
						"url":               sr("HTTPBIN_URL/redirect/1"),
						"name":              sr("HTTPBIN_URL/redirect/1"),
						"status":            "302",
						"group":             "",
						"expected_response": "false", // this is on purpose
						"proto":             "HTTP/1.1",
					},
					metrics: allHTTPMetrics,
				},
				{
					tags: map[string]string{
						"method":            "GET",
						"url":               sr("HTTPBIN_URL/get"),
						"name":              sr("HTTPBIN_URL/get"),
						"status":            "200",
						"group":             "",
						"expected_response": "true",
						"proto":             "HTTP/1.1",
					},
					metrics: allHTTPMetrics,
				},
			},
		},

		"global overwrite": {
			code: `http.request("GET", "HTTPBIN_URL/redirect/1", null, {responseCallback: http.expectedStatuses(200)});`,
			expectedSamples: []expectedSample{
				{
					tags: map[string]string{
						"method":            "GET",
						"url":               sr("HTTPBIN_URL/redirect/1"),
						"name":              sr("HTTPBIN_URL/redirect/1"),
						"status":            "302",
						"group":             "",
						"expected_response": "false", // this is on purpose
						"proto":             "HTTP/1.1",
					},
					metrics: allHTTPMetrics,
				},
				{
					tags: map[string]string{
						"method":            "GET",
						"url":               sr("HTTPBIN_URL/get"),
						"name":              sr("HTTPBIN_URL/get"),
						"status":            "200",
						"group":             "",
						"expected_response": "true",
						"proto":             "HTTP/1.1",
					},
					metrics: allHTTPMetrics,
				},
			},
		},
		"per request overwrite with null": {
			code: `http.request("GET", "HTTPBIN_URL/redirect/1", null, {responseCallback: null});`,
			expectedSamples: []expectedSample{
				{
					tags: map[string]string{
						"method": "GET",
						"url":    sr("HTTPBIN_URL/redirect/1"),
						"name":   sr("HTTPBIN_URL/redirect/1"),
						"status": "302",
						"group":  "",
						"proto":  "HTTP/1.1",
					},
					metrics: HTTPMetricsWithoutFailed,
				},
				{
					tags: map[string]string{
						"method": "GET",
						"url":    sr("HTTPBIN_URL/get"),
						"name":   sr("HTTPBIN_URL/get"),
						"status": "200",
						"group":  "",
						"proto":  "HTTP/1.1",
					},
					metrics: HTTPMetricsWithoutFailed,
				},
			},
		},
		"global overwrite with null": {
			code: `
			http.setResponseCallback(null);
			http.request("GET", "HTTPBIN_URL/redirect/1");
			`,
			expectedSamples: []expectedSample{
				{
					tags: map[string]string{
						"method": "GET",
						"url":    sr("HTTPBIN_URL/redirect/1"),
						"name":   sr("HTTPBIN_URL/redirect/1"),
						"status": "302",
						"group":  "",
						"proto":  "HTTP/1.1",
					},
					metrics: HTTPMetricsWithoutFailed,
				},
				{
					tags: map[string]string{
						"method": "GET",
						"url":    sr("HTTPBIN_URL/get"),
						"name":   sr("HTTPBIN_URL/get"),
						"status": "200",
						"group":  "",
						"proto":  "HTTP/1.1",
					},
					metrics: HTTPMetricsWithoutFailed,
				},
			},
		},
	}
	for name, testCase := range testCases {
		testCase := testCase

		runCode := func(code string) {
			t.Helper()
			ts.instance.defaultClient.responseCallback = defaultExpectedStatuses.match

			_, err := ts.runtime.RunOnEventLoop(sr(code))
			assert.NoError(t, err)
			bufSamples := metrics.GetBufferedSamples(samples)

			reqsCount := 0
			for _, container := range bufSamples {
				for _, sample := range container.GetSamples() {
					if sample.Metric.Name == "http_reqs" {
						reqsCount++
					}
				}
			}

			require.Equal(t, len(testCase.expectedSamples), reqsCount)

			for i, expectedSample := range testCase.expectedSamples {
				assertRequestMetricsEmittedSingle(t, bufSamples[i], expectedSample.tags, expectedSample.metrics, nil)
			}
		}
		t.Run(name, func(t *testing.T) {
			runCode(testCase.code)
		})
		t.Run("async_"+name, func(t *testing.T) {
			runCode(strings.ReplaceAll(testCase.code, "http.request", "http.asyncRequest"))
		})
	}
}

func TestResponseCallbackBatch(t *testing.T) {
	t.Parallel()
	ts := newTestCase(t)
	tb := ts.tb
	samples := ts.samples
	rt := ts.runtime.VU.Runtime()

	sr := tb.Replacer.Replace

	HTTPMetricsWithoutFailed := []string{
		metrics.HTTPReqsName,
		metrics.HTTPReqBlockedName,
		metrics.HTTPReqConnectingName,
		metrics.HTTPReqDurationName,
		metrics.HTTPReqReceivingName,
		metrics.HTTPReqWaitingName,
		metrics.HTTPReqSendingName,
		metrics.HTTPReqTLSHandshakingName,
	}

	allHTTPMetrics := append(HTTPMetricsWithoutFailed, metrics.HTTPReqFailedName)
	// IMPORTANT: the tests here depend on the fact that the url they hit can be ordered in the same
	// order as the expectedSamples even if they are made concurrently
	testCases := map[string]struct {
		code            string
		expectedSamples []expectedSample
	}{
		"basic": {
			code: `
	http.batch([["GET", "HTTPBIN_URL/status/200", null, {responseCallback: null}],
			["GET", "HTTPBIN_URL/status/201"],
			["GET", "HTTPBIN_URL/status/202", null, {responseCallback: http.expectedStatuses(4)}],
			["GET", "HTTPBIN_URL/status/405", null, {responseCallback: http.expectedStatuses(405)}],
	]);`,
			expectedSamples: []expectedSample{
				{
					tags: map[string]string{
						"method": "GET",
						"url":    sr("HTTPBIN_URL/status/200"),
						"name":   sr("HTTPBIN_URL/status/200"),
						"status": "200",
						"group":  "",
						"proto":  "HTTP/1.1",
					},
					metrics: HTTPMetricsWithoutFailed,
				},
				{
					tags: map[string]string{
						"method":            "GET",
						"url":               sr("HTTPBIN_URL/status/201"),
						"name":              sr("HTTPBIN_URL/status/201"),
						"status":            "201",
						"group":             "",
						"expected_response": "true",
						"proto":             "HTTP/1.1",
					},
					metrics: allHTTPMetrics,
				},
				{
					tags: map[string]string{
						"method":            "GET",
						"url":               sr("HTTPBIN_URL/status/202"),
						"name":              sr("HTTPBIN_URL/status/202"),
						"status":            "202",
						"group":             "",
						"expected_response": "false",
						"proto":             "HTTP/1.1",
					},
					metrics: allHTTPMetrics,
				},
				{
					tags: map[string]string{
						"method":            "GET",
						"url":               sr("HTTPBIN_URL/status/405"),
						"name":              sr("HTTPBIN_URL/status/405"),
						"status":            "405",
						"error_code":        "1405",
						"group":             "",
						"expected_response": "true",
						"proto":             "HTTP/1.1",
					},
					metrics: allHTTPMetrics,
				},
			},
		},
	}
	for name, testCase := range testCases {
		testCase := testCase
		t.Run(name, func(t *testing.T) {
			ts.instance.defaultClient.responseCallback = defaultExpectedStatuses.match

			_, err := rt.RunString(sr(testCase.code))
			assert.NoError(t, err)
			bufSamples := metrics.GetBufferedSamples(samples)

			reqsCount := 0
			for _, container := range bufSamples {
				for _, sample := range container.GetSamples() {
					if sample.Metric.Name == "http_reqs" {
						reqsCount++
					}
				}
			}
			sort.Slice(bufSamples, func(i, j int) bool {
				iURL, _ := bufSamples[i].GetSamples()[0].Tags.Get("url")
				jURL, _ := bufSamples[j].GetSamples()[0].Tags.Get("url")
				return iURL < jURL
			})

			require.Equal(t, len(testCase.expectedSamples), reqsCount)

			for i, expectedSample := range testCase.expectedSamples {
				assertRequestMetricsEmittedSingle(t, bufSamples[i], expectedSample.tags, expectedSample.metrics, nil)
			}
		})
	}
}

func TestResponseCallbackInActionWithoutPassedTag(t *testing.T) {
	t.Parallel()
	ts := newTestCase(t)
	tb := ts.tb
	samples := ts.samples
	rt := ts.runtime.VU.Runtime()
	state := ts.runtime.VU.State()
	sr := tb.Replacer.Replace
	allHTTPMetrics := []string{
		metrics.HTTPReqsName,
		metrics.HTTPReqFailedName,
		metrics.HTTPReqBlockedName,
		metrics.HTTPReqConnectingName,
		metrics.HTTPReqDurationName,
		metrics.HTTPReqReceivingName,
		metrics.HTTPReqSendingName,
		metrics.HTTPReqWaitingName,
		metrics.HTTPReqTLSHandshakingName,
	}
	deleteSystemTag(state, metrics.TagExpectedResponse.String())

	_, err := rt.RunString(sr(`http.request("GET", "HTTPBIN_URL/redirect/1", null, {responseCallback: http.expectedStatuses(200)});`))
	assert.NoError(t, err)
	bufSamples := metrics.GetBufferedSamples(samples)

	reqsCount := 0
	for _, container := range bufSamples {
		for _, sample := range container.GetSamples() {
			if sample.Metric.Name == "http_reqs" {
				reqsCount++
			}
		}
	}

	require.Equal(t, 2, reqsCount)

	tags := map[string]string{
		"method": "GET",
		"url":    sr("HTTPBIN_URL/redirect/1"),
		"name":   sr("HTTPBIN_URL/redirect/1"),
		"status": "302",
		"group":  "",
		"proto":  "HTTP/1.1",
	}
	assertRequestMetricsEmittedSingle(t, bufSamples[0], tags, allHTTPMetrics, func(sample metrics.Sample) {
		if sample.Metric.Name == metrics.HTTPReqFailedName {
			require.EqualValues(t, sample.Value, 1)
		}
	})
	tags["url"] = sr("HTTPBIN_URL/get")
	tags["name"] = tags["url"]
	tags["status"] = "200"
	assertRequestMetricsEmittedSingle(t, bufSamples[1], tags, allHTTPMetrics, func(sample metrics.Sample) {
		if sample.Metric.Name == metrics.HTTPReqFailedName {
			require.EqualValues(t, sample.Value, 0)
		}
	})
}

func TestDigestWithResponseCallback(t *testing.T) {
	t.Parallel()
	ts := newTestCase(t)
	tb := ts.tb
	samples := ts.samples
	rt := ts.runtime.VU.Runtime()

	urlWithCreds := tb.Replacer.Replace(
		"http://testuser:testpwd@HTTPBIN_IP:HTTPBIN_PORT/digest-auth/auth/testuser/testpwd",
	)

	allHTTPMetrics := []string{
		metrics.HTTPReqsName,
		metrics.HTTPReqFailedName,
		metrics.HTTPReqBlockedName,
		metrics.HTTPReqConnectingName,
		metrics.HTTPReqDurationName,
		metrics.HTTPReqReceivingName,
		metrics.HTTPReqSendingName,
		metrics.HTTPReqWaitingName,
		metrics.HTTPReqTLSHandshakingName,
	}
	_, err := rt.RunString(fmt.Sprintf(`
		var res = http.get(%q,  { auth: "digest" });
		if (res.status !== 200) { throw new Error("wrong status: " + res.status); }
		if (res.error_code !== 0) { throw new Error("wrong error code: " + res.error_code); }
	`, urlWithCreds))
	require.NoError(t, err)
	bufSamples := metrics.GetBufferedSamples(samples)

	reqsCount := 0
	for _, container := range bufSamples {
		for _, sample := range container.GetSamples() {
			if sample.Metric.Name == "http_reqs" {
				reqsCount++
			}
		}
	}

	require.Equal(t, 2, reqsCount)

	urlRaw := tb.Replacer.Replace(
		"http://HTTPBIN_IP:HTTPBIN_PORT/digest-auth/auth/testuser/testpwd")

	tags := map[string]string{
		"method":            "GET",
		"url":               urlRaw,
		"name":              urlRaw,
		"status":            "401",
		"group":             "",
		"proto":             "HTTP/1.1",
		"expected_response": "true",
		"error_code":        "1401",
	}
	assertRequestMetricsEmittedSingle(t, bufSamples[0], tags, allHTTPMetrics, func(sample metrics.Sample) {
		if sample.Metric.Name == metrics.HTTPReqFailedName {
			require.EqualValues(t, sample.Value, 0)
		}
	})
	tags["status"] = "200"
	delete(tags, "error_code")
	assertRequestMetricsEmittedSingle(t, bufSamples[1], tags, allHTTPMetrics, func(sample metrics.Sample) {
		if sample.Metric.Name == metrics.HTTPReqFailedName {
			require.EqualValues(t, sample.Value, 0)
		}
	})
}

func deleteSystemTag(state *lib.State, tag string) {
	enabledTags := state.Options.SystemTags.Map()
	delete(enabledTags, tag)
	tagsList := make([]string, 0, len(enabledTags))
	for k := range enabledTags {
		tagsList = append(tagsList, k)
	}
	state.Options.SystemTags = metrics.ToSystemTagSet(tagsList)
}
