package insights

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/cloudapi/insights"
	"go.k6.io/k6/metrics"
)

type mockWorkingInsightsClient struct {
	ingestRequestMetadatasBatchInvoked bool
	dataSent                           bool
	data                               insights.RequestMetadatas
}

func (c *mockWorkingInsightsClient) IngestRequestMetadatasBatch(ctx context.Context, data insights.RequestMetadatas) error {
	c.ingestRequestMetadatasBatchInvoked = true

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	c.dataSent = true
	c.data = data

	return nil
}

func (c *mockWorkingInsightsClient) Close() error {
	return nil
}

type mockFailingInsightsClient struct {
	err error
}

func (c *mockFailingInsightsClient) IngestRequestMetadatasBatch(_ context.Context, _ insights.RequestMetadatas) error {
	return c.err
}

func (c *mockFailingInsightsClient) Close() error {
	return nil
}

type mockRequestMetadatasCollector struct {
	data insights.RequestMetadatas
}

func (m *mockRequestMetadatasCollector) CollectRequestMetadatas(_ []metrics.SampleContainer) {
	panic("implement me")
}

func (m *mockRequestMetadatasCollector) PopAll() insights.RequestMetadatas {
	return m.data
}

func newMockRequestMetadatas() insights.RequestMetadatas {
	return insights.RequestMetadatas{
		{
			TraceID:        "test-trace-id-1",
			Start:          time.Unix(1337, 0),
			End:            time.Unix(1338, 0),
			TestRunLabels:  insights.TestRunLabels{ID: 1, Scenario: "test-scenario-1", Group: "test-group-1"},
			ProtocolLabels: insights.ProtocolHTTPLabels{URL: "test-url-1", Method: "test-method-1", StatusCode: 200},
		},
		{
			TraceID:        "test-trace-id-2",
			Start:          time.Unix(2337, 0),
			End:            time.Unix(2338, 0),
			TestRunLabels:  insights.TestRunLabels{ID: 1, Scenario: "test-scenario-2", Group: "test-group-2"},
			ProtocolLabels: insights.ProtocolHTTPLabels{URL: "test-url-2", Method: "test-method-2", StatusCode: 200},
		},
	}
}

func Test_tracesFlusher_Flush_ReturnsNoErrorWithWorkingInsightsClientAndNonCancelledContextAndNoData(t *testing.T) {
	t.Parallel()

	// Given
	data := insights.RequestMetadatas{}
	cli := &mockWorkingInsightsClient{}
	col := &mockRequestMetadatasCollector{data: data}
	flusher := NewFlusher(cli, col)

	// When
	err := flusher.Flush()

	// Then
	require.NoError(t, err)
	require.False(t, cli.ingestRequestMetadatasBatchInvoked)
	require.False(t, cli.dataSent)
	require.Empty(t, cli.data)
}

func Test_tracesFlusher_Flush_ReturnsNoErrorWithWorkingInsightsClientAndNonCancelledContextAndData(t *testing.T) {
	t.Parallel()

	// Given
	data := newMockRequestMetadatas()
	cli := &mockWorkingInsightsClient{}
	col := &mockRequestMetadatasCollector{data: data}
	flusher := NewFlusher(cli, col)

	// When
	err := flusher.Flush()

	// Then
	require.NoError(t, err)
	require.True(t, cli.ingestRequestMetadatasBatchInvoked)
	require.True(t, cli.dataSent)
	require.Equal(t, data, cli.data)
}

func Test_tracesFlusher_Flush_ReturnsErrorWithFailingInsightsClientAndNonCancelledContext(t *testing.T) {
	t.Parallel()

	// Given
	data := newMockRequestMetadatas()
	testErr := errors.New("test-error")
	cli := &mockFailingInsightsClient{err: testErr}
	col := &mockRequestMetadatasCollector{data: data}
	flusher := NewFlusher(cli, col)

	// When
	err := flusher.Flush()

	// Then
	require.ErrorIs(t, err, testErr)
}
