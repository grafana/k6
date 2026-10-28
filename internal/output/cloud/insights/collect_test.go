package insights

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/cloudapi/insights"
	"go.k6.io/k6/v2/lib/netext/httpext"
	"go.k6.io/k6/v2/metrics"
)

func Test_Collector_CollectRequestMetadatas_DoesNothingWithEmptyData(t *testing.T) {
	t.Parallel()

	// Given
	testRunID := int64(1337)
	col := NewCollector(testRunID)
	var data []metrics.SampleContainer

	// When
	col.CollectRequestMetadatas(data)

	// Then
	require.Empty(t, col.buffer)
}

func Test_Collector_CollectRequestMetadatas_FiltersAndStoresHTTPTrailsAsRequestMetadatas(t *testing.T) {
	t.Parallel()

	// Given
	testRunID := int64(1337)
	col := NewCollector(testRunID)
	data := []metrics.SampleContainer{
		&httpext.Trail{
			EndTime:  time.Unix(10, 0),
			Duration: time.Second,
			Tags: metrics.NewRegistry().RootTagSet().
				With(scenarioTag, "test-scenario-1").
				With(groupTag, "test-group-1").
				With(nameTag, "test-url-1").
				With(methodTag, "test-method-1").
				With(statusTag, "200"),
			Metadata: map[string]string{
				metadataTraceIDKey: "test-trace-id-1",
			},
		},
		&httpext.Trail{
			// HTTP trail without trace ID should be ignored
		},
		&httpext.Trail{
			EndTime:  time.Unix(20, 0),
			Duration: time.Second,
			Tags: metrics.NewRegistry().RootTagSet().
				With(scenarioTag, "test-scenario-2").
				With(groupTag, "test-group-2").
				With(nameTag, "test-url-2").
				With(methodTag, "test-method-2").
				With(statusTag, "401"),
			Metadata: map[string]string{
				metadataTraceIDKey: "test-trace-id-2",
			},
		},
		&httpext.Trail{
			EndTime:  time.Unix(20, 0),
			Duration: time.Second,
			Tags:     metrics.NewRegistry().RootTagSet(),
			// HTTP trail without `trace_id` metadata key should be ignored
			Metadata: map[string]string{},
		},
		&httpext.Trail{
			EndTime:  time.Unix(20, 0),
			Duration: time.Second,
			// If no tags are present, output should be set to `unknown`
			Tags: metrics.NewRegistry().RootTagSet(),
			Metadata: map[string]string{
				metadataTraceIDKey: "test-trace-id-3",
			},
		},
	}

	// When
	col.CollectRequestMetadatas(data)

	// Then
	require.Len(t, col.buffer, 3)
	require.Contains(t, col.buffer, insights.RequestMetadata{
		TraceID:        "test-trace-id-1",
		Start:          time.Unix(9, 0),
		End:            time.Unix(10, 0),
		TestRunLabels:  insights.TestRunLabels{ID: 1337, Scenario: "test-scenario-1", Group: "test-group-1"},
		ProtocolLabels: insights.ProtocolHTTPLabels{URL: "test-url-1", Method: "test-method-1", StatusCode: 200},
	})
	require.Contains(t, col.buffer, insights.RequestMetadata{
		TraceID:        "test-trace-id-2",
		Start:          time.Unix(19, 0),
		End:            time.Unix(20, 0),
		TestRunLabels:  insights.TestRunLabels{ID: 1337, Scenario: "test-scenario-2", Group: "test-group-2"},
		ProtocolLabels: insights.ProtocolHTTPLabels{URL: "test-url-2", Method: "test-method-2", StatusCode: 401},
	})
	require.Contains(t, col.buffer, insights.RequestMetadata{
		TraceID:        "test-trace-id-3",
		Start:          time.Unix(19, 0),
		End:            time.Unix(20, 0),
		TestRunLabels:  insights.TestRunLabels{ID: 1337, Scenario: "", Group: ""},
		ProtocolLabels: insights.ProtocolHTTPLabels{URL: "", Method: "", StatusCode: 0},
	})
}

func Test_Collector_CollectRequestMetadatas_FiltersAndStoresGRPCSamplesAsRequestMetadatas(t *testing.T) {
	t.Parallel()

	// Given
	testRunID := int64(1337)
	col := NewCollector(testRunID)
	registry := metrics.NewRegistry()
	builtinMetrics := metrics.RegisterBuiltinMetrics(registry)
	data := []metrics.SampleContainer{
		metrics.Sample{
			TimeSeries: metrics.TimeSeries{
				Metric: builtinMetrics.GRPCReqDuration,
				Tags: registry.RootTagSet().
					With(scenarioTag, "test-scenario-1").
					With(groupTag, "test-group-1").
					With(urlTag, "test-url-1").
					With(serviceTag, "test-service-1").
					With(methodTag, "test-method-1").
					With(statusTag, "0"),
			},
			Time:     time.Unix(10, 0),
			Value:    metrics.D(time.Second),
			Metadata: map[string]string{metadataTraceIDKey: "test-trace-id-1"},
		},
		metrics.Sample{
			// gRPC sample without `trace_id` metadata key should be ignored
			TimeSeries: metrics.TimeSeries{Metric: builtinMetrics.GRPCReqDuration, Tags: registry.RootTagSet()},
			Time:       time.Unix(10, 0),
			Value:      metrics.D(time.Second),
		},
		metrics.Sample{
			// Samples of other metrics should be ignored, even when they carry a trace ID
			TimeSeries: metrics.TimeSeries{Metric: builtinMetrics.HTTPReqDuration, Tags: registry.RootTagSet()},
			Time:       time.Unix(10, 0),
			Value:      1,
			Metadata:   map[string]string{metadataTraceIDKey: "test-trace-id-ignored"},
		},
		metrics.Samples{
			// Samples wrapped in other containers should be handled as well
			metrics.Sample{
				TimeSeries: metrics.TimeSeries{
					Metric: builtinMetrics.GRPCReqDuration,
					Tags: registry.RootTagSet().
						With(scenarioTag, "test-scenario-2").
						With(groupTag, "test-group-2").
						With(urlTag, "test-url-2").
						With(serviceTag, "test-service-2").
						With(methodTag, "test-method-2").
						With(statusTag, "14"),
				},
				Time:     time.Unix(20, 0),
				Value:    metrics.D(time.Second),
				Metadata: map[string]string{metadataTraceIDKey: "test-trace-id-2"},
			},
		},
		metrics.Sample{
			// If no tags are present, labels should be left empty
			TimeSeries: metrics.TimeSeries{Metric: builtinMetrics.GRPCReqDuration},
			Time:       time.Unix(20, 0),
			Value:      metrics.D(time.Second),
			Metadata:   map[string]string{metadataTraceIDKey: "test-trace-id-3"},
		},
	}

	// When
	col.CollectRequestMetadatas(data)

	// Then
	require.Len(t, col.buffer, 3)
	require.Contains(t, col.buffer, insights.RequestMetadata{
		TraceID:       "test-trace-id-1",
		Start:         time.Unix(9, 0),
		End:           time.Unix(10, 0),
		TestRunLabels: insights.TestRunLabels{ID: 1337, Scenario: "test-scenario-1", Group: "test-group-1"},
		ProtocolLabels: insights.ProtocolGRPCLabels{
			URL: "test-url-1", Service: "test-service-1", Method: "test-method-1", StatusCode: 0,
		},
	})
	require.Contains(t, col.buffer, insights.RequestMetadata{
		TraceID:       "test-trace-id-2",
		Start:         time.Unix(19, 0),
		End:           time.Unix(20, 0),
		TestRunLabels: insights.TestRunLabels{ID: 1337, Scenario: "test-scenario-2", Group: "test-group-2"},
		ProtocolLabels: insights.ProtocolGRPCLabels{
			URL: "test-url-2", Service: "test-service-2", Method: "test-method-2", StatusCode: 14,
		},
	})
	require.Contains(t, col.buffer, insights.RequestMetadata{
		TraceID:        "test-trace-id-3",
		Start:          time.Unix(19, 0),
		End:            time.Unix(20, 0),
		TestRunLabels:  insights.TestRunLabels{ID: 1337, Scenario: "", Group: ""},
		ProtocolLabels: insights.ProtocolGRPCLabels{URL: "", Service: "", Method: "", StatusCode: 0},
	})
}

func Test_Collector_PopAll_DoesNothingWithEmptyData(t *testing.T) {
	t.Parallel()

	// Given
	data := insights.RequestMetadatas{
		insights.RequestMetadata{
			TraceID:        "test-trace-id-1",
			Start:          time.Unix(9, 0),
			End:            time.Unix(10, 0),
			TestRunLabels:  insights.TestRunLabels{ID: 1337, Scenario: "test-scenario-1", Group: "test-group-1"},
			ProtocolLabels: insights.ProtocolHTTPLabels{URL: "test-url-1", Method: "test-method-1", StatusCode: 200},
		},
		insights.RequestMetadata{
			TraceID:        "test-trace-id-2",
			Start:          time.Unix(19, 0),
			End:            time.Unix(20, 0),
			TestRunLabels:  insights.TestRunLabels{ID: 1337, Scenario: "unknown", Group: "unknown"},
			ProtocolLabels: insights.ProtocolHTTPLabels{URL: "unknown", Method: "unknown", StatusCode: 0},
		},
	}
	col := &Collector{
		buffer:   data,
		bufferMu: &sync.Mutex{},
	}

	// When
	got := col.PopAll()

	// Then
	require.Nil(t, col.buffer)
	require.Empty(t, col.buffer)
	require.Equal(t, data, got)
}
