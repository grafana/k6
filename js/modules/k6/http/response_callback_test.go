/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2021 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package http

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/dop251/goja"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/metrics"
	"go.k6.io/k6/stats"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpectedStatuses(t *testing.T) {
	t.Parallel()
	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})
	ctx := context.Background()

	ctx = common.WithRuntime(ctx, rt)
	rt.Set("http", common.Bind(rt, new(GlobalHTTP).NewModuleInstancePerVU(), &ctx))
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
	metrics []*stats.Metric
}

func TestResponseCallbackInAction(t *testing.T) {
	t.Parallel()
	tb, _, samples, rt, ctx := newRuntime(t)
	defer tb.Cleanup()
	sr := tb.Replacer.Replace
	httpModule := new(GlobalHTTP).NewModuleInstancePerVU().(*HTTP)
	rt.Set("http", common.Bind(rt, httpModule, ctx))

	HTTPMetricsWithoutFailed := []*stats.Metric{
		metrics.HTTPReqs,
		metrics.HTTPReqBlocked,
		metrics.HTTPReqConnecting,
		metrics.HTTPReqDuration,
		metrics.HTTPReqReceiving,
		metrics.HTTPReqWaiting,
		metrics.HTTPReqSending,
		metrics.HTTPReqTLSHandshaking,
	}

	allHTTPMetrics := append(HTTPMetricsWithoutFailed, metrics.HTTPReqFailed)

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
			res = http.request("GET", "HTTPBIN_URL/redirect/1");
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
			res = http.request("GET", "HTTPBIN_URL/redirect/1");
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
		t.Run(name, func(t *testing.T) {
			httpModule.responseCallback = defaultExpectedStatuses.match

			_, err := rt.RunString(sr(testCase.code))
			assert.NoError(t, err)
			bufSamples := stats.GetBufferedSamples(samples)

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
		})
	}
}

func TestResponseCallbackBatch(t *testing.T) {
	t.Parallel()
	tb, _, samples, rt, ctx := newRuntime(t)
	defer tb.Cleanup()
	sr := tb.Replacer.Replace
	httpModule := new(GlobalHTTP).NewModuleInstancePerVU().(*HTTP)
	rt.Set("http", common.Bind(rt, httpModule, ctx))

	HTTPMetricsWithoutFailed := []*stats.Metric{
		metrics.HTTPReqs,
		metrics.HTTPReqBlocked,
		metrics.HTTPReqConnecting,
		metrics.HTTPReqDuration,
		metrics.HTTPReqReceiving,
		metrics.HTTPReqWaiting,
		metrics.HTTPReqSending,
		metrics.HTTPReqTLSHandshaking,
	}

	allHTTPMetrics := append(HTTPMetricsWithoutFailed, metrics.HTTPReqFailed)
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
			httpModule.responseCallback = defaultExpectedStatuses.match

			_, err := rt.RunString(sr(testCase.code))
			assert.NoError(t, err)
			bufSamples := stats.GetBufferedSamples(samples)

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
	tb, state, samples, rt, ctx := newRuntime(t)
	defer tb.Cleanup()
	sr := tb.Replacer.Replace
	allHTTPMetrics := []*stats.Metric{
		metrics.HTTPReqs,
		metrics.HTTPReqFailed,
		metrics.HTTPReqBlocked,
		metrics.HTTPReqConnecting,
		metrics.HTTPReqDuration,
		metrics.HTTPReqReceiving,
		metrics.HTTPReqSending,
		metrics.HTTPReqWaiting,
		metrics.HTTPReqTLSHandshaking,
	}
	deleteSystemTag(state, stats.TagExpectedResponse.String())
	httpModule := new(GlobalHTTP).NewModuleInstancePerVU().(*HTTP)
	rt.Set("http", common.Bind(rt, httpModule, ctx))

	_, err := rt.RunString(sr(`http.request("GET", "HTTPBIN_URL/redirect/1", null, {responseCallback: http.expectedStatuses(200)});`))
	assert.NoError(t, err)
	bufSamples := stats.GetBufferedSamples(samples)

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
	assertRequestMetricsEmittedSingle(t, bufSamples[0], tags, allHTTPMetrics, func(sample stats.Sample) {
		if sample.Metric.Name == metrics.HTTPReqFailed.Name {
			require.EqualValues(t, sample.Value, 1)
		}
	})
	tags["url"] = sr("HTTPBIN_URL/get")
	tags["name"] = tags["url"]
	tags["status"] = "200"
	assertRequestMetricsEmittedSingle(t, bufSamples[1], tags, allHTTPMetrics, func(sample stats.Sample) {
		if sample.Metric.Name == metrics.HTTPReqFailed.Name {
			require.EqualValues(t, sample.Value, 0)
		}
	})
}

func TestDigestWithResponseCallback(t *testing.T) {
	t.Parallel()
	tb, _, samples, rt, ctx := newRuntime(t)
	defer tb.Cleanup()

	httpModule := new(GlobalHTTP).NewModuleInstancePerVU().(*HTTP)
	rt.Set("http", common.Bind(rt, httpModule, ctx))

	urlWithCreds := tb.Replacer.Replace(
		"http://testuser:testpwd@HTTPBIN_IP:HTTPBIN_PORT/digest-auth/auth/testuser/testpwd",
	)

	allHTTPMetrics := []*stats.Metric{
		metrics.HTTPReqs,
		metrics.HTTPReqFailed,
		metrics.HTTPReqBlocked,
		metrics.HTTPReqConnecting,
		metrics.HTTPReqDuration,
		metrics.HTTPReqReceiving,
		metrics.HTTPReqSending,
		metrics.HTTPReqWaiting,
		metrics.HTTPReqTLSHandshaking,
	}
	_, err := rt.RunString(fmt.Sprintf(`
		var res = http.get(%q,  { auth: "digest" });
		if (res.status !== 200) { throw new Error("wrong status: " + res.status); }
		if (res.error_code !== 0) { throw new Error("wrong error code: " + res.error_code); }
	`, urlWithCreds))
	require.NoError(t, err)
	bufSamples := stats.GetBufferedSamples(samples)

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
	assertRequestMetricsEmittedSingle(t, bufSamples[0], tags, allHTTPMetrics, func(sample stats.Sample) {
		if sample.Metric.Name == metrics.HTTPReqFailed.Name {
			require.EqualValues(t, sample.Value, 0)
		}
	})
	tags["status"] = "200"
	delete(tags, "error_code")
	assertRequestMetricsEmittedSingle(t, bufSamples[1], tags, allHTTPMetrics, func(sample stats.Sample) {
		if sample.Metric.Name == metrics.HTTPReqFailed.Name {
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
	state.Options.SystemTags = stats.ToSystemTagSet(tagsList)
}
