package insights

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"go.k6.io/k6/cloudapi/insights/proto/v1/ingester"
)

const (
	testRunIDHeader     = "X-K6TestRun-Id"
	authorizationHeader = "Authorization"
)

var (
	ErrAlreadyInitialized = errors.New("insights client already initialized")
	ErrClientClosed       = errors.New("insights client closed")
)

type ClientConfig struct {
	IngesterHost string
	AuthConfig   ClientAuthConfig
	TLSConfig    ClientTLSConfig
}

type ClientAuthConfig struct {
	Enabled                  bool
	TestRunID                int64
	Token                    string
	RequireTransportSecurity bool
}

type ClientTLSConfig struct {
	Insecure bool
	CertFile string
}

type Client struct {
	cfg    ClientConfig
	client ingester.IngesterServiceClient
	conn   *grpc.ClientConn
	connMu *sync.RWMutex
}

func NewClient(cfg ClientConfig) *Client {
	return &Client{
		cfg:    cfg,
		client: nil,
		conn:   nil,
		connMu: &sync.RWMutex{},
	}
}

func (c *Client) Dial(ctx context.Context) error {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if c.conn != nil {
		return ErrAlreadyInitialized
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

func (c *Client) IngestRequestMetadatasBatch(ctx context.Context, requestMetadatas RequestMetadatas) error {
	c.connMu.RLock()
	if c.conn == nil {
		return ErrClientClosed
	}
	c.connMu.RUnlock()

	req := newBatchCreateRequestMetadatasRequest(requestMetadatas)
	_, err := c.client.BatchCreateRequestMetadatas(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to ingest request metadatas batch: %w", err)
	}

	return nil
}

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

	if cfg.TLSConfig.Insecure {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		if cfg.TLSConfig.CertFile != "" {
			creds, err := credentials.NewClientTLSFromFile(cfg.TLSConfig.CertFile, "")
			if err != nil {
				return nil, fmt.Errorf("failed to load TLS credentials from file: %w", err)
			}
			opts = append(opts, grpc.WithTransportCredentials(creds))
		} else {
			opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
		}
	}

	if cfg.AuthConfig.Enabled {
		opts = append(opts, grpc.WithPerRPCCredentials(newPerRPCCredentials(cfg.AuthConfig)))
	}

	return opts, nil
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
