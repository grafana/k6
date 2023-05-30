package insights

import (
	"go.k6.io/k6/cloudapi/insights/proto/v1/ingester"
)

func newBatchCreateRequestMetadatasRequest(requestMetadatas RequestMetadatas) *ingester.BatchCreateRequestMetadatasRequest {
	reqs := make([]*ingester.CreateRequestMetadataRequest, 0, len(requestMetadatas))
	for _, rm := range requestMetadatas {
		reqs = append(reqs, newCreateRequestMetadataRequest(rm))
	}

	return &ingester.BatchCreateRequestMetadatasRequest{
		Requests: reqs,
	}
}

func newCreateRequestMetadataRequest(requestMetadata RequestMetadata) *ingester.CreateRequestMetadataRequest {
	setProtocolLabels := func(rm *ingester.RequestMetadata, labels ProtocolLabels) {
		// TODO(lukasz, other-proto-support): Set other protocol labels.
		switch l := labels.(type) {
		case ProtocolHTTPLabels:
			rm.ProtocolLabels = &ingester.RequestMetadata_HTTPLabels{
				HTTPLabels: &ingester.HTTPLabels{
					Url:        l.Url,
					Method:     l.Method,
					StatusCode: l.StatusCode,
				},
			}
		}
	}

	rm := &ingester.RequestMetadata{
		TraceID:           requestMetadata.TraceID,
		StartTimeUnixNano: requestMetadata.Start.UnixNano(),
		EndTimeUnixNano:   requestMetadata.End.UnixNano(),
		TestRunLabels: &ingester.TestRunLabels{
			ID:       requestMetadata.TestRunLabels.ID,
			Scenario: requestMetadata.TestRunLabels.Scenario,
			Group:    requestMetadata.TestRunLabels.Group,
		},
		ProtocolLabels: nil,
	}

	setProtocolLabels(rm, requestMetadata.ProtocolLabels)

	return &ingester.CreateRequestMetadataRequest{
		RequestMetadata: rm,
	}
}
