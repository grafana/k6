package insights

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"go.k6.io/k6/internal/cloudapi/insights/proto/v1/ingester"
)

func newMockListener(t *testing.T, ingesterServer ingester.IngesterServiceServer) *bufconn.Listener {
	t.Helper()

	const size = 1024 * 1024
	l := bufconn.Listen(size)
	t.Cleanup(func() { _ = l.Close() })

	s := grpc.NewServer()
	ingester.RegisterIngesterServiceServer(s, ingesterServer)
	go func() { _ = s.Serve(l) }()
	t.Cleanup(func() { s.GracefulStop() })

	return l
}

func newMockContextDialer(t *testing.T, l *bufconn.Listener) func(context.Context, string) (net.Conn, error) {
	t.Helper()

	return func(ctx context.Context, _ string) (net.Conn, error) {
		return l.DialContext(ctx)
	}
}

type mockWorkingIngesterServer struct {
	ingester.UnimplementedIngesterServiceServer
	batchCreateRequestMetadatasInvoked bool
	dataUploaded                       bool
	maxAttemptsTillSuccess             uint
	attempts                           uint
}

func (s *mockWorkingIngesterServer) BatchCreateRequestMetadatas(ctx context.Context, _ *ingester.BatchCreateRequestMetadatasRequest) (*ingester.BatchCreateRequestMetadatasResponse, error) {
	s.batchCreateRequestMetadatasInvoked = true

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	s.dataUploaded = true

	if s.attempts >= s.maxAttemptsTillSuccess {
		return &ingester.BatchCreateRequestMetadatasResponse{
			RequestMetadatas: nil,
		}, nil
	}

	s.attempts++

	return nil, status.Error(codes.Unavailable, "unavailable")
}

type mockFailingIngesterServer struct {
	ingester.UnimplementedIngesterServiceServer
	err error
}

func (m *mockFailingIngesterServer) BatchCreateRequestMetadatas(_ context.Context, _ *ingester.BatchCreateRequestMetadatasRequest) (*ingester.BatchCreateRequestMetadatasResponse, error) {
	return nil, m.err
}

type fatalError struct{}

func (*fatalError) Error() string   { return "context dialer error" }
func (*fatalError) Temporary() bool { return false }

func TestClient_Dial_ReturnsNoErrorWithWorkingDialer(t *testing.T) {
	t.Parallel()

	// Given
	ser := &mockWorkingIngesterServer{}
	lis := newMockListener(t, ser)

	cfg := ClientConfig{
		Timeout:       1 * time.Second,
		ConnectConfig: ClientConnectConfig{Timeout: 1 * time.Second, Dialer: newMockContextDialer(t, lis)},
		TLSConfig:     ClientTLSConfig{Insecure: true},
		RetryConfig:   ClientRetryConfig{RetryableStatusCodes: `"UNKNOWN","INTERNAL","UNAVAILABLE","DEADLINE_EXCEEDED"`},
	}
	cli := NewClient(cfg)

	// When
	err := cli.Dial(context.Background())

	// Then
	require.NoError(t, err)
}

func TestClient_Dial_ReturnsErrorWhenCalledTwice(t *testing.T) {
	t.Parallel()

	// Given
	ser := &mockWorkingIngesterServer{}
	lis := newMockListener(t, ser)

	cfg := ClientConfig{
		Timeout:       1 * time.Second,
		ConnectConfig: ClientConnectConfig{Timeout: 1 * time.Second, Dialer: newMockContextDialer(t, lis)},
		TLSConfig:     ClientTLSConfig{Insecure: true},
		RetryConfig:   ClientRetryConfig{RetryableStatusCodes: `"UNKNOWN","INTERNAL","UNAVAILABLE","DEADLINE_EXCEEDED"`},
	}
	cli := NewClient(cfg)

	// When
	noErr := cli.Dial(context.Background())
	err := cli.Dial(context.Background())

	// Then
	require.NoError(t, noErr)
	require.ErrorIs(t, err, ErrClientAlreadyInitialized)
}

func TestClient_Dial_ReturnsNoErrorWithFailingDialer(t *testing.T) {
	t.Parallel()

	// Given
	cfg := ClientConfig{
		ConnectConfig: ClientConnectConfig{
			Block:                  true,
			FailOnNonTempDialError: true,
			Timeout:                1 * time.Second,
			Dialer: func(_ context.Context, _ string) (net.Conn, error) {
				return nil, &fatalError{}
			},
		},
		TLSConfig:   ClientTLSConfig{Insecure: true},
		RetryConfig: ClientRetryConfig{RetryableStatusCodes: `"UNKNOWN","INTERNAL","UNAVAILABLE","DEADLINE_EXCEEDED"`},
	}
	cli := NewClient(cfg)

	// When
	err := cli.Dial(context.Background())

	// Then
	var fatalErr *fatalError
	require.ErrorAs(t, err, &fatalErr)
}

func TestClient_Dial_ReturnsErrorWithoutRetryableStatusCodes(t *testing.T) {
	t.Parallel()

	// Given
	ser := &mockWorkingIngesterServer{}
	lis := newMockListener(t, ser)

	cfg := ClientConfig{
		Timeout:       1 * time.Second,
		ConnectConfig: ClientConnectConfig{Dialer: newMockContextDialer(t, lis)},
		TLSConfig:     ClientTLSConfig{Insecure: true},
	}
	cli := NewClient(cfg)

	// When
	err := cli.Dial(context.Background())

	// Then
	require.Error(t, err)
}

func TestClient_Dial_ReturnsErrorWithInvalidRetryableStatusCodes(t *testing.T) {
	t.Parallel()

	// Given
	ser := &mockWorkingIngesterServer{}
	lis := newMockListener(t, ser)

	cfg := ClientConfig{
		Timeout:       1 * time.Second,
		ConnectConfig: ClientConnectConfig{Dialer: newMockContextDialer(t, lis)},
		TLSConfig:     ClientTLSConfig{Insecure: true},
		RetryConfig:   ClientRetryConfig{RetryableStatusCodes: "RANDOM,INTERNAL"},
	}
	cli := NewClient(cfg)

	// When
	err := cli.Dial(context.Background())

	// Then
	require.Error(t, err)
}

func TestClient_IngestRequestMetadatasBatch_ReturnsNoErrorWithWorkingServerAndNonCancelledContextAndNoData(t *testing.T) {
	t.Parallel()

	// Given
	ser := &mockWorkingIngesterServer{}
	lis := newMockListener(t, ser)

	cfg := ClientConfig{
		Timeout:       1 * time.Second,
		ConnectConfig: ClientConnectConfig{Timeout: 1 * time.Second, Dialer: newMockContextDialer(t, lis)},
		TLSConfig:     ClientTLSConfig{Insecure: true},
		RetryConfig:   ClientRetryConfig{RetryableStatusCodes: `"UNKNOWN","INTERNAL","UNAVAILABLE","DEADLINE_EXCEEDED"`},
	}
	cli := NewClient(cfg)
	require.NoError(t, cli.Dial(context.Background()))

	// When
	err := cli.IngestRequestMetadatasBatch(context.Background(), nil)

	// Then
	require.NoError(t, err)
	require.False(t, ser.batchCreateRequestMetadatasInvoked)
	require.False(t, ser.dataUploaded)
}

func TestClient_IngestRequestMetadatasBatch_ReturnsNoErrorWithWorkingServerAndNonCancelledContextAndData(t *testing.T) {
	t.Parallel()

	// Given
	ser := &mockWorkingIngesterServer{}
	lis := newMockListener(t, ser)

	cfg := ClientConfig{
		Timeout:       1 * time.Second,
		ConnectConfig: ClientConnectConfig{Timeout: 1 * time.Second, Dialer: newMockContextDialer(t, lis)},
		TLSConfig:     ClientTLSConfig{Insecure: true},
		RetryConfig:   ClientRetryConfig{RetryableStatusCodes: `"UNKNOWN","INTERNAL","UNAVAILABLE","DEADLINE_EXCEEDED"`},
	}
	cli := NewClient(cfg)
	require.NoError(t, cli.Dial(context.Background()))
	data := RequestMetadatas{
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
	err := cli.IngestRequestMetadatasBatch(context.Background(), data)

	// Then
	require.NoError(t, err)
	require.True(t, ser.batchCreateRequestMetadatasInvoked)
	require.True(t, ser.dataUploaded)
}

func TestClient_IngestRequestMetadatasBatch_ReturnsErrorWithWorkingServerAndCancelledContext(t *testing.T) {
	t.Parallel()

	// Given
	ser := &mockWorkingIngesterServer{}
	lis := newMockListener(t, ser)

	cfg := ClientConfig{
		Timeout:       1 * time.Second,
		ConnectConfig: ClientConnectConfig{Timeout: 1 * time.Second, Dialer: newMockContextDialer(t, lis)},
		TLSConfig:     ClientTLSConfig{Insecure: true},
		RetryConfig:   ClientRetryConfig{RetryableStatusCodes: `"UNKNOWN","INTERNAL","UNAVAILABLE","DEADLINE_EXCEEDED"`},
	}
	cli := NewClient(cfg)
	require.NoError(t, cli.Dial(context.Background()))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	data := RequestMetadatas{
		{
			TraceID:        "test-trace-id-1",
			Start:          time.Unix(9, 0),
			End:            time.Unix(10, 0),
			TestRunLabels:  TestRunLabels{ID: 1337, Scenario: "test-scenario-1", Group: "test-group-1"},
			ProtocolLabels: ProtocolHTTPLabels{URL: "test-url-1", Method: "test-method-1", StatusCode: 200},
		},
	}

	// When
	err := cli.IngestRequestMetadatasBatch(ctx, data)

	// Then
	require.Error(t, err)
	require.False(t, ser.batchCreateRequestMetadatasInvoked)
	require.False(t, ser.dataUploaded)
}

func TestClient_IngestRequestMetadatasBatch_ReturnsErrorWithUninitializedClient(t *testing.T) {
	t.Parallel()

	// Given
	ser := &mockWorkingIngesterServer{}
	lis := newMockListener(t, ser)

	cfg := ClientConfig{
		Timeout:       1 * time.Second,
		ConnectConfig: ClientConnectConfig{Dialer: newMockContextDialer(t, lis)},
		TLSConfig:     ClientTLSConfig{Insecure: true},
	}
	cli := NewClient(cfg)
	data := RequestMetadatas{
		{
			TraceID:        "test-trace-id-1",
			Start:          time.Unix(9, 0),
			End:            time.Unix(10, 0),
			TestRunLabels:  TestRunLabels{ID: 1337, Scenario: "test-scenario-1", Group: "test-group-1"},
			ProtocolLabels: ProtocolHTTPLabels{URL: "test-url-1", Method: "test-method-1", StatusCode: 200},
		},
	}

	// When
	err := cli.IngestRequestMetadatasBatch(context.Background(), data)

	// Then
	require.ErrorIs(t, err, ErrClientClosed)
	require.False(t, ser.batchCreateRequestMetadatasInvoked)
	require.False(t, ser.dataUploaded)
}

func TestClient_IngestRequestMetadatasBatch_ReturnsErrorWithFailingServerAndNonCancelledContext(t *testing.T) {
	t.Parallel()

	// Given
	testErr := errors.New("test error")
	ser := &mockFailingIngesterServer{err: testErr}
	lis := newMockListener(t, ser)

	cfg := ClientConfig{
		Timeout:       1 * time.Second,
		ConnectConfig: ClientConnectConfig{Timeout: 1 * time.Second, Dialer: newMockContextDialer(t, lis)},
		TLSConfig:     ClientTLSConfig{Insecure: true},
		RetryConfig:   ClientRetryConfig{RetryableStatusCodes: `"UNKNOWN","INTERNAL","UNAVAILABLE","DEADLINE_EXCEEDED"`},
	}
	cli := NewClient(cfg)
	require.NoError(t, cli.Dial(context.Background()))
	data := RequestMetadatas{
		{
			TraceID:        "test-trace-id-1",
			Start:          time.Unix(9, 0),
			End:            time.Unix(10, 0),
			TestRunLabels:  TestRunLabels{ID: 1337, Scenario: "test-scenario-1", Group: "test-group-1"},
			ProtocolLabels: ProtocolHTTPLabels{URL: "test-url-1", Method: "test-method-1", StatusCode: 200},
		},
	}

	// When
	err := cli.IngestRequestMetadatasBatch(context.Background(), data)

	// Then
	require.ErrorContains(t, err, testErr.Error())
}

func TestClient_IngestRequestMetadatasBatch_ReturnsNoErrorAfterRetrySeveralTimes(t *testing.T) {
	t.Parallel()

	// Given
	ser := &mockWorkingIngesterServer{maxAttemptsTillSuccess: 10}
	lis := newMockListener(t, ser)

	cfg := ClientConfig{
		Timeout:       1 * time.Second,
		ConnectConfig: ClientConnectConfig{Timeout: 1 * time.Second, Dialer: newMockContextDialer(t, lis)},
		TLSConfig:     ClientTLSConfig{Insecure: true},
		RetryConfig: ClientRetryConfig{
			MaxAttempts:          20,
			RetryableStatusCodes: `"UNKNOWN","INTERNAL","UNAVAILABLE","DEADLINE_EXCEEDED"`,
			BackoffConfig: ClientBackoffConfig{
				Enabled:        true,
				JitterFraction: 0.1,
			},
		},
	}
	cli := NewClient(cfg)
	require.NoError(t, cli.Dial(context.Background()))
	data := RequestMetadatas{
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
	err := cli.IngestRequestMetadatasBatch(context.Background(), data)

	// Then
	require.NoError(t, err)
	require.True(t, ser.batchCreateRequestMetadatasInvoked)
	require.True(t, ser.dataUploaded)
	require.Equal(t, uint(10), ser.attempts)
}

func TestClient_IngestRequestMetadatasBatch_ReturnsErrorAfterExhaustingMaxRetryAttempts(t *testing.T) {
	t.Parallel()

	// Given
	ser := &mockWorkingIngesterServer{maxAttemptsTillSuccess: 10}
	lis := newMockListener(t, ser)

	cfg := ClientConfig{
		Timeout:       1 * time.Second,
		ConnectConfig: ClientConnectConfig{Timeout: 1 * time.Second, Dialer: newMockContextDialer(t, lis)},
		TLSConfig:     ClientTLSConfig{Insecure: true},
		RetryConfig: ClientRetryConfig{
			BackoffConfig: ClientBackoffConfig{
				Enabled:        true,
				JitterFraction: 0.1,
			},
			MaxAttempts:          5,
			RetryableStatusCodes: `"UNKNOWN","INTERNAL","UNAVAILABLE","DEADLINE_EXCEEDED"`,
		},
	}
	cli := NewClient(cfg)
	require.NoError(t, cli.Dial(context.Background()))
	data := RequestMetadatas{
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
	err := cli.IngestRequestMetadatasBatch(context.Background(), data)

	// Then
	require.Error(t, err)
	require.Equal(t, uint(5), ser.attempts)
}

func TestClient_Close_ReturnsNoErrorWhenClosedOnce(t *testing.T) {
	t.Parallel()

	// Given
	ser := &mockWorkingIngesterServer{}
	lis := newMockListener(t, ser)

	cfg := ClientConfig{
		ConnectConfig: ClientConnectConfig{Timeout: 1 * time.Second, Dialer: newMockContextDialer(t, lis)},
		TLSConfig:     ClientTLSConfig{Insecure: true},
		RetryConfig:   ClientRetryConfig{RetryableStatusCodes: `"UNKNOWN","INTERNAL","UNAVAILABLE","DEADLINE_EXCEEDED"`},
	}
	cli := NewClient(cfg)
	require.NoError(t, cli.Dial(context.Background()))

	// When
	err := cli.Close()

	// Then
	require.NoError(t, err)
}

func TestClient_Close_ReturnsNoErrorWhenClosedTwice(t *testing.T) {
	t.Parallel()

	// Given
	ser := &mockWorkingIngesterServer{}
	lis := newMockListener(t, ser)

	cfg := ClientConfig{
		ConnectConfig: ClientConnectConfig{Timeout: 1 * time.Second, Dialer: newMockContextDialer(t, lis)},
		TLSConfig:     ClientTLSConfig{Insecure: true},
		RetryConfig:   ClientRetryConfig{RetryableStatusCodes: `"UNKNOWN","INTERNAL","UNAVAILABLE","DEADLINE_EXCEEDED"`},
	}
	cli := NewClient(cfg)
	require.NoError(t, cli.Dial(context.Background()))

	// When
	noErr := cli.Close()
	err := cli.Close()

	// Then
	require.NoError(t, noErr)
	require.ErrorIs(t, err, ErrClientClosed)
}
