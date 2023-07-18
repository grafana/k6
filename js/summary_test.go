package js

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/metrics"
)

const (
	checksOut = "     █ child\n\n" +
		"       ✓ check1\n" +
		"       ✗ check3\n        ↳  66% — ✓ 10 / ✗ 5\n" +
		"       ✗ check2\n        ↳  33% — ✓ 5 / ✗ 10\n\n" +
		"   ✓ checks......: 75.00% ✓ 45  ✗ 15 \n"
	countOut = "   ✗ http_reqs...: 3      3/s\n"
	gaugeOut = "     vus.........: 1      min=1 max=1\n"
	trendOut = "   ✗ my_trend....: avg=15ms min=10ms med=15ms max=20ms p(90)=19ms " +
		"p(95)=19.5ms p(99.9)=19.99ms\n"
)

func TestTextSummary(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		stats    []string
		expected string
	}{
		{
			[]string{"avg", "min", "med", "max", "p(90)", "p(95)", "p(99.9)"},
			checksOut + countOut + trendOut + gaugeOut,
		},
		{[]string{"count"}, checksOut + countOut + "   ✗ my_trend....: count=3\n" + gaugeOut},
		{[]string{"avg", "count"}, checksOut + countOut + "   ✗ my_trend....: avg=15ms count=3\n" + gaugeOut},
	}

	for i, tc := range testCases {
		i, tc := i, tc
		t.Run(fmt.Sprintf("%d_%v", i, tc.stats), func(t *testing.T) {
			t.Parallel()
			summary := createTestSummary(t)
			trendStats, err := json.Marshal(tc.stats)
			require.NoError(t, err)
			runner, err := getSimpleRunner(
				t, "/script.js",
				fmt.Sprintf(`
					exports.options = {summaryTrendStats: %s};
					exports.default = function() {/* we don't run this, metrics are mocked */};
				`, string(trendStats)),
				lib.RuntimeOptions{CompatibilityMode: null.NewString("base", true)},
			)
			require.NoError(t, err)

			result, err := runner.HandleSummary(context.Background(), summary)
			require.NoError(t, err)

			require.Len(t, result, 1)
			stdout := result["stdout"]
			require.NotNil(t, stdout)
			summaryOut, err := io.ReadAll(stdout)
			require.NoError(t, err)
			assert.Equal(t, "\n"+tc.expected+"\n", string(summaryOut))
		})
	}
}

func TestTextSummaryWithSubMetrics(t *testing.T) {
	t.Parallel()

	registry := metrics.NewRegistry()
	parentMetric, err := registry.NewMetric("my_parent", metrics.Counter)
	require.NoError(t, err)
	parentMetric.Sink.Add(metrics.Sample{Value: 11})

	parentMetricPost, err := registry.NewMetric("my_parent_post", metrics.Counter)
	require.NoError(t, err)
	parentMetricPost.Sink.Add(metrics.Sample{Value: 22})

	subMetric, err := parentMetric.AddSubmetric("sub:1")
	require.NoError(t, err)
	subMetric.Metric.Sink.Add(metrics.Sample{Value: 1})

	subMetricPost, err := parentMetricPost.AddSubmetric("sub:2")
	require.NoError(t, err)
	subMetricPost.Metric.Sink.Add(metrics.Sample{Value: 2})

	metrics := map[string]*metrics.Metric{
		parentMetric.Name:     parentMetric,
		parentMetricPost.Name: parentMetricPost,
		subMetric.Name:        subMetric.Metric,
		subMetricPost.Name:    subMetricPost.Metric,
	}

	summary := &lib.Summary{
		Metrics:         metrics,
		RootGroup:       &lib.Group{},
		TestRunDuration: time.Second,
	}

	runner, err := getSimpleRunner(
		t,
		"/script.js",
		"exports.default = function() {/* we don't run this, metrics are mocked */};",
		lib.RuntimeOptions{CompatibilityMode: null.NewString("base", true)},
	)
	require.NoError(t, err)

	result, err := runner.HandleSummary(context.Background(), summary)
	require.NoError(t, err)

	require.Len(t, result, 1)
	stdout := result["stdout"]
	require.NotNil(t, stdout)

	summaryOut, err := io.ReadAll(stdout)
	require.NoError(t, err)

	expected := "     my_parent........: 11 11/s\n" +
		"       { sub:1 }......: 1  1/s\n" +
		"     my_parent_post...: 22 22/s\n" +
		"       { sub:2 }......: 2  2/s\n"
	assert.Equal(t, "\n"+expected+"\n", string(summaryOut))
}

func createTestMetrics(t *testing.T) (map[string]*metrics.Metric, *lib.Group) {
	registry := metrics.NewRegistry()
	testMetrics := make(map[string]*metrics.Metric)

	gaugeMetric, err := registry.NewMetric("vus", metrics.Gauge)
	require.NoError(t, err)
	gaugeMetric.Sink.Add(metrics.Sample{Value: 1})

	countMetric, err := registry.NewMetric("http_reqs", metrics.Counter)
	require.NoError(t, err)
	countMetric.Tainted = null.BoolFrom(true)
	countMetric.Thresholds = metrics.Thresholds{Thresholds: []*metrics.Threshold{{Source: "rate<100", LastFailed: true}}}

	checksMetric, err := registry.NewMetric("checks", metrics.Rate)
	require.NoError(t, err)
	checksMetric.Tainted = null.BoolFrom(false)
	checksMetric.Thresholds = metrics.Thresholds{Thresholds: []*metrics.Threshold{{Source: "rate>70", LastFailed: false}}}
	sink := metrics.NewTrendSink()

	samples := []float64{10.0, 15.0, 20.0}
	for _, s := range samples {
		sink.Add(metrics.Sample{Value: s})
		countMetric.Sink.Add(metrics.Sample{Value: 1})
	}

	testMetrics["vus"] = gaugeMetric
	testMetrics["http_reqs"] = countMetric
	testMetrics["checks"] = checksMetric
	testMetrics["my_trend"] = &metrics.Metric{
		Name:     "my_trend",
		Type:     metrics.Trend,
		Contains: metrics.Time,
		Sink:     sink,
		Tainted:  null.BoolFrom(true),
		Thresholds: metrics.Thresholds{
			Thresholds: []*metrics.Threshold{
				{
					Source:     "my_trend<1000",
					LastFailed: true,
				},
			},
		},
	}

	rootG, err := lib.NewGroup("", nil)
	require.NoError(t, err)
	childG, err := rootG.Group("child")
	require.NoError(t, err)
	check1, err := childG.Check("check1")
	require.NoError(t, err)
	check1.Passes = 30

	check3, err := childG.Check("check3") // intentionally before check2
	require.NoError(t, err)
	check3.Passes = 10
	check3.Fails = 5

	check2, err := childG.Check("check2")
	require.NoError(t, err)
	check2.Passes = 5
	check2.Fails = 10

	for i := 0; i < int(check1.Passes+check2.Passes+check3.Passes); i++ {
		checksMetric.Sink.Add(metrics.Sample{Value: 1})
	}
	for i := 0; i < int(check1.Fails+check2.Fails+check3.Fails); i++ {
		checksMetric.Sink.Add(metrics.Sample{Value: 0})
	}

	return testMetrics, rootG
}

func createTestSummary(t *testing.T) *lib.Summary {
	metrics, rootG := createTestMetrics(t)
	return &lib.Summary{
		Metrics:         metrics,
		RootGroup:       rootG,
		TestRunDuration: time.Second,
	}
}

const expectedOldJSONExportResult = `{
    "root_group": {
        "name": "",
        "path": "",
        "id": "d41d8cd98f00b204e9800998ecf8427e",
        "groups": {
            "child": {
                "name": "child",
                "path": "::child",
                "id": "f41cbb53a398ec1c9fb3d33e20c9b040",
                "groups": {},
                "checks": {
                    "check1": {
                        "name": "check1",
                        "path": "::child::check1",
                        "id": "6289a7a06253a1c3f6137dfb25695563",
                        "passes":30,
                        "fails": 0
                    },
                    "check2": {
                        "name": "check2",
                        "path": "::child::check2",
                        "id": "06f5922794bef0d4584ba76a49893e1f",
                        "passes": 5,
                        "fails": 10
                    },
                    "check3": {
                        "name": "check3",
                        "path": "::child::check3",
                        "id": "c7553eca92d3e034b5808332296d304a",
                        "passes": 10,
                        "fails": 5
                    }
                }
            }
        },
        "checks": {}
    },
    "metrics": {
        "checks": {
            "value": 0.75,
            "passes": 45,
            "fails": 15,
            "thresholds": {
                "rate>70": false
            }
        },
        "http_reqs": {
            "count": 3,
            "rate": 3,
            "thresholds": {
                "rate<100": true
            }
        },
        "my_trend": {
            "avg": 15,
            "max": 20,
            "med": 15,
            "min": 10,
            "p(90)": 19,
            "p(95)": 19.5,
            "p(99)": 19.9,
            "count": 3,
            "thresholds": {
                "my_trend<1000": true
            }
        },
        "vus": {
            "value": 1,
            "min": 1,
            "max": 1
        }
    }
}
`

func TestOldJSONExport(t *testing.T) {
	t.Parallel()
	runner, err := getSimpleRunner(
		t, "/script.js",
		`
		exports.options = {summaryTrendStats: ["avg", "min", "med", "max", "p(90)", "p(95)", "p(99)", "count"]};
		exports.default = function() {/* we don't run this, metrics are mocked */};
		`,
		lib.RuntimeOptions{
			CompatibilityMode: null.NewString("base", true),
			SummaryExport:     null.StringFrom("result.json"),
		},
	)

	require.NoError(t, err)

	summary := createTestSummary(t)
	result, err := runner.HandleSummary(context.Background(), summary)
	require.NoError(t, err)

	require.Len(t, result, 2)
	require.NotNil(t, result["stdout"])
	textSummary, err := io.ReadAll(result["stdout"])
	require.NoError(t, err)
	assert.Contains(t, string(textSummary), checksOut+countOut)
	require.NotNil(t, result["result.json"])
	jsonExport, err := io.ReadAll(result["result.json"])
	require.NoError(t, err)
	assert.JSONEq(t, expectedOldJSONExportResult, string(jsonExport))
}

const expectedHandleSummaryRawData = `
{
    "root_group": {
        "groups": [
            {
                "name": "child",
                "path": "::child",
                "id": "f41cbb53a398ec1c9fb3d33e20c9b040",
                "groups": [],
                "checks": [
                        {
                            "id": "6289a7a06253a1c3f6137dfb25695563",
                            "passes": 30,
                            "fails": 0,
                            "name": "check1",
                            "path": "::child::check1"
                        },
                        {
                            "fails": 5,
                            "name": "check3",
                            "path": "::child::check3",
                            "id": "c7553eca92d3e034b5808332296d304a",
                            "passes": 10
                        },
                        {
                            "name": "check2",
                            "path": "::child::check2",
                            "id": "06f5922794bef0d4584ba76a49893e1f",
                            "passes": 5,
                            "fails": 10
                        }
                    ]
            }
        ],
        "checks": [],
        "name": "",
        "path": "",
        "id": "d41d8cd98f00b204e9800998ecf8427e"
    },
    "options": {
        "summaryTrendStats": [
            "avg",
            "min",
            "med",
            "max",
            "p(90)",
            "p(95)",
            "p(99)",
            "count"
        ],
        "summaryTimeUnit": "",
        "noColor": false
    },
    "state": {
        "isStdErrTTY": false,
        "isStdOutTTY": false,
        "testRunDurationMs": 1000
    },
    "metrics": {
        "checks": {
            "contains": "default",
            "values": {
                "passes": 45,
                "fails": 15,
                "rate": 0.75
            },
            "type": "rate",
            "thresholds": {
                "rate>70": {
                    "ok": true
                }
            }
        },
        "my_trend": {
            "thresholds": {
                "my_trend<1000": {
                    "ok": false
                }
            },
            "type": "trend",
            "contains": "time",
            "values": {
                "max": 20,
                "p(90)": 19,
                "p(95)": 19.5,
                "p(99)": 19.9,
                "count": 3,
                "avg": 15,
                "min": 10,
                "med": 15
            }
        },
        "vus": {
            "contains": "default",
            "values": {
                "value": 1,
                "min": 1,
                "max": 1
            },
            "type": "gauge"
        },
        "http_reqs": {
            "type": "counter",
            "contains": "default",
            "values": {
                "count": 3,
                "rate": 3
            },
            "thresholds": {
                "rate<100": {
                    "ok": false
                }
            }
        }
    }
}`

const expectedHandleSummaryDataWithSetup = `
{
    "root_group": {
        "groups": [
            {
                "name": "child",
                "path": "::child",
                "id": "f41cbb53a398ec1c9fb3d33e20c9b040",
                "groups": [],
                "checks": [
                        {
                            "id": "6289a7a06253a1c3f6137dfb25695563",
                            "passes": 30,
                            "fails": 0,
                            "name": "check1",
                            "path": "::child::check1"
                        },
                        {
                            "fails": 5,
                            "name": "check3",
                            "path": "::child::check3",
                            "id": "c7553eca92d3e034b5808332296d304a",
                            "passes": 10
                        },
                        {
                            "name": "check2",
                            "path": "::child::check2",
                            "id": "06f5922794bef0d4584ba76a49893e1f",
                            "passes": 5,
                            "fails": 10
                        }
                    ]
            }
        ],
        "checks": [],
        "name": "",
        "path": "",
        "id": "d41d8cd98f00b204e9800998ecf8427e"
    },
    "options": {
        "summaryTrendStats": [
            "avg",
            "min",
            "med",
            "max",
            "p(90)",
            "p(95)",
            "p(99)",
            "count"
            ],
            "summaryTimeUnit": "",
            "noColor": false
        },
        "state": {
            "isStdErrTTY": false,
            "isStdOutTTY": false,
            "testRunDurationMs": 1000
        },
        "setup_data": 5,
        "metrics": {
        "checks": {
            "contains": "default",
            "values": {
                "passes": 45,
                "fails": 15,
                "rate": 0.75
            },
            "type": "rate",
            "thresholds": {
                "rate>70": {
                    "ok": true
                }
            }
        },
        "my_trend": {
            "thresholds": {
                "my_trend<1000": {
                    "ok": false
                }
            },
            "type": "trend",
            "contains": "time",
            "values": {
                "max": 20,
                "p(90)": 19,
                "p(95)": 19.5,
                "p(99)": 19.9,
                "count": 3,
                "avg": 15,
                "min": 10,
                "med": 15
            }
        },
        "vus": {
            "contains": "default",
            "values": {
                "value": 1,
                "min": 1,
                "max": 1
            },
            "type": "gauge"
        },
        "http_reqs": {
            "type": "counter",
            "contains": "default",
            "values": {
                "count": 3,
                "rate": 3
            },
            "thresholds": {
                "rate<100": {
                    "ok": false
                }
            }
        }
    }
}`

func TestRawHandleSummaryData(t *testing.T) {
	t.Parallel()
	runner, err := getSimpleRunner(
		t, "/script.js",
		`
		exports.options = {summaryTrendStats: ["avg", "min", "med", "max", "p(90)", "p(95)", "p(99)", "count"]};
		exports.default = function() { /* we don't run this, metrics are mocked */ };
		exports.handleSummary = function(data) {
			return {'rawdata.json': JSON.stringify(data)};
		};
		`,
		lib.RuntimeOptions{
			CompatibilityMode: null.NewString("base", true),
			// we still want to check this
			SummaryExport: null.StringFrom("old-export.json"),
		},
	)

	require.NoError(t, err)

	summary := createTestSummary(t)
	result, err := runner.HandleSummary(context.Background(), summary)
	require.NoError(t, err)

	require.Len(t, result, 2)
	require.Nil(t, result["stdout"])

	require.NotNil(t, result["old-export.json"])
	oldExport, err := io.ReadAll(result["old-export.json"])
	require.NoError(t, err)
	assert.JSONEq(t, expectedOldJSONExportResult, string(oldExport))
	require.NotNil(t, result["rawdata.json"])
	newRawData, err := io.ReadAll(result["rawdata.json"])
	require.NoError(t, err)
	assert.JSONEq(t, expectedHandleSummaryRawData, string(newRawData))
}

func TestRawHandleSummaryDataWithSetupData(t *testing.T) {
	t.Parallel()
	runner, err := getSimpleRunner(
		t, "/script.js",
		`
		exports.options = {summaryTrendStats: ["avg", "min", "med", "max", "p(90)", "p(95)", "p(99)", "count"]};
		exports.default = function() { /* we don't run this, metrics are mocked */ };
		exports.handleSummary = function(data) {
			if(data.setup_data != 5) {
				throw new Error("handleSummary: wrong data: " + JSON.stringify(data))
			}
			return {'dataWithSetup.json': JSON.stringify(data)};
		};
		`,
	)
	require.NoError(t, err)
	runner.SetSetupData([]byte("5"))

	summary := createTestSummary(t)
	result, err := runner.HandleSummary(context.Background(), summary)
	require.NoError(t, err)
	dataWithSetup, err := io.ReadAll(result["dataWithSetup.json"])
	require.NoError(t, err)
	assert.JSONEq(t, expectedHandleSummaryDataWithSetup, string(dataWithSetup))
}

func TestRawHandleSummaryPromise(t *testing.T) {
	t.Parallel()
	runner, err := getSimpleRunner(
		t, "/script.js",
		`
		exports.options = {summaryTrendStats: ["avg", "min", "med", "max", "p(90)", "p(95)", "p(99)", "count"]};
		exports.default = function() { /* we don't run this, metrics are mocked */ };
		exports.handleSummary = async function(data) {
            return await Promise.resolve({'dataWithSetup.json': JSON.stringify(data)});
		};
		`,
	)
	require.NoError(t, err)
	runner.SetSetupData([]byte("5"))

	summary := createTestSummary(t)
	result, err := runner.HandleSummary(context.Background(), summary)
	require.NoError(t, err)
	dataWithSetup, err := io.ReadAll(result["dataWithSetup.json"])
	require.NoError(t, err)
	assert.JSONEq(t, expectedHandleSummaryDataWithSetup, string(dataWithSetup))
}

func TestWrongSummaryHandlerExportTypes(t *testing.T) {
	t.Parallel()
	testCases := []string{"{}", `"foo"`, "null", "undefined", "123"}

	for i, tc := range testCases {
		i, tc := i, tc
		t.Run(fmt.Sprintf("%d_%s", i, tc), func(t *testing.T) {
			t.Parallel()
			runner, err := getSimpleRunner(t, "/script.js",
				fmt.Sprintf(`
					exports.default = function() { /* we don't run this, metrics are mocked */ };
					exports.handleSummary = %s;
				`, tc),
				lib.RuntimeOptions{CompatibilityMode: null.NewString("base", true)},
			)
			require.NoError(t, err)

			summary := createTestSummary(t)
			_, err = runner.HandleSummary(context.Background(), summary)
			require.Error(t, err)
		})
	}
}

func TestExceptionInHandleSummaryFallsBackToTextSummary(t *testing.T) {
	t.Parallel()

	logger := logrus.New()
	logger.SetOutput(io.Discard)
	logHook := testutils.NewLogHook(logrus.ErrorLevel)
	logger.AddHook(logHook)

	runner, err := getSimpleRunner(t, "/script.js", `
			exports.default = function() {/* we don't run this, metrics are mocked */};
			exports.handleSummary = function(data) {
				throw new Error('intentional error');
			};
		`, logger, lib.RuntimeOptions{CompatibilityMode: null.NewString("base", true)},
	)

	require.NoError(t, err)

	summary := createTestSummary(t)
	result, err := runner.HandleSummary(context.Background(), summary)
	require.NoError(t, err)

	require.Len(t, result, 1)
	require.NotNil(t, result["stdout"])
	textSummary, err := io.ReadAll(result["stdout"])
	require.NoError(t, err)
	assert.Contains(t, string(textSummary), checksOut+countOut)

	logErrors := logHook.Drain()
	assert.Equal(t, 1, len(logErrors))
	errMsg, err := logErrors[0].String()
	require.NoError(t, err)
	assert.Contains(t, errMsg, "intentional error")
}
