package insights

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/cloudapi/insights/proto/v1/ingester"
	"go.k6.io/k6/v2/internal/cloudapi/insights/proto/v1/k6"
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
		{
			TraceID:       "test-trace-id-3",
			Start:         time.Unix(29, 0),
			End:           time.Unix(30, 0),
			TestRunLabels: TestRunLabels{ID: 1337, Scenario: "test-scenario-3", Group: "test-group-3"},
			ProtocolLabels: ProtocolGRPCLabels{
				URL: "test-url-3", Service: "test-service-3", Method: "test-method-3", StatusCode: 14,
			},
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
		{
			RequestMetadata: &k6.RequestMetadata{
				TraceID:           "test-trace-id-3",
				StartTimeUnixNano: time.Unix(29, 0).UnixNano(),
				EndTimeUnixNano:   time.Unix(30, 0).UnixNano(),
				TestRunLabels:     &k6.TestRunLabels{ID: 1337, Scenario: "test-scenario-3", Group: "test-group-3"},
				ProtocolLabels: &k6.RequestMetadata_GRPCLabels{
					GRPCLabels: &k6.GRPCLabels{
						Url: "test-url-3", Service: "test-service-3", Method: "test-method-3", StatusCode: 14,
					},
				},
			},
		},
	}

	require.NoError(t, err)
	require.Len(t, got.Requests, 3)
	require.ElementsMatch(t, got.Requests, expected)
}

func Test_newBatchCreateRequestMetadatasRequest_FailsOnUnknownProtocolLabels(t *testing.T) {
	t.Parallel()

	// Given
	rms := RequestMetadatas{
		{
			TraceID:        "test-trace-id-1",
			Start:          time.Unix(9, 0),
			End:            time.Unix(10, 0),
			TestRunLabels:  TestRunLabels{ID: 1337},
			ProtocolLabels: nil,
		},
	}

	// When
	got, err := newBatchCreateRequestMetadatasRequest(rms)

	// Then
	require.ErrorContains(t, err, "unknown protocol labels type")
	require.Nil(t, got)
}
