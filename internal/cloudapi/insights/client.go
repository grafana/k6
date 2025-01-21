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

	"go.k6.io/k6/internal/cloudapi/insights/proto/v1/ingester"
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
	Timeout       time.Duration
	ConnectConfig ClientConnectConfig
	AuthConfig    ClientAuthConfig
	TLSConfig     ClientTLSConfig
	RetryConfig   ClientRetryConfig
}

// ClientConnectConfig is the configuration for the client connection.
type ClientConnectConfig struct {
	Block                  bool
	FailOnNonTempDialError bool
	Timeout                time.Duration
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

// ClientRetryConfig is the configuration for the client retries.
type ClientRetryConfig struct {
	RetryableStatusCodes string
	MaxAttempts          uint
	PerRetryTimeout      time.Duration
	BackoffConfig        ClientBackoffConfig
}

// ClientBackoffConfig is the configuration for the client retries using the backoff exponential with jitter algorithm.
type ClientBackoffConfig struct {
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

// NewDefaultClientConfigForTestRun creates a new default client config for a test run.
func NewDefaultClientConfigForTestRun(ingesterHost, authToken string, testRunID int64) ClientConfig {
	return ClientConfig{
		IngesterHost: ingesterHost,
		Timeout:      90 * time.Second,
		ConnectConfig: ClientConnectConfig{
			Block:                  false,
			FailOnNonTempDialError: false,
			Timeout:                10 * time.Second,
			Dialer:                 nil,
		},
		AuthConfig: ClientAuthConfig{
			Enabled:                  true,
			TestRunID:                testRunID,
			Token:                    authToken,
			RequireTransportSecurity: true,
		},
		TLSConfig: ClientTLSConfig{
			Insecure: false,
		},
		RetryConfig: ClientRetryConfig{
			RetryableStatusCodes: `"UNKNOWN","INTERNAL","UNAVAILABLE","DEADLINE_EXCEEDED"`,
			MaxAttempts:          3,
			PerRetryTimeout:      30 * time.Second,
			BackoffConfig: ClientBackoffConfig{
				Enabled:        true,
				JitterFraction: 0.1,
				WaitBetween:    1 * time.Second,
			},
		},
	}
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

	ctx, cancel := context.WithTimeout(ctx, c.cfg.ConnectConfig.Timeout)
	defer cancel()
	//nolint:staticcheck // see https://github.com/grafana/k6/issues/3699
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

	ctx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()
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
		//nolint:staticcheck // see https://github.com/grafana/k6/issues/3699
		opts = append(opts, grpc.WithBlock())
	}

	if cfg.ConnectConfig.FailOnNonTempDialError {
		//nolint:staticcheck // see https://github.com/grafana/k6/issues/3699
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

	opts = append(opts, grpc.WithChainUnaryInterceptor([]grpc.UnaryClientInterceptor{rI}...))

	return opts, nil
}

func retryInterceptor(retryConfig ClientRetryConfig) (grpc.UnaryClientInterceptor, error) {
	rSC, err := retryableStatusCodes(retryConfig.RetryableStatusCodes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse retryable status codes: %w", err)
	}
	withCodes := grpcRetry.WithCodes(rSC...)
	withMax := grpcRetry.WithMax(retryConfig.MaxAttempts)
	withPerRetryTimeout := grpcRetry.WithPerRetryTimeout(retryConfig.PerRetryTimeout)
	callOptions := []grpcRetry.CallOption{withCodes, withMax, withPerRetryTimeout}

	backoffConfig := retryConfig.BackoffConfig
	if backoffConfig.Enabled {
		backoff := grpcRetry.WithBackoff(
			grpcRetry.BackoffExponentialWithJitter(backoffConfig.WaitBetween, backoffConfig.JitterFraction))
		callOptions = append(callOptions, backoff)
	}

	unaryInterceptor := grpcRetry.UnaryClientInterceptor(callOptions...)
	return unaryInterceptor, nil
}

func retryableStatusCodes(retryableStatusCodes string) ([]codes.Code, error) {
	if len(retryableStatusCodes) == 0 {
		return nil, fmt.Errorf("no retryable status codes provided")
	}

	statusCodes := strings.Split(retryableStatusCodes, ",")
	errorCodes := make([]codes.Code, len(statusCodes))
	for i, code := range statusCodes {
		err := errorCodes[i].UnmarshalJSON([]byte(code))
		if err != nil {
			return nil, fmt.Errorf("invalid status code %s provided", code)
		}
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
