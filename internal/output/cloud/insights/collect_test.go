package insights

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/cloudapi/insights"
	"go.k6.io/k6/lib/netext/httpext"
	"go.k6.io/k6/metrics"
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

func Test_Collector_PopAll_DoesNothingWithEmptyData(t *testing.T) {
	t.Parallel()

	// Given
	data := insights.RequestMetadatas{
		{
			TraceID:        "test-trace-id-1",
			Start:          time.Unix(9, 0),
			End:            time.Unix(10, 0),
			TestRunLabels:  insights.TestRunLabels{ID: 1337, Scenario: "test-scenario-1", Group: "test-group-1"},
			ProtocolLabels: insights.ProtocolHTTPLabels{URL: "test-url-1", Method: "test-method-1", StatusCode: 200},
		},
		{
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
