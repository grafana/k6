package redis

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9/auth"
	"github.com/redis/go-redis/v9/internal"
	"github.com/redis/go-redis/v9/internal/pool"
	"github.com/redis/go-redis/v9/internal/rand"
	"github.com/redis/go-redis/v9/internal/util"
)

//------------------------------------------------------------------------------

// FailoverOptions are used to configure a failover client and should
// be passed to NewFailoverClient.
type FailoverOptions struct {
	// The master name.
	MasterName string
	// A seed list of host:port addresses of sentinel nodes.
	SentinelAddrs []string

	// ClientName will execute the `CLIENT SETNAME ClientName` command for each conn.
	ClientName string

	// If specified with SentinelPassword, enables ACL-based authentication (via
	// AUTH <user> <pass>).
	SentinelUsername string
	// Sentinel password from "requirepass <password>" (if enabled) in Sentinel
	// configuration, or, if SentinelUsername is also supplied, used for ACL-based
	// authentication.
	SentinelPassword string

	// Allows routing read-only commands to the closest master or replica node.
	// This option only works with NewFailoverClusterClient.
	RouteByLatency bool
	// Allows routing read-only commands to the random master or replica node.
	// This option only works with NewFailoverClusterClient.
	RouteRandomly bool

	// Route all commands to replica read-only nodes.
	ReplicaOnly bool

	// Use replicas disconnected with master when cannot get connected replicas
	// Now, this option only works in RandomReplicaAddr function.
	UseDisconnectedReplicas bool

	// Following options are copied from Options struct.

	Dialer    func(ctx context.Context, network, addr string) (net.Conn, error)
	OnConnect func(ctx context.Context, cn *Conn) error

	Protocol int
	Username string
	Password string
	// CredentialsProvider allows the username and password to be updated
	// before reconnecting. It should return the current username and password.
	CredentialsProvider func() (username string, password string)

	// CredentialsProviderContext is an enhanced parameter of CredentialsProvider,
	// done to maintain API compatibility. In the future,
	// there might be a merge between CredentialsProviderContext and CredentialsProvider.
	// There will be a conflict between them; if CredentialsProviderContext exists, we will ignore CredentialsProvider.
	CredentialsProviderContext func(ctx context.Context) (username string, password string, err error)

	// StreamingCredentialsProvider is used to retrieve the credentials
	// for the connection from an external source. Those credentials may change
	// during the connection lifetime. This is useful for managed identity
	// scenarios where the credentials are retrieved from an external source.
	//
	// Currently, this is a placeholder for the future implementation.
	StreamingCredentialsProvider auth.StreamingCredentialsProvider
	DB                           int

	MaxRetries      int
	MinRetryBackoff time.Duration
	MaxRetryBackoff time.Duration

	DialTimeout           time.Duration
	ReadTimeout           time.Duration
	WriteTimeout          time.Duration
	ContextTimeoutEnabled bool

	// ReadBufferSize is the size of the bufio.Reader buffer for each connection.
	// Larger buffers can improve performance for commands that return large responses.
	// Smaller buffers can improve memory usage for larger pools.
	//
	// default: 32KiB (32768 bytes)
	ReadBufferSize int

	// WriteBufferSize is the size of the bufio.Writer buffer for each connection.
	// Larger buffers can improve performance for large pipelines and commands with many arguments.
	// Smaller buffers can improve memory usage for larger pools.
	//
	// default: 32KiB (32768 bytes)
	WriteBufferSize int

	PoolFIFO bool

	PoolSize        int
	PoolTimeout     time.Duration
	MinIdleConns    int
	MaxIdleConns    int
	MaxActiveConns  int
	ConnMaxIdleTime time.Duration
	ConnMaxLifetime time.Duration

	TLSConfig *tls.Config

	// DisableIndentity - Disable set-lib on connect.
	//
	// default: false
	//
	// Deprecated: Use DisableIdentity instead.
	DisableIndentity bool

	// DisableIdentity is used to disable CLIENT SETINFO command on connect.
	//
	// default: false
	DisableIdentity bool

	IdentitySuffix string

	// FailingTimeoutSeconds is the timeout in seconds for marking a cluster node as failing.
	// When a node is marked as failing, it will be avoided for this duration.
	// Only applies to failover cluster clients. Default is 15 seconds.
	FailingTimeoutSeconds int

	UnstableResp3 bool
}

func (opt *FailoverOptions) clientOptions() *Options {
	return &Options{
		Addr:       "FailoverClient",
		ClientName: opt.ClientName,

		Dialer:    opt.Dialer,
		OnConnect: opt.OnConnect,

		DB:                           opt.DB,
		Protocol:                     opt.Protocol,
		Username:                     opt.Username,
		Password:                     opt.Password,
		CredentialsProvider:          opt.CredentialsProvider,
		CredentialsProviderContext:   opt.CredentialsProviderContext,
		StreamingCredentialsProvider: opt.StreamingCredentialsProvider,

		MaxRetries:      opt.MaxRetries,
		MinRetryBackoff: opt.MinRetryBackoff,
		MaxRetryBackoff: opt.MaxRetryBackoff,

		ReadBufferSize:  opt.ReadBufferSize,
		WriteBufferSize: opt.WriteBufferSize,

		DialTimeout:           opt.DialTimeout,
		ReadTimeout:           opt.ReadTimeout,
		WriteTimeout:          opt.WriteTimeout,
		ContextTimeoutEnabled: opt.ContextTimeoutEnabled,

		PoolFIFO:        opt.PoolFIFO,
		PoolSize:        opt.PoolSize,
		PoolTimeout:     opt.PoolTimeout,
		MinIdleConns:    opt.MinIdleConns,
		MaxIdleConns:    opt.MaxIdleConns,
		MaxActiveConns:  opt.MaxActiveConns,
		ConnMaxIdleTime: opt.ConnMaxIdleTime,
		ConnMaxLifetime: opt.ConnMaxLifetime,

		TLSConfig: opt.TLSConfig,

		DisableIdentity:  opt.DisableIdentity,
		DisableIndentity: opt.DisableIndentity,

		IdentitySuffix: opt.IdentitySuffix,
		UnstableResp3:  opt.UnstableResp3,
	}
}

func (opt *FailoverOptions) sentinelOptions(addr string) *Options {
	return &Options{
		Addr:       addr,
		ClientName: opt.ClientName,

		Dialer:    opt.Dialer,
		OnConnect: opt.OnConnect,

		DB:       0,
		Username: opt.SentinelUsername,
		Password: opt.SentinelPassword,

		MaxRetries:      opt.MaxRetries,
		MinRetryBackoff: opt.MinRetryBackoff,
		MaxRetryBackoff: opt.MaxRetryBackoff,

		// The sentinel client uses a 4KiB read/write buffer size.
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,

		DialTimeout:           opt.DialTimeout,
		ReadTimeout:           opt.ReadTimeout,
		WriteTimeout:          opt.WriteTimeout,
		ContextTimeoutEnabled: opt.ContextTimeoutEnabled,

		PoolFIFO:        opt.PoolFIFO,
		PoolSize:        opt.PoolSize,
		PoolTimeout:     opt.PoolTimeout,
		MinIdleConns:    opt.MinIdleConns,
		MaxIdleConns:    opt.MaxIdleConns,
		MaxActiveConns:  opt.MaxActiveConns,
		ConnMaxIdleTime: opt.ConnMaxIdleTime,
		ConnMaxLifetime: opt.ConnMaxLifetime,

		TLSConfig: opt.TLSConfig,

		DisableIdentity:  opt.DisableIdentity,
		DisableIndentity: opt.DisableIndentity,

		IdentitySuffix: opt.IdentitySuffix,
		UnstableResp3:  opt.UnstableResp3,
	}
}

func (opt *FailoverOptions) clusterOptions() *ClusterOptions {
	return &ClusterOptions{
		ClientName: opt.ClientName,

		Dialer:    opt.Dialer,
		OnConnect: opt.OnConnect,

		Protocol:                     opt.Protocol,
		Username:                     opt.Username,
		Password:                     opt.Password,
		CredentialsProvider:          opt.CredentialsProvider,
		CredentialsProviderContext:   opt.CredentialsProviderContext,
		StreamingCredentialsProvider: opt.StreamingCredentialsProvider,

		MaxRedirects: opt.MaxRetries,

		ReadOnly:       opt.ReplicaOnly,
		RouteByLatency: opt.RouteByLatency,
		RouteRandomly:  opt.RouteRandomly,

		MinRetryBackoff: opt.MinRetryBackoff,
		MaxRetryBackoff: opt.MaxRetryBackoff,

		ReadBufferSize:  opt.ReadBufferSize,
		WriteBufferSize: opt.WriteBufferSize,

		DialTimeout:           opt.DialTimeout,
		ReadTimeout:           opt.ReadTimeout,
		WriteTimeout:          opt.WriteTimeout,
		ContextTimeoutEnabled: opt.ContextTimeoutEnabled,

		PoolFIFO:        opt.PoolFIFO,
		PoolSize:        opt.PoolSize,
		PoolTimeout:     opt.PoolTimeout,
		MinIdleConns:    opt.MinIdleConns,
		MaxIdleConns:    opt.MaxIdleConns,
		MaxActiveConns:  opt.MaxActiveConns,
		ConnMaxIdleTime: opt.ConnMaxIdleTime,
		ConnMaxLifetime: opt.ConnMaxLifetime,

		TLSConfig: opt.TLSConfig,

		DisableIdentity:       opt.DisableIdentity,
		DisableIndentity:      opt.DisableIndentity,
		IdentitySuffix:        opt.IdentitySuffix,
		FailingTimeoutSeconds: opt.FailingTimeoutSeconds,
	}
}

// ParseFailoverURL parses a URL into FailoverOptions that can be used to connect to Redis.
// The URL must be in the form:
//
//	redis://<user>:<password>@<host>:<port>/<db_number>
//	or
//	rediss://<user>:<password>@<host>:<port>/<db_number>
//
// To add additional addresses, specify the query parameter, "addr" one or more times. e.g:
//
//	redis://<user>:<password>@<host>:<port>/<db_number>?addr=<host2>:<port2>&addr=<host3>:<port3>
//	or
//	rediss://<user>:<password>@<host>:<port>/<db_number>?addr=<host2>:<port2>&addr=<host3>:<port3>
//
// Most Option fields can be set using query parameters, with the following restrictions:
//   - field names are mapped using snake-case conversion: to set MaxRetries, use max_retries
//   - only scalar type fields are supported (bool, int, time.Duration)
//   - for time.Duration fields, values must be a valid input for time.ParseDuration();
//     additionally a plain integer as value (i.e. without unit) is interpreted as seconds
//   - to disable a duration field, use value less than or equal to 0; to use the default
//     value, leave the value blank or remove the parameter
//   - only the last value is interpreted if a parameter is given multiple times
//   - fields "network", "addr", "sentinel_username" and "sentinel_password" can only be set using other
//     URL attributes (scheme, host, userinfo, resp.), query parameters using these
//     names will be treated as unknown parameters
//   - unknown parameter names will result in an error
//   - use "skip_verify=true" to ignore TLS certificate validation
//
// Example:
//
//	redis://user:password@localhost:6789?master_name=mymaster&dial_timeout=3&read_timeout=6s&addr=localhost:6790&addr=localhost:6791
//	is equivalent to:
//	&FailoverOptions{
//		MasterName:  "mymaster",
//		Addr:        ["localhost:6789", "localhost:6790", "localhost:6791"]
//		DialTimeout: 3 * time.Second, // no time unit = seconds
//		ReadTimeout: 6 * time.Second,
//	}
func ParseFailoverURL(redisURL string) (*FailoverOptions, error) {
	u, err := url.Parse(redisURL)
	if err != nil {
		return nil, err
	}
	return setupFailoverConn(u)
}

func setupFailoverConn(u *url.URL) (*FailoverOptions, error) {
	o := &FailoverOptions{}

	o.SentinelUsername, o.SentinelPassword = getUserPassword(u)

	h, p := getHostPortWithDefaults(u)
	o.SentinelAddrs = append(o.SentinelAddrs, net.JoinHostPort(h, p))

	switch u.Scheme {
	case "rediss":
		o.TLSConfig = &tls.Config{ServerName: h, MinVersion: tls.VersionTLS12}
	case "redis":
		o.TLSConfig = nil
	default:
		return nil, fmt.Errorf("redis: invalid URL scheme: %s", u.Scheme)
	}

	f := strings.FieldsFunc(u.Path, func(r rune) bool {
		return r == '/'
	})
	switch len(f) {
	case 0:
		o.DB = 0
	case 1:
		var err error
		if o.DB, err = strconv.Atoi(f[0]); err != nil {
			return nil, fmt.Errorf("redis: invalid database number: %q", f[0])
		}
	default:
		return nil, fmt.Errorf("redis: invalid URL path: %s", u.Path)
	}

	return setupFailoverConnParams(u, o)
}

func setupFailoverConnParams(u *url.URL, o *FailoverOptions) (*FailoverOptions, error) {
	q := queryOptions{q: u.Query()}

	o.MasterName = q.string("master_name")
	o.ClientName = q.string("client_name")
	o.RouteByLatency = q.bool("route_by_latency")
	o.RouteRandomly = q.bool("route_randomly")
	o.ReplicaOnly = q.bool("replica_only")
	o.UseDisconnectedReplicas = q.bool("use_disconnected_replicas")
	o.Protocol = q.int("protocol")
	o.Username = q.string("username")
	o.Password = q.string("password")
	o.MaxRetries = q.int("max_retries")
	o.MinRetryBackoff = q.duration("min_retry_backoff")
	o.MaxRetryBackoff = q.duration("max_retry_backoff")
	o.DialTimeout = q.duration("dial_timeout")
	o.ReadTimeout = q.duration("read_timeout")
	o.WriteTimeout = q.duration("write_timeout")
	o.ContextTimeoutEnabled = q.bool("context_timeout_enabled")
	o.PoolFIFO = q.bool("pool_fifo")
	o.PoolSize = q.int("pool_size")
	o.MinIdleConns = q.int("min_idle_conns")
	o.MaxIdleConns = q.int("max_idle_conns")
	o.MaxActiveConns = q.int("max_active_conns")
	o.ConnMaxLifetime = q.duration("conn_max_lifetime")
	o.ConnMaxIdleTime = q.duration("conn_max_idle_time")
	o.PoolTimeout = q.duration("pool_timeout")
	o.DisableIdentity = q.bool("disableIdentity")
	o.IdentitySuffix = q.string("identitySuffix")
	o.UnstableResp3 = q.bool("unstable_resp3")

	if q.err != nil {
		return nil, q.err
	}

	if tmp := q.string("db"); tmp != "" {
		db, err := strconv.Atoi(tmp)
		if err != nil {
			return nil, fmt.Errorf("redis: invalid database number: %w", err)
		}
		o.DB = db
	}

	addrs := q.strings("addr")
	for _, addr := range addrs {
		h, p, err := net.SplitHostPort(addr)
		if err != nil || h == "" || p == "" {
			return nil, fmt.Errorf("redis: unable to parse addr param: %s", addr)
		}

		o.SentinelAddrs = append(o.SentinelAddrs, net.JoinHostPort(h, p))
	}

	if o.TLSConfig != nil && q.has("skip_verify") {
		o.TLSConfig.InsecureSkipVerify = q.bool("skip_verify")
	}

	// any parameters left?
	if r := q.remaining(); len(r) > 0 {
		return nil, fmt.Errorf("redis: unexpected option: %s", strings.Join(r, ", "))
	}

	return o, nil
}

// NewFailoverClient returns a Redis client that uses Redis Sentinel
// for automatic failover. It's safe for concurrent use by multiple
// goroutines.
func NewFailoverClient(failoverOpt *FailoverOptions) *Client {
	if failoverOpt == nil {
		panic("redis: NewFailoverClient nil options")
	}

	if failoverOpt.RouteByLatency {
		panic("to route commands by latency, use NewFailoverClusterClient")
	}
	if failoverOpt.RouteRandomly {
		panic("to route commands randomly, use NewFailoverClusterClient")
	}

	sentinelAddrs := make([]string, len(failoverOpt.SentinelAddrs))
	copy(sentinelAddrs, failoverOpt.SentinelAddrs)

	rand.Shuffle(len(sentinelAddrs), func(i, j int) {
		sentinelAddrs[i], sentinelAddrs[j] = sentinelAddrs[j], sentinelAddrs[i]
	})

	failover := &sentinelFailover{
		opt:           failoverOpt,
		sentinelAddrs: sentinelAddrs,
	}

	opt := failoverOpt.clientOptions()
	opt.Dialer = masterReplicaDialer(failover)
	opt.init()

	var connPool *pool.ConnPool

	rdb := &Client{
		baseClient: &baseClient{
			opt: opt,
		},
	}
	rdb.init()

	connPool = newConnPool(opt, rdb.dialHook)
	rdb.connPool = connPool
	rdb.onClose = rdb.wrappedOnClose(failover.Close)

	failover.mu.Lock()
	failover.onFailover = func(ctx context.Context, addr string) {
		_ = connPool.Filter(func(cn *pool.Conn) bool {
			return cn.RemoteAddr().String() != addr
		})
	}
	failover.mu.Unlock()

	return rdb
}

func masterReplicaDialer(
	failover *sentinelFailover,
) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, _ string) (net.Conn, error) {
		var addr string
		var err error

		if failover.opt.ReplicaOnly {
			addr, err = failover.RandomReplicaAddr(ctx)
		} else {
			addr, err = failover.MasterAddr(ctx)
			if err == nil {
				failover.trySwitchMaster(ctx, addr)
			}
		}
		if err != nil {
			return nil, err
		}
		if failover.opt.Dialer != nil {
			return failover.opt.Dialer(ctx, network, addr)
		}

		netDialer := &net.Dialer{
			Timeout:   failover.opt.DialTimeout,
			KeepAlive: 5 * time.Minute,
		}
		if failover.opt.TLSConfig == nil {
			return netDialer.DialContext(ctx, network, addr)
		}
		return tls.DialWithDialer(netDialer, network, addr, failover.opt.TLSConfig)
	}
}

//------------------------------------------------------------------------------

// SentinelClient is a client for a Redis Sentinel.
type SentinelClient struct {
	*baseClient
}

func NewSentinelClient(opt *Options) *SentinelClient {
	if opt == nil {
		panic("redis: NewSentinelClient nil options")
	}
	opt.init()
	c := &SentinelClient{
		baseClient: &baseClient{
			opt: opt,
		},
	}

	c.initHooks(hooks{
		dial:    c.baseClient.dial,
		process: c.baseClient.process,
	})
	c.connPool = newConnPool(opt, c.dialHook)

	return c
}

func (c *SentinelClient) Process(ctx context.Context, cmd Cmder) error {
	err := c.processHook(ctx, cmd)
	cmd.SetErr(err)
	return err
}

func (c *SentinelClient) pubSub() *PubSub {
	pubsub := &PubSub{
		opt: c.opt,

		newConn: func(ctx context.Context, channels []string) (*pool.Conn, error) {
			return c.newConn(ctx)
		},
		closeConn: c.connPool.CloseConn,
	}
	pubsub.init()
	return pubsub
}

// Ping is used to test if a connection is still alive, or to
// measure latency.
func (c *SentinelClient) Ping(ctx context.Context) *StringCmd {
	cmd := NewStringCmd(ctx, "ping")
	_ = c.Process(ctx, cmd)
	return cmd
}

// Subscribe subscribes the client to the specified channels.
// Channels can be omitted to create empty subscription.
func (c *SentinelClient) Subscribe(ctx context.Context, channels ...string) *PubSub {
	pubsub := c.pubSub()
	if len(channels) > 0 {
		_ = pubsub.Subscribe(ctx, channels...)
	}
	return pubsub
}

// PSubscribe subscribes the client to the given patterns.
// Patterns can be omitted to create empty subscription.
func (c *SentinelClient) PSubscribe(ctx context.Context, channels ...string) *PubSub {
	pubsub := c.pubSub()
	if len(channels) > 0 {
		_ = pubsub.PSubscribe(ctx, channels...)
	}
	return pubsub
}

func (c *SentinelClient) GetMasterAddrByName(ctx context.Context, name string) *StringSliceCmd {
	cmd := NewStringSliceCmd(ctx, "sentinel", "get-master-addr-by-name", name)
	_ = c.Process(ctx, cmd)
	return cmd
}

func (c *SentinelClient) Sentinels(ctx context.Context, name string) *MapStringStringSliceCmd {
	cmd := NewMapStringStringSliceCmd(ctx, "sentinel", "sentinels", name)
	_ = c.Process(ctx, cmd)
	return cmd
}

// Failover forces a failover as if the master was not reachable, and without
// asking for agreement to other Sentinels.
func (c *SentinelClient) Failover(ctx context.Context, name string) *StatusCmd {
	cmd := NewStatusCmd(ctx, "sentinel", "failover", name)
	_ = c.Process(ctx, cmd)
	return cmd
}

// Reset resets all the masters with matching name. The pattern argument is a
// glob-style pattern. The reset process clears any previous state in a master
// (including a failover in progress), and removes every replica and sentinel
// already discovered and associated with the master.
func (c *SentinelClient) Reset(ctx context.Context, pattern string) *IntCmd {
	cmd := NewIntCmd(ctx, "sentinel", "reset", pattern)
	_ = c.Process(ctx, cmd)
	return cmd
}

// FlushConfig forces Sentinel to rewrite its configuration on disk, including
// the current Sentinel state.
func (c *SentinelClient) FlushConfig(ctx context.Context) *StatusCmd {
	cmd := NewStatusCmd(ctx, "sentinel", "flushconfig")
	_ = c.Process(ctx, cmd)
	return cmd
}

// Master shows the state and info of the specified master.
func (c *SentinelClient) Master(ctx context.Context, name string) *MapStringStringCmd {
	cmd := NewMapStringStringCmd(ctx, "sentinel", "master", name)
	_ = c.Process(ctx, cmd)
	return cmd
}

// Masters shows a list of monitored masters and their state.
func (c *SentinelClient) Masters(ctx context.Context) *SliceCmd {
	cmd := NewSliceCmd(ctx, "sentinel", "masters")
	_ = c.Process(ctx, cmd)
	return cmd
}

// Replicas shows a list of replicas for the specified master and their state.
func (c *SentinelClient) Replicas(ctx context.Context, name string) *MapStringStringSliceCmd {
	cmd := NewMapStringStringSliceCmd(ctx, "sentinel", "replicas", name)
	_ = c.Process(ctx, cmd)
	return cmd
}

// CkQuorum checks if the current Sentinel configuration is able to reach the
// quorum needed to failover a master, and the majority needed to authorize the
// failover. This command should be used in monitoring systems to check if a
// Sentinel deployment is ok.
func (c *SentinelClient) CkQuorum(ctx context.Context, name string) *StringCmd {
	cmd := NewStringCmd(ctx, "sentinel", "ckquorum", name)
	_ = c.Process(ctx, cmd)
	return cmd
}

// Monitor tells the Sentinel to start monitoring a new master with the specified
// name, ip, port, and quorum.
func (c *SentinelClient) Monitor(ctx context.Context, name, ip, port, quorum string) *StringCmd {
	cmd := NewStringCmd(ctx, "sentinel", "monitor", name, ip, port, quorum)
	_ = c.Process(ctx, cmd)
	return cmd
}

// Set is used in order to change configuration parameters of a specific master.
func (c *SentinelClient) Set(ctx context.Context, name, option, value string) *StringCmd {
	cmd := NewStringCmd(ctx, "sentinel", "set", name, option, value)
	_ = c.Process(ctx, cmd)
	return cmd
}

// Remove is used in order to remove the specified master: the master will no
// longer be monitored, and will totally be removed from the internal state of
// the Sentinel.
func (c *SentinelClient) Remove(ctx context.Context, name string) *StringCmd {
	cmd := NewStringCmd(ctx, "sentinel", "remove", name)
	_ = c.Process(ctx, cmd)
	return cmd
}

//------------------------------------------------------------------------------

type sentinelFailover struct {
	opt *FailoverOptions

	sentinelAddrs []string

	onFailover func(ctx context.Context, addr string)
	onUpdate   func(ctx context.Context)

	mu         sync.RWMutex
	masterAddr string
	sentinel   *SentinelClient
	pubsub     *PubSub
}

func (c *sentinelFailover) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sentinel != nil {
		return c.closeSentinel()
	}
	return nil
}

func (c *sentinelFailover) closeSentinel() error {
	firstErr := c.pubsub.Close()
	c.pubsub = nil

	err := c.sentinel.Close()
	if err != nil && firstErr == nil {
		firstErr = err
	}
	c.sentinel = nil

	return firstErr
}

func (c *sentinelFailover) RandomReplicaAddr(ctx context.Context) (string, error) {
	if c.opt == nil {
		return "", errors.New("opt is nil")
	}

	addresses, err := c.replicaAddrs(ctx, false)
	if err != nil {
		return "", err
	}

	if len(addresses) == 0 && c.opt.UseDisconnectedReplicas {
		addresses, err = c.replicaAddrs(ctx, true)
		if err != nil {
			return "", err
		}
	}

	if len(addresses) == 0 {
		return c.MasterAddr(ctx)
	}
	return addresses[rand.Intn(len(addresses))], nil
}

func (c *sentinelFailover) MasterAddr(ctx context.Context) (string, error) {
	c.mu.RLock()
	sentinel := c.sentinel
	c.mu.RUnlock()

	if sentinel != nil {
		addr, err := c.getMasterAddr(ctx, sentinel)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return "", err
			}
			// Continue on other errors
			internal.Logger.Printf(ctx, "sentinel: GetMasterAddrByName name=%q failed: %s",
				c.opt.MasterName, err)
		} else {
			return addr, nil
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.sentinel != nil {
		addr, err := c.getMasterAddr(ctx, c.sentinel)
		if err != nil {
			_ = c.closeSentinel()
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return "", err
			}
			// Continue on other errors
			internal.Logger.Printf(ctx, "sentinel: GetMasterAddrByName name=%q failed: %s",
				c.opt.MasterName, err)
		} else {
			return addr, nil
		}
	}

	var (
		masterAddr string
		wg         sync.WaitGroup
		once       sync.Once
		errCh      = make(chan error, len(c.sentinelAddrs))
	)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for i, sentinelAddr := range c.sentinelAddrs {
		wg.Add(1)
		go func(i int, addr string) {
			defer wg.Done()
			sentinelCli := NewSentinelClient(c.opt.sentinelOptions(addr))
			addrVal, err := sentinelCli.GetMasterAddrByName(ctx, c.opt.MasterName).Result()
			if err != nil {
				internal.Logger.Printf(ctx, "sentinel: GetMasterAddrByName addr=%s, master=%q failed: %s",
					addr, c.opt.MasterName, err)
				_ = sentinelCli.Close()
				errCh <- err
				return
			}
			once.Do(func() {
				masterAddr = net.JoinHostPort(addrVal[0], addrVal[1])
				// Push working sentinel to the top
				c.sentinelAddrs[0], c.sentinelAddrs[i] = c.sentinelAddrs[i], c.sentinelAddrs[0]
				c.setSentinel(ctx, sentinelCli)
				internal.Logger.Printf(ctx, "sentinel: selected addr=%s masterAddr=%s", addr, masterAddr)
				cancel()
			})
		}(i, sentinelAddr)
	}

	wg.Wait()
	close(errCh)
	if masterAddr != "" {
		return masterAddr, nil
	}
	errs := make([]error, 0, len(errCh))
	for err := range errCh {
		errs = append(errs, err)
	}
	return "", fmt.Errorf("redis: all sentinels specified in configuration are unreachable: %s", joinErrors(errs))
}

func joinErrors(errs []error) string {
	if len(errs) == 1 {
		return errs[0].Error()
	}

	b := []byte(errs[0].Error())
	for _, err := range errs[1:] {
		b = append(b, '\n')
		b = append(b, err.Error()...)
	}
	return util.BytesToString(b)
}

func (c *sentinelFailover) replicaAddrs(ctx context.Context, useDisconnected bool) ([]string, error) {
	c.mu.RLock()
	sentinel := c.sentinel
	c.mu.RUnlock()

	if sentinel != nil {
		addrs, err := c.getReplicaAddrs(ctx, sentinel)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil, err
			}
			// Continue on other errors
			internal.Logger.Printf(ctx, "sentinel: Replicas name=%q failed: %s",
				c.opt.MasterName, err)
		} else if len(addrs) > 0 {
			return addrs, nil
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.sentinel != nil {
		addrs, err := c.getReplicaAddrs(ctx, c.sentinel)
		if err != nil {
			_ = c.closeSentinel()
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil, err
			}
			// Continue on other errors
			internal.Logger.Printf(ctx, "sentinel: Replicas name=%q failed: %s",
				c.opt.MasterName, err)
		} else if len(addrs) > 0 {
			return addrs, nil
		} else {
			// No error and no replicas.
			_ = c.closeSentinel()
		}
	}

	var sentinelReachable bool

	for i, sentinelAddr := range c.sentinelAddrs {
		sentinel := NewSentinelClient(c.opt.sentinelOptions(sentinelAddr))

		replicas, err := sentinel.Replicas(ctx, c.opt.MasterName).Result()
		if err != nil {
			_ = sentinel.Close()
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil, err
			}
			internal.Logger.Printf(ctx, "sentinel: Replicas master=%q failed: %s",
				c.opt.MasterName, err)
			continue
		}
		sentinelReachable = true
		addrs := parseReplicaAddrs(replicas, useDisconnected)
		if len(addrs) == 0 {
			continue
		}
		// Push working sentinel to the top.
		c.sentinelAddrs[0], c.sentinelAddrs[i] = c.sentinelAddrs[i], c.sentinelAddrs[0]
		c.setSentinel(ctx, sentinel)

		return addrs, nil
	}

	if sentinelReachable {
		return []string{}, nil
	}
	return []string{}, errors.New("redis: all sentinels specified in configuration are unreachable")
}

func (c *sentinelFailover) getMasterAddr(ctx context.Context, sentinel *SentinelClient) (string, error) {
	addr, err := sentinel.GetMasterAddrByName(ctx, c.opt.MasterName).Result()
	if err != nil {
		return "", err
	}
	return net.JoinHostPort(addr[0], addr[1]), nil
}

func (c *sentinelFailover) getReplicaAddrs(ctx context.Context, sentinel *SentinelClient) ([]string, error) {
	addrs, err := sentinel.Replicas(ctx, c.opt.MasterName).Result()
	if err != nil {
		internal.Logger.Printf(ctx, "sentinel: Replicas name=%q failed: %s",
			c.opt.MasterName, err)
		return nil, err
	}
	return parseReplicaAddrs(addrs, false), nil
}

func parseReplicaAddrs(addrs []map[string]string, keepDisconnected bool) []string {
	nodes := make([]string, 0, len(addrs))
	for _, node := range addrs {
		isDown := false
		if flags, ok := node["flags"]; ok {
			for _, flag := range strings.Split(flags, ",") {
				switch flag {
				case "s_down", "o_down":
					isDown = true
				case "disconnected":
					if !keepDisconnected {
						isDown = true
					}
				}
			}
		}
		if !isDown && node["ip"] != "" && node["port"] != "" {
			nodes = append(nodes, net.JoinHostPort(node["ip"], node["port"]))
		}
	}

	return nodes
}

func (c *sentinelFailover) trySwitchMaster(ctx context.Context, addr string) {
	c.mu.RLock()
	currentAddr := c.masterAddr //nolint:ifshort
	c.mu.RUnlock()

	if addr == currentAddr {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if addr == c.masterAddr {
		return
	}
	c.masterAddr = addr

	internal.Logger.Printf(ctx, "sentinel: new master=%q addr=%q",
		c.opt.MasterName, addr)
	if c.onFailover != nil {
		c.onFailover(ctx, addr)
	}
}

func (c *sentinelFailover) setSentinel(ctx context.Context, sentinel *SentinelClient) {
	if c.sentinel != nil {
		panic("not reached")
	}
	c.sentinel = sentinel
	c.discoverSentinels(ctx)

	c.pubsub = sentinel.Subscribe(ctx, "+switch-master", "+replica-reconf-done")
	go c.listen(c.pubsub)
}

func (c *sentinelFailover) discoverSentinels(ctx context.Context) {
	sentinels, err := c.sentinel.Sentinels(ctx, c.opt.MasterName).Result()
	if err != nil {
		internal.Logger.Printf(ctx, "sentinel: Sentinels master=%q failed: %s", c.opt.MasterName, err)
		return
	}
	for _, sentinel := range sentinels {
		ip, ok := sentinel["ip"]
		if !ok {
			continue
		}
		port, ok := sentinel["port"]
		if !ok {
			continue
		}
		if ip != "" && port != "" {
			sentinelAddr := net.JoinHostPort(ip, port)
			if !contains(c.sentinelAddrs, sentinelAddr) {
				internal.Logger.Printf(ctx, "sentinel: discovered new sentinel=%q for master=%q",
					sentinelAddr, c.opt.MasterName)
				c.sentinelAddrs = append(c.sentinelAddrs, sentinelAddr)
			}
		}
	}
}

func (c *sentinelFailover) listen(pubsub *PubSub) {
	ctx := context.TODO()

	if c.onUpdate != nil {
		c.onUpdate(ctx)
	}

	ch := pubsub.Channel()
	for msg := range ch {
		if msg.Channel == "+switch-master" {
			parts := strings.Split(msg.Payload, " ")
			if parts[0] != c.opt.MasterName {
				internal.Logger.Printf(pubsub.getContext(), "sentinel: ignore addr for master=%q", parts[0])
				continue
			}
			addr := net.JoinHostPort(parts[3], parts[4])
			c.trySwitchMaster(pubsub.getContext(), addr)
		}

		if c.onUpdate != nil {
			c.onUpdate(ctx)
		}
	}
}

func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

//------------------------------------------------------------------------------

// NewFailoverClusterClient returns a client that supports routing read-only commands
// to a replica node.
func NewFailoverClusterClient(failoverOpt *FailoverOptions) *ClusterClient {
	if failoverOpt == nil {
		panic("redis: NewFailoverClusterClient nil options")
	}

	sentinelAddrs := make([]string, len(failoverOpt.SentinelAddrs))
	copy(sentinelAddrs, failoverOpt.SentinelAddrs)

	failover := &sentinelFailover{
		opt:           failoverOpt,
		sentinelAddrs: sentinelAddrs,
	}

	opt := failoverOpt.clusterOptions()
	if failoverOpt.DB != 0 {
		onConnect := opt.OnConnect

		opt.OnConnect = func(ctx context.Context, cn *Conn) error {
			if err := cn.Select(ctx, failoverOpt.DB).Err(); err != nil {
				return err
			}

			if onConnect != nil {
				return onConnect(ctx, cn)
			}

			return nil
		}
	}

	opt.ClusterSlots = func(ctx context.Context) ([]ClusterSlot, error) {
		masterAddr, err := failover.MasterAddr(ctx)
		if err != nil {
			return nil, err
		}

		nodes := []ClusterNode{{
			Addr: masterAddr,
		}}

		replicaAddrs, err := failover.replicaAddrs(ctx, false)
		if err != nil {
			return nil, err
		}

		for _, replicaAddr := range replicaAddrs {
			nodes = append(nodes, ClusterNode{
				Addr: replicaAddr,
			})
		}

		slots := []ClusterSlot{
			{
				Start: 0,
				End:   16383,
				Nodes: nodes,
			},
		}
		return slots, nil
	}

	c := NewClusterClient(opt)

	failover.mu.Lock()
	failover.onUpdate = func(ctx context.Context) {
		c.ReloadState(ctx)
	}
	failover.mu.Unlock()

	return c
}
