package insights

import (
	"errors"
	"fmt"

	"go.k6.io/k6/internal/cloudapi/insights/proto/v1/ingester"
	"go.k6.io/k6/internal/cloudapi/insights/proto/v1/k6"
)

func newBatchCreateRequestMetadatasRequest(
	requestMetadatas RequestMetadatas,
) (*ingester.BatchCreateRequestMetadatasRequest, error) {
	reqs := make([]*ingester.CreateRequestMetadataRequest, 0, len(requestMetadatas))
	for _, rm := range requestMetadatas {
		req, err := newCreateRequestMetadataRequest(rm)
		if err != nil {
			return nil, fmt.Errorf("failed to create request metadata request: %w", err)
		}

		reqs = append(reqs, req)
	}

	return &ingester.BatchCreateRequestMetadatasRequest{
		Requests: reqs,
	}, nil
}

func newCreateRequestMetadataRequest(requestMetadata RequestMetadata) (*ingester.CreateRequestMetadataRequest, error) {
	rm := &k6.RequestMetadata{
		TraceID:           requestMetadata.TraceID,
		StartTimeUnixNano: requestMetadata.Start.UnixNano(),
		EndTimeUnixNano:   requestMetadata.End.UnixNano(),
		TestRunLabels: &k6.TestRunLabels{
			ID:       requestMetadata.TestRunLabels.ID,
			Scenario: requestMetadata.TestRunLabels.Scenario,
			Group:    requestMetadata.TestRunLabels.Group,
		},
		ProtocolLabels: nil,
	}

	if err := setProtocolLabels(rm, requestMetadata.ProtocolLabels); err != nil {
		return nil, fmt.Errorf("failed to set protocol labels: %w", err)
	}

	return &ingester.CreateRequestMetadataRequest{
		RequestMetadata: rm,
	}, nil
}

func setProtocolLabels(rm *k6.RequestMetadata, labels ProtocolLabels) error {
	// TODO(lukasz, other-proto-support): Set other protocol labels.
	switch l := labels.(type) {
	case ProtocolHTTPLabels:
		rm.ProtocolLabels = &k6.RequestMetadata_HTTPLabels{
			HTTPLabels: &k6.HTTPLabels{
				Url:        l.URL,
				Method:     l.Method,
				StatusCode: l.StatusCode,
			},
		}
	default:
		return errors.New("unknown protocol labels type")
	}

	return nil
}
