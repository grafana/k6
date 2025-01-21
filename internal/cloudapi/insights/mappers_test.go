package insights

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/cloudapi/insights/proto/v1/ingester"
	"go.k6.io/k6/internal/cloudapi/insights/proto/v1/k6"
)

func Test_newBatchCreateRequestMetadatasRequest_CorrectlyMapsDomainTypeToProtoDefinition(t *testing.T) {
	t.Parallel()

	// Given
	rms := RequestMetadatas{
		{
			TraceID:        "test-trace-id-1",
			Start:          time.Unix(9, 0),
			End:            time.Unix(10, 0),
			TestRunLabels:  TestRunLabels{ID: 1337, Scenario: "test-scenario-1", Group: "test-group-1"},
			ProtocolLabels: ProtocolHTTPLabels{URL: "test-url-1", Method: "test-method-1", StatusCode: 200},
		},
		{
			TraceID:        "test-trace-id-2",
			Start:          time.Unix(19, 0),
			End:            time.Unix(20, 0),
			TestRunLabels:  TestRunLabels{ID: 1337, Scenario: "test-scenario-2", Group: "test-group-2"},
			ProtocolLabels: ProtocolHTTPLabels{URL: "test-url-2", Method: "test-method-2", StatusCode: 401},
		},
	}

	// When
	got, err := newBatchCreateRequestMetadatasRequest(rms)

	// Then
	expected := []*ingester.CreateRequestMetadataRequest{
		{
			RequestMetadata: &k6.RequestMetadata{
				TraceID:           "test-trace-id-1",
				StartTimeUnixNano: time.Unix(9, 0).UnixNano(),
				EndTimeUnixNano:   time.Unix(10, 0).UnixNano(),
				TestRunLabels:     &k6.TestRunLabels{ID: 1337, Scenario: "test-scenario-1", Group: "test-group-1"},
				ProtocolLabels: &k6.RequestMetadata_HTTPLabels{
					HTTPLabels: &k6.HTTPLabels{
						Url: "test-url-1", Method: "test-method-1", StatusCode: 200,
					},
				},
			},
		},
		{
			RequestMetadata: &k6.RequestMetadata{
				TraceID:           "test-trace-id-2",
				StartTimeUnixNano: time.Unix(19, 0).UnixNano(),
				EndTimeUnixNano:   time.Unix(20, 0).UnixNano(),
				TestRunLabels:     &k6.TestRunLabels{ID: 1337, Scenario: "test-scenario-2", Group: "test-group-2"},
				ProtocolLabels: &k6.RequestMetadata_HTTPLabels{
					HTTPLabels: &k6.HTTPLabels{
						Url: "test-url-2", Method: "test-method-2", StatusCode: 401,
					},
				},
			},
		},
	}

	require.NoError(t, err)
	require.Len(t, got.Requests, 2)
	require.ElementsMatch(t, got.Requests, expected)
}
