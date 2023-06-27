package insights

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	grpcRetry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"go.k6.io/k6/cloudapi/insights/proto/v1/ingester"
)

const (
	testRunIDHeader     = "X-K6TestRun-Id"
	authorizationHeader = "Authorization"
)

var (
	// ErrClientAlreadyInitialized is returned when the client is already initialized.
	ErrClientAlreadyInitialized = errors.New("insights client already initialized")

	// ErrClientClosed is returned when the client is closed.
	ErrClientClosed = errors.New("insights client closed")
)

// ClientConfig is the configuration for the client.
type ClientConfig struct {
	IngesterHost  string
	ConnectConfig ClientConnectConfig
	AuthConfig    ClientAuthConfig
	TLSConfig     ClientTLSConfig
	RetryConfig   RetryConfig
}

// ClientConnectConfig is the configuration for the client connection.
type ClientConnectConfig struct {
	Block                  bool
	FailOnNonTempDialError bool
	Dialer                 func(context.Context, string) (net.Conn, error)
}

// ClientAuthConfig is the configuration for the client authentication.
type ClientAuthConfig struct {
	Enabled                  bool
	TestRunID                int64
	Token                    string
	RequireTransportSecurity bool
}

// ClientTLSConfig is the configuration for the client TLS.
type ClientTLSConfig struct {
	Insecure bool
	CertFile string
}

// RetryConfig is the configuration for the client retries.
type RetryConfig struct {
	RetryableStatusCodes string
	MaxAttempts          uint
	PerRetryTimeout      time.Duration
	BackoffConfig        BackoffConfig
}

// BackoffConfig is the configuration for the client retries using the backoff exponential with jitter algorithm.
type BackoffConfig struct {
	Enabled        bool
	JitterFraction float64
	WaitBetween    time.Duration
}

// Client is the client for the k6 Insights ingester service.
type Client struct {
	cfg    ClientConfig
	client ingester.IngesterServiceClient
	conn   *grpc.ClientConn
	connMu *sync.RWMutex
}

// NewClient creates a new client.
func NewClient(cfg ClientConfig) *Client {
	return &Client{
		cfg:    cfg,
		client: nil,
		conn:   nil,
		connMu: &sync.RWMutex{},
	}
}

// Dial creates a client connection using ClientConfig.
func (c *Client) Dial(ctx context.Context) error {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if c.conn != nil {
		return ErrClientAlreadyInitialized
	}

	opts, err := dialOptionsFromClientConfig(c.cfg)
	if err != nil {
		return fmt.Errorf("failed to create dial options: %w", err)
	}

	conn, err := grpc.DialContext(ctx, c.cfg.IngesterHost, opts...)
	if err != nil {
		return fmt.Errorf("failed to dial: %w", err)
	}

	c.client = ingester.NewIngesterServiceClient(conn)
	c.conn = conn

	return nil
}

// IngestRequestMetadatasBatch ingests a batch of request metadatas.
func (c *Client) IngestRequestMetadatasBatch(ctx context.Context, requestMetadatas RequestMetadatas) error {
	c.connMu.RLock()
	closed := c.conn == nil
	c.connMu.RUnlock()
	if closed {
		return ErrClientClosed
	}

	if len(requestMetadatas) < 1 {
		return nil
	}

	req, err := newBatchCreateRequestMetadatasRequest(requestMetadatas)
	if err != nil {
		return fmt.Errorf("failed to create request from request metadatas: %w", err)
	}

	_, err = c.client.BatchCreateRequestMetadatas(ctx, req)
	if err != nil {
		st := status.Convert(err)
		return fmt.Errorf("failed to ingest request metadatas batch: code=%s, msg=%s", st.Code().String(), st.Message())
	}

	return nil
}

// Close closes the client.
func (c *Client) Close() error {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if c.conn == nil {
		return ErrClientClosed
	}

	conn := c.conn
	c.client = nil
	c.conn = nil

	return conn.Close()
}

func dialOptionsFromClientConfig(cfg ClientConfig) ([]grpc.DialOption, error) {
	var opts []grpc.DialOption

	if cfg.ConnectConfig.Block {
		opts = append(opts, grpc.WithBlock())
	}

	if cfg.ConnectConfig.FailOnNonTempDialError {
		opts = append(opts, grpc.FailOnNonTempDialError(true))
	}

	if cfg.ConnectConfig.Dialer != nil {
		opts = append(opts, grpc.WithContextDialer(cfg.ConnectConfig.Dialer))
	}

	if cfg.TLSConfig.Insecure { //nolint: nestif
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		if cfg.TLSConfig.CertFile != "" {
			creds, err := credentials.NewClientTLSFromFile(cfg.TLSConfig.CertFile, "")
			if err != nil {
				return nil, fmt.Errorf("failed to load TLS credentials from file: %w", err)
			}
			opts = append(opts, grpc.WithTransportCredentials(creds))
		} else {
			opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS13})))
		}
	}

	if cfg.AuthConfig.Enabled {
		opts = append(opts, grpc.WithPerRPCCredentials(newPerRPCCredentials(cfg.AuthConfig)))
	}

	rI, err := retryInterceptor(cfg.RetryConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create retry interceptors: %w", err)
	}

	opts = append(opts, grpc.WithChainUnaryInterceptor(rI...))

	return opts, nil
}

func retryInterceptor(retryConfig RetryConfig) ([]grpc.UnaryClientInterceptor, error) {
	rSC, err := retryableStatusCodes(retryConfig.RetryableStatusCodes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse retryable status codes: %w", err)
	}
	withCodes := grpcRetry.WithCodes(rSC...)
	withMax := grpcRetry.WithMax(retryConfig.MaxAttempts)
	withPerRetryTimeout := grpcRetry.WithPerRetryTimeout(retryConfig.PerRetryTimeout)

	backoffConfig := retryConfig.BackoffConfig
	if backoffConfig.Enabled {
		backoff := grpcRetry.WithBackoff(grpcRetry.BackoffExponentialWithJitter(backoffConfig.WaitBetween, backoffConfig.JitterFraction))
		unaryInterceptor := grpcRetry.UnaryClientInterceptor(withCodes, withMax, withPerRetryTimeout, backoff)
		return []grpc.UnaryClientInterceptor{unaryInterceptor}, nil
	}

	unaryInterceptor := grpcRetry.UnaryClientInterceptor(withCodes, withMax, withPerRetryTimeout)
	return []grpc.UnaryClientInterceptor{unaryInterceptor}, nil
}

func retryableStatusCodes(retryableStatusCodes string) ([]codes.Code, error) {
	statusCodeMap := map[string]codes.Code{
		"OK":                  codes.OK,
		"CANCELLED":           codes.Canceled,
		"UNKNOWN":             codes.Unknown,
		"INVALID_ARGUMENT":    codes.InvalidArgument,
		"DEADLINE_EXCEEDED":   codes.DeadlineExceeded,
		"NOT_FOUND":           codes.NotFound,
		"ALREADY_EXISTS":      codes.AlreadyExists,
		"PERMISSION_DENIED":   codes.PermissionDenied,
		"UNAUTHENTICATED":     codes.Unauthenticated,
		"RESOURCE_EXHAUSTED":  codes.ResourceExhausted,
		"FAILED_PRECONDITION": codes.FailedPrecondition,
		"ABORTED":             codes.Aborted,
		"OUT_OF_RANGE":        codes.OutOfRange,
		"UNIMPLEMENTED":       codes.Unimplemented,
		"INTERNAL":            codes.Internal,
		"UNAVAILABLE":         codes.Unavailable,
		"DATA_LOSS":           codes.DataLoss,
	}

	if len(retryableStatusCodes) == 0 {
		return nil, fmt.Errorf("no retryable status codes provided")
	}

	var errorCodes []codes.Code
	for _, code := range strings.Split(retryableStatusCodes, ",") {
		errorCode, ok := statusCodeMap[code]
		if !ok {
			return nil, fmt.Errorf("invalid status code %s provided", code)
		}
		errorCodes = append(errorCodes, errorCode)
	}

	return errorCodes, nil
}

type perRPCCredentials struct {
	metadata                 map[string]string
	requireTransportSecurity bool
}

func newPerRPCCredentials(cfg ClientAuthConfig) perRPCCredentials {
	return perRPCCredentials{
		metadata: map[string]string{
			testRunIDHeader:     fmt.Sprintf("%d", cfg.TestRunID),
			authorizationHeader: fmt.Sprintf("Token %s", cfg.Token),
		},
		requireTransportSecurity: cfg.RequireTransportSecurity,
	}
}

func (c perRPCCredentials) GetRequestMetadata(_ context.Context, _ ...string) (map[string]string, error) {
	return c.metadata, nil
}

func (c perRPCCredentials) RequireTransportSecurity() bool {
	return c.requireTransportSecurity
}
