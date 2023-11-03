package redis

import (
	"context"
	"crypto/tls"
	"net"
	"time"
)

// UniversalOptions information is required by UniversalClient to establish
// connections.
type UniversalOptions struct {
	// Either a single address or a seed list of host:port addresses
	// of cluster/sentinel nodes.
	Addrs []string

	// ClientName will execute the `CLIENT SETNAME ClientName` command for each conn.
	ClientName string

	// Database to be selected after connecting to the server.
	// Only single-node and failover clients.
	DB int

	// Common options.

	Dialer    func(ctx context.Context, network, addr string) (net.Conn, error)
	OnConnect func(ctx context.Context, cn *Conn) error

	Protocol         int
	Username         string
	Password         string
	SentinelUsername string
	SentinelPassword string

	MaxRetries      int
	MinRetryBackoff time.Duration
	MaxRetryBackoff time.Duration

	DialTimeout           time.Duration
	ReadTimeout           time.Duration
	WriteTimeout          time.Duration
	ContextTimeoutEnabled bool

	// PoolFIFO uses FIFO mode for each node connection pool GET/PUT (default LIFO).
	PoolFIFO bool

	PoolSize        int
	PoolTimeout     time.Duration
	MinIdleConns    int
	MaxIdleConns    int
	ConnMaxIdleTime time.Duration
	ConnMaxLifetime time.Duration

	TLSConfig *tls.Config

	// Only cluster clients.

	MaxRedirects   int
	ReadOnly       bool
	RouteByLatency bool
	RouteRandomly  bool

	// The sentinel master name.
	// Only failover clients.

	MasterName string
}

// Cluster returns cluster options created from the universal options.
func (o *UniversalOptions) Cluster() *ClusterOptions {
	if len(o.Addrs) == 0 {
		o.Addrs = []string{"127.0.0.1:6379"}
	}

	return &ClusterOptions{
		Addrs:      o.Addrs,
		ClientName: o.ClientName,
		Dialer:     o.Dialer,
		OnConnect:  o.OnConnect,

		Protocol: o.Protocol,
		Username: o.Username,
		Password: o.Password,

		MaxRedirects:   o.MaxRedirects,
		ReadOnly:       o.ReadOnly,
		RouteByLatency: o.RouteByLatency,
		RouteRandomly:  o.RouteRandomly,

		MaxRetries:      o.MaxRetries,
		MinRetryBackoff: o.MinRetryBackoff,
		MaxRetryBackoff: o.MaxRetryBackoff,

		DialTimeout:           o.DialTimeout,
		ReadTimeout:           o.ReadTimeout,
		WriteTimeout:          o.WriteTimeout,
		ContextTimeoutEnabled: o.ContextTimeoutEnabled,

		PoolFIFO: o.PoolFIFO,

		PoolSize:        o.PoolSize,
		PoolTimeout:     o.PoolTimeout,
		MinIdleConns:    o.MinIdleConns,
		MaxIdleConns:    o.MaxIdleConns,
		ConnMaxIdleTime: o.ConnMaxIdleTime,
		ConnMaxLifetime: o.ConnMaxLifetime,

		TLSConfig: o.TLSConfig,
	}
}

// Failover returns failover options created from the universal options.
func (o *UniversalOptions) Failover() *FailoverOptions {
	if len(o.Addrs) == 0 {
		o.Addrs = []string{"127.0.0.1:26379"}
	}

	return &FailoverOptions{
		SentinelAddrs: o.Addrs,
		MasterName:    o.MasterName,
		ClientName:    o.ClientName,

		Dialer:    o.Dialer,
		OnConnect: o.OnConnect,

		DB:               o.DB,
		Protocol:         o.Protocol,
		Username:         o.Username,
		Password:         o.Password,
		SentinelUsername: o.SentinelUsername,
		SentinelPassword: o.SentinelPassword,

		MaxRetries:      o.MaxRetries,
		MinRetryBackoff: o.MinRetryBackoff,
		MaxRetryBackoff: o.MaxRetryBackoff,

		DialTimeout:           o.DialTimeout,
		ReadTimeout:           o.ReadTimeout,
		WriteTimeout:          o.WriteTimeout,
		ContextTimeoutEnabled: o.ContextTimeoutEnabled,

		PoolFIFO:        o.PoolFIFO,
		PoolSize:        o.PoolSize,
		PoolTimeout:     o.PoolTimeout,
		MinIdleConns:    o.MinIdleConns,
		MaxIdleConns:    o.MaxIdleConns,
		ConnMaxIdleTime: o.ConnMaxIdleTime,
		ConnMaxLifetime: o.ConnMaxLifetime,

		TLSConfig: o.TLSConfig,
	}
}

// Simple returns basic options created from the universal options.
func (o *UniversalOptions) Simple() *Options {
	addr := "127.0.0.1:6379"
	if len(o.Addrs) > 0 {
		addr = o.Addrs[0]
	}

	return &Options{
		Addr:       addr,
		ClientName: o.ClientName,
		Dialer:     o.Dialer,
		OnConnect:  o.OnConnect,

		DB:       o.DB,
		Protocol: o.Protocol,
		Username: o.Username,
		Password: o.Password,

		MaxRetries:      o.MaxRetries,
		MinRetryBackoff: o.MinRetryBackoff,
		MaxRetryBackoff: o.MaxRetryBackoff,

		DialTimeout:           o.DialTimeout,
		ReadTimeout:           o.ReadTimeout,
		WriteTimeout:          o.WriteTimeout,
		ContextTimeoutEnabled: o.ContextTimeoutEnabled,

		PoolFIFO:        o.PoolFIFO,
		PoolSize:        o.PoolSize,
		PoolTimeout:     o.PoolTimeout,
		MinIdleConns:    o.MinIdleConns,
		MaxIdleConns:    o.MaxIdleConns,
		ConnMaxIdleTime: o.ConnMaxIdleTime,
		ConnMaxLifetime: o.ConnMaxLifetime,

		TLSConfig: o.TLSConfig,
	}
}

// --------------------------------------------------------------------

// UniversalClient is an abstract client which - based on the provided options -
// represents either a ClusterClient, a FailoverClient, or a single-node Client.
// This can be useful for testing cluster-specific applications locally or having different
// clients in different environments.
type UniversalClient interface {
	Cmdable
	AddHook(Hook)
	Watch(ctx context.Context, fn func(*Tx) error, keys ...string) error
	Do(ctx context.Context, args ...interface{}) *Cmd
	Process(ctx context.Context, cmd Cmder) error
	Subscribe(ctx context.Context, channels ...string) *PubSub
	PSubscribe(ctx context.Context, channels ...string) *PubSub
	SSubscribe(ctx context.Context, channels ...string) *PubSub
	Close() error
	PoolStats() *PoolStats
}

var (
	_ UniversalClient = (*Client)(nil)
	_ UniversalClient = (*ClusterClient)(nil)
	_ UniversalClient = (*Ring)(nil)
)

// NewUniversalClient returns a new multi client. The type of the returned client depends
// on the following conditions:
//
// 1. If the MasterName option is specified, a sentinel-backed FailoverClient is returned.
// 2. if the number of Addrs is two or more, a ClusterClient is returned.
// 3. Otherwise, a single-node Client is returned.
func NewUniversalClient(opts *UniversalOptions) UniversalClient {
	if opts.MasterName != "" {
		return NewFailoverClient(opts.Failover())
	} else if len(opts.Addrs) > 1 {
		return NewClusterClient(opts.Cluster())
	}
	return NewClient(opts.Simple())
}
