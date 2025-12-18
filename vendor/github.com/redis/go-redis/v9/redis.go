package redis

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9/auth"
	"github.com/redis/go-redis/v9/internal"
	"github.com/redis/go-redis/v9/internal/auth/streaming"
	"github.com/redis/go-redis/v9/internal/hscan"
	"github.com/redis/go-redis/v9/internal/pool"
	"github.com/redis/go-redis/v9/internal/proto"
	"github.com/redis/go-redis/v9/maintnotifications"
	"github.com/redis/go-redis/v9/push"
)

// Scanner internal/hscan.Scanner exposed interface.
type Scanner = hscan.Scanner

// Nil reply returned by Redis when key does not exist.
const Nil = proto.Nil

// SetLogger set custom log
// Use with VoidLogger to disable logging.
func SetLogger(logger internal.Logging) {
	internal.Logger = logger
}

// SetLogLevel sets the log level for the library.
func SetLogLevel(logLevel internal.LogLevelT) {
	internal.LogLevel = logLevel
}

//------------------------------------------------------------------------------

type Hook interface {
	DialHook(next DialHook) DialHook
	ProcessHook(next ProcessHook) ProcessHook
	ProcessPipelineHook(next ProcessPipelineHook) ProcessPipelineHook
}

type (
	DialHook            func(ctx context.Context, network, addr string) (net.Conn, error)
	ProcessHook         func(ctx context.Context, cmd Cmder) error
	ProcessPipelineHook func(ctx context.Context, cmds []Cmder) error
)

type hooksMixin struct {
	hooksMu *sync.RWMutex

	slice   []Hook
	initial hooks
	current hooks
}

func (hs *hooksMixin) initHooks(hooks hooks) {
	hs.hooksMu = new(sync.RWMutex)
	hs.initial = hooks
	hs.chain()
}

type hooks struct {
	dial       DialHook
	process    ProcessHook
	pipeline   ProcessPipelineHook
	txPipeline ProcessPipelineHook
}

func (h *hooks) setDefaults() {
	if h.dial == nil {
		h.dial = func(ctx context.Context, network, addr string) (net.Conn, error) { return nil, nil }
	}
	if h.process == nil {
		h.process = func(ctx context.Context, cmd Cmder) error { return nil }
	}
	if h.pipeline == nil {
		h.pipeline = func(ctx context.Context, cmds []Cmder) error { return nil }
	}
	if h.txPipeline == nil {
		h.txPipeline = func(ctx context.Context, cmds []Cmder) error { return nil }
	}
}

// AddHook is to add a hook to the queue.
// Hook is a function executed during network connection, command execution, and pipeline,
// it is a first-in-first-out stack queue (FIFO).
// You need to execute the next hook in each hook, unless you want to terminate the execution of the command.
// For example, you added hook-1, hook-2:
//
//	client.AddHook(hook-1, hook-2)
//
// hook-1:
//
//	func (Hook1) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
//	 	return func(ctx context.Context, cmd Cmder) error {
//		 	print("hook-1 start")
//		 	next(ctx, cmd)
//		 	print("hook-1 end")
//		 	return nil
//	 	}
//	}
//
// hook-2:
//
//	func (Hook2) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
//		return func(ctx context.Context, cmd redis.Cmder) error {
//			print("hook-2 start")
//			next(ctx, cmd)
//			print("hook-2 end")
//			return nil
//		}
//	}
//
// The execution sequence is:
//
//	hook-1 start -> hook-2 start -> exec redis cmd -> hook-2 end -> hook-1 end
//
// Please note: "next(ctx, cmd)" is very important, it will call the next hook,
// if "next(ctx, cmd)" is not executed, the redis command will not be executed.
func (hs *hooksMixin) AddHook(hook Hook) {
	hs.slice = append(hs.slice, hook)
	hs.chain()
}

func (hs *hooksMixin) chain() {
	hs.initial.setDefaults()

	hs.hooksMu.Lock()
	defer hs.hooksMu.Unlock()

	hs.current.dial = hs.initial.dial
	hs.current.process = hs.initial.process
	hs.current.pipeline = hs.initial.pipeline
	hs.current.txPipeline = hs.initial.txPipeline

	for i := len(hs.slice) - 1; i >= 0; i-- {
		if wrapped := hs.slice[i].DialHook(hs.current.dial); wrapped != nil {
			hs.current.dial = wrapped
		}
		if wrapped := hs.slice[i].ProcessHook(hs.current.process); wrapped != nil {
			hs.current.process = wrapped
		}
		if wrapped := hs.slice[i].ProcessPipelineHook(hs.current.pipeline); wrapped != nil {
			hs.current.pipeline = wrapped
		}
		if wrapped := hs.slice[i].ProcessPipelineHook(hs.current.txPipeline); wrapped != nil {
			hs.current.txPipeline = wrapped
		}
	}
}

func (hs *hooksMixin) clone() hooksMixin {
	hs.hooksMu.Lock()
	defer hs.hooksMu.Unlock()

	clone := *hs
	l := len(clone.slice)
	clone.slice = clone.slice[:l:l]
	clone.hooksMu = new(sync.RWMutex)
	return clone
}

func (hs *hooksMixin) withProcessHook(ctx context.Context, cmd Cmder, hook ProcessHook) error {
	for i := len(hs.slice) - 1; i >= 0; i-- {
		if wrapped := hs.slice[i].ProcessHook(hook); wrapped != nil {
			hook = wrapped
		}
	}
	return hook(ctx, cmd)
}

func (hs *hooksMixin) withProcessPipelineHook(
	ctx context.Context, cmds []Cmder, hook ProcessPipelineHook,
) error {
	for i := len(hs.slice) - 1; i >= 0; i-- {
		if wrapped := hs.slice[i].ProcessPipelineHook(hook); wrapped != nil {
			hook = wrapped
		}
	}
	return hook(ctx, cmds)
}

func (hs *hooksMixin) dialHook(ctx context.Context, network, addr string) (net.Conn, error) {
	// Access to hs.current is guarded by a read-only lock since it may be mutated by AddHook(...)
	// while this dialer is concurrently accessed by the background connection pool population
	// routine when MinIdleConns > 0.
	hs.hooksMu.RLock()
	current := hs.current
	hs.hooksMu.RUnlock()

	return current.dial(ctx, network, addr)
}

func (hs *hooksMixin) processHook(ctx context.Context, cmd Cmder) error {
	return hs.current.process(ctx, cmd)
}

func (hs *hooksMixin) processPipelineHook(ctx context.Context, cmds []Cmder) error {
	return hs.current.pipeline(ctx, cmds)
}

func (hs *hooksMixin) processTxPipelineHook(ctx context.Context, cmds []Cmder) error {
	return hs.current.txPipeline(ctx, cmds)
}

//------------------------------------------------------------------------------

type baseClient struct {
	opt        *Options
	optLock    sync.RWMutex
	connPool   pool.Pooler
	pubSubPool *pool.PubSubPool
	hooksMixin

	onClose func() error // hook called when client is closed

	// Push notification processing
	pushProcessor push.NotificationProcessor

	// Maintenance notifications manager
	maintNotificationsManager     *maintnotifications.Manager
	maintNotificationsManagerLock sync.RWMutex

	// streamingCredentialsManager is used to manage streaming credentials
	streamingCredentialsManager *streaming.Manager
}

func (c *baseClient) clone() *baseClient {
	c.maintNotificationsManagerLock.RLock()
	maintNotificationsManager := c.maintNotificationsManager
	c.maintNotificationsManagerLock.RUnlock()

	clone := &baseClient{
		opt:                         c.opt,
		connPool:                    c.connPool,
		onClose:                     c.onClose,
		pushProcessor:               c.pushProcessor,
		maintNotificationsManager:   maintNotificationsManager,
		streamingCredentialsManager: c.streamingCredentialsManager,
	}
	return clone
}

func (c *baseClient) withTimeout(timeout time.Duration) *baseClient {
	opt := c.opt.clone()
	opt.ReadTimeout = timeout
	opt.WriteTimeout = timeout

	clone := c.clone()
	clone.opt = opt

	return clone
}

func (c *baseClient) String() string {
	return fmt.Sprintf("Redis<%s db:%d>", c.getAddr(), c.opt.DB)
}

func (c *baseClient) getConn(ctx context.Context) (*pool.Conn, error) {
	if c.opt.Limiter != nil {
		err := c.opt.Limiter.Allow()
		if err != nil {
			return nil, err
		}
	}

	cn, err := c._getConn(ctx)
	if err != nil {
		if c.opt.Limiter != nil {
			c.opt.Limiter.ReportResult(err)
		}
		return nil, err
	}

	return cn, nil
}

func (c *baseClient) _getConn(ctx context.Context) (*pool.Conn, error) {
	cn, err := c.connPool.Get(ctx)
	if err != nil {
		return nil, err
	}

	if cn.IsInited() {
		return cn, nil
	}

	if err := c.initConn(ctx, cn); err != nil {
		c.connPool.Remove(ctx, cn, err)
		if err := errors.Unwrap(err); err != nil {
			return nil, err
		}
		return nil, err
	}

	// initConn will transition to IDLE state, so we need to acquire it
	// before returning it to the user.
	if !cn.TryAcquire() {
		return nil, fmt.Errorf("redis: connection is not usable")
	}

	return cn, nil
}

func (c *baseClient) reAuthConnection() func(poolCn *pool.Conn, credentials auth.Credentials) error {
	return func(poolCn *pool.Conn, credentials auth.Credentials) error {
		var err error
		username, password := credentials.BasicAuth()

		// Use background context - timeout is handled by ReadTimeout in WithReader/WithWriter
		ctx := context.Background()

		connPool := pool.NewSingleConnPool(c.connPool, poolCn)

		// Pass hooks so that reauth commands are recorded/traced
		cn := newConn(c.opt, connPool, &c.hooksMixin)

		if username != "" {
			err = cn.AuthACL(ctx, username, password).Err()
		} else {
			err = cn.Auth(ctx, password).Err()
		}

		return err
	}
}
func (c *baseClient) onAuthenticationErr() func(poolCn *pool.Conn, err error) {
	return func(poolCn *pool.Conn, err error) {
		if err != nil {
			if isBadConn(err, false, c.opt.Addr) {
				// Close the connection to force a reconnection.
				err := c.connPool.CloseConn(poolCn)
				if err != nil {
					internal.Logger.Printf(context.Background(), "redis: failed to close connection: %v", err)
					// try to close the network connection directly
					// so that no resource is leaked
					err := poolCn.Close()
					if err != nil {
						internal.Logger.Printf(context.Background(), "redis: failed to close network connection: %v", err)
					}
				}
			}
			internal.Logger.Printf(context.Background(), "redis: re-authentication failed: %v", err)
		}
	}
}

func (c *baseClient) wrappedOnClose(newOnClose func() error) func() error {
	onClose := c.onClose
	return func() error {
		var firstErr error
		err := newOnClose()
		// Even if we have an error we would like to execute the onClose hook
		// if it exists. We will return the first error that occurred.
		// This is to keep error handling consistent with the rest of the code.
		if err != nil {
			firstErr = err
		}
		if onClose != nil {
			err = onClose()
			if err != nil && firstErr == nil {
				firstErr = err
			}
		}
		return firstErr
	}
}

func (c *baseClient) initConn(ctx context.Context, cn *pool.Conn) error {
	// This function is called in two scenarios:
	// 1. First-time init: Connection is in CREATED state (from pool.Get())
	//    - We need to transition CREATED → INITIALIZING and do the initialization
	//    - If another goroutine is already initializing, we WAIT for it to finish
	// 2. Re-initialization: Connection is in INITIALIZING state (from SetNetConnAndInitConn())
	//    - We're already in INITIALIZING, so just proceed with initialization

	currentState := cn.GetStateMachine().GetState()

	// Fast path: Check if already initialized (IDLE or IN_USE)
	if currentState == pool.StateIdle || currentState == pool.StateInUse {
		return nil
	}

	// If in CREATED state, try to transition to INITIALIZING
	if currentState == pool.StateCreated {
		finalState, err := cn.GetStateMachine().TryTransition([]pool.ConnState{pool.StateCreated}, pool.StateInitializing)
		if err != nil {
			// Another goroutine is initializing or connection is in unexpected state
			// Check what state we're in now
			if finalState == pool.StateIdle || finalState == pool.StateInUse {
				// Already initialized by another goroutine
				return nil
			}

			if finalState == pool.StateInitializing {
				// Another goroutine is initializing - WAIT for it to complete
				// Use a context with timeout = min(remaining command timeout, DialTimeout)
				// This prevents waiting too long while respecting the caller's deadline
				var waitCtx context.Context
				var cancel context.CancelFunc
				dialTimeout := c.opt.DialTimeout

				if cmdDeadline, hasCmdDeadline := ctx.Deadline(); hasCmdDeadline {
					// Calculate remaining time until command deadline
					remainingTime := time.Until(cmdDeadline)
					// Use the minimum of remaining time and DialTimeout
					if remainingTime < dialTimeout {
						// Command deadline is sooner, use it
						waitCtx = ctx
					} else {
						// DialTimeout is shorter, cap the wait at DialTimeout
						waitCtx, cancel = context.WithTimeout(ctx, dialTimeout)
					}
				} else {
					// No command deadline, use DialTimeout to prevent waiting indefinitely
					waitCtx, cancel = context.WithTimeout(ctx, dialTimeout)
				}
				if cancel != nil {
					defer cancel()
				}

				finalState, err := cn.GetStateMachine().AwaitAndTransition(
					waitCtx,
					[]pool.ConnState{pool.StateIdle, pool.StateInUse},
					pool.StateIdle, // Target is IDLE (but we're already there, so this is a no-op)
				)
				if err != nil {
					return err
				}
				// Verify we're now initialized
				if finalState == pool.StateIdle || finalState == pool.StateInUse {
					return nil
				}
				// Unexpected state after waiting
				return fmt.Errorf("connection in unexpected state after initialization: %s", finalState)
			}

			// Unexpected state (CLOSED, UNUSABLE, etc.)
			return err
		}
	}

	// At this point, we're in INITIALIZING state and we own the initialization
	// If we fail, we must transition to CLOSED
	var initErr error
	connPool := pool.NewSingleConnPool(c.connPool, cn)
	conn := newConn(c.opt, connPool, &c.hooksMixin)

	username, password := "", ""
	if c.opt.StreamingCredentialsProvider != nil {
		credListener, initErr := c.streamingCredentialsManager.Listener(
			cn,
			c.reAuthConnection(),
			c.onAuthenticationErr(),
		)
		if initErr != nil {
			cn.GetStateMachine().Transition(pool.StateClosed)
			return fmt.Errorf("failed to create credentials listener: %w", initErr)
		}

		credentials, unsubscribeFromCredentialsProvider, initErr := c.opt.StreamingCredentialsProvider.
			Subscribe(credListener)
		if initErr != nil {
			cn.GetStateMachine().Transition(pool.StateClosed)
			return fmt.Errorf("failed to subscribe to streaming credentials: %w", initErr)
		}

		c.onClose = c.wrappedOnClose(unsubscribeFromCredentialsProvider)
		cn.SetOnClose(unsubscribeFromCredentialsProvider)

		username, password = credentials.BasicAuth()
	} else if c.opt.CredentialsProviderContext != nil {
		username, password, initErr = c.opt.CredentialsProviderContext(ctx)
		if initErr != nil {
			cn.GetStateMachine().Transition(pool.StateClosed)
			return fmt.Errorf("failed to get credentials from context provider: %w", initErr)
		}
	} else if c.opt.CredentialsProvider != nil {
		username, password = c.opt.CredentialsProvider()
	} else if c.opt.Username != "" || c.opt.Password != "" {
		username, password = c.opt.Username, c.opt.Password
	}

	// for redis-server versions that do not support the HELLO command,
	// RESP2 will continue to be used.
	if initErr = conn.Hello(ctx, c.opt.Protocol, username, password, c.opt.ClientName).Err(); initErr == nil {
		// Authentication successful with HELLO command
	} else if !isRedisError(initErr) {
		// When the server responds with the RESP protocol and the result is not a normal
		// execution result of the HELLO command, we consider it to be an indication that
		// the server does not support the HELLO command.
		// The server may be a redis-server that does not support the HELLO command,
		// or it could be DragonflyDB or a third-party redis-proxy. They all respond
		// with different error string results for unsupported commands, making it
		// difficult to rely on error strings to determine all results.
		cn.GetStateMachine().Transition(pool.StateClosed)
		return initErr
	} else if password != "" {
		// Try legacy AUTH command if HELLO failed
		if username != "" {
			initErr = conn.AuthACL(ctx, username, password).Err()
		} else {
			initErr = conn.Auth(ctx, password).Err()
		}
		if initErr != nil {
			cn.GetStateMachine().Transition(pool.StateClosed)
			return fmt.Errorf("failed to authenticate: %w", initErr)
		}
	}

	_, initErr = conn.Pipelined(ctx, func(pipe Pipeliner) error {
		if c.opt.DB > 0 {
			pipe.Select(ctx, c.opt.DB)
		}

		if c.opt.readOnly {
			pipe.ReadOnly(ctx)
		}

		if c.opt.ClientName != "" {
			pipe.ClientSetName(ctx, c.opt.ClientName)
		}

		return nil
	})
	if initErr != nil {
		cn.GetStateMachine().Transition(pool.StateClosed)
		return fmt.Errorf("failed to initialize connection options: %w", initErr)
	}

	// Enable maintnotifications if maintnotifications are configured
	c.optLock.RLock()
	maintNotifEnabled := c.opt.MaintNotificationsConfig != nil && c.opt.MaintNotificationsConfig.Mode != maintnotifications.ModeDisabled
	protocol := c.opt.Protocol
	endpointType := c.opt.MaintNotificationsConfig.EndpointType
	c.optLock.RUnlock()
	var maintNotifHandshakeErr error
	if maintNotifEnabled && protocol == 3 {
		maintNotifHandshakeErr = conn.ClientMaintNotifications(
			ctx,
			true,
			endpointType.String(),
		).Err()
		if maintNotifHandshakeErr != nil {
			if !isRedisError(maintNotifHandshakeErr) {
				// if not redis error, fail the connection
				cn.GetStateMachine().Transition(pool.StateClosed)
				return maintNotifHandshakeErr
			}
			c.optLock.Lock()
			// handshake failed - check and modify config atomically
			switch c.opt.MaintNotificationsConfig.Mode {
			case maintnotifications.ModeEnabled:
				// enabled mode, fail the connection
				c.optLock.Unlock()
				cn.GetStateMachine().Transition(pool.StateClosed)
				return fmt.Errorf("failed to enable maintnotifications: %w", maintNotifHandshakeErr)
			default: // will handle auto and any other
				// Disabling logging here as it's too noisy.
				// TODO: Enable when we have a better logging solution for log levels
				// internal.Logger.Printf(ctx, "auto mode fallback: maintnotifications disabled due to handshake error: %v", maintNotifHandshakeErr)
				c.opt.MaintNotificationsConfig.Mode = maintnotifications.ModeDisabled
				c.optLock.Unlock()
				// auto mode, disable maintnotifications and continue
				if initErr := c.disableMaintNotificationsUpgrades(); initErr != nil {
					// Log error but continue - auto mode should be resilient
					internal.Logger.Printf(ctx, "failed to disable maintnotifications in auto mode: %v", initErr)
				}
			}
		} else {
			// handshake was executed successfully
			// to make sure that the handshake will be executed on other connections as well if it was successfully
			// executed on this connection, we will force the handshake to be executed on all connections
			c.optLock.Lock()
			c.opt.MaintNotificationsConfig.Mode = maintnotifications.ModeEnabled
			c.optLock.Unlock()
		}
	}

	if !c.opt.DisableIdentity && !c.opt.DisableIndentity {
		libName := ""
		libVer := Version()
		if c.opt.IdentitySuffix != "" {
			libName = c.opt.IdentitySuffix
		}
		p := conn.Pipeline()
		p.ClientSetInfo(ctx, WithLibraryName(libName))
		p.ClientSetInfo(ctx, WithLibraryVersion(libVer))
		// Handle network errors (e.g. timeouts) in CLIENT SETINFO to avoid
		// out of order responses later on.
		if _, initErr = p.Exec(ctx); initErr != nil && !isRedisError(initErr) {
			cn.GetStateMachine().Transition(pool.StateClosed)
			return initErr
		}
	}

	// Set the connection initialization function for potential reconnections
	// This must be set before transitioning to IDLE so that handoff/reauth can use it
	cn.SetInitConnFunc(c.createInitConnFunc())

	// Initialization succeeded - transition to IDLE state
	// This marks the connection as initialized and ready for use
	// NOTE: The connection is still owned by the calling goroutine at this point
	// and won't be available to other goroutines until it's Put() back into the pool
	cn.GetStateMachine().Transition(pool.StateIdle)

	// Call OnConnect hook if configured
	// The connection is in IDLE state but still owned by this goroutine
	// If OnConnect needs to send commands, it can use the connection safely
	if c.opt.OnConnect != nil {
		if initErr = c.opt.OnConnect(ctx, conn); initErr != nil {
			// OnConnect failed - transition to closed
			cn.GetStateMachine().Transition(pool.StateClosed)
			return initErr
		}
	}

	return nil
}

func (c *baseClient) releaseConn(ctx context.Context, cn *pool.Conn, err error) {
	if c.opt.Limiter != nil {
		c.opt.Limiter.ReportResult(err)
	}

	if isBadConn(err, false, c.opt.Addr) {
		c.connPool.Remove(ctx, cn, err)
	} else {
		// process any pending push notifications before returning the connection to the pool
		if err := c.processPushNotifications(ctx, cn); err != nil {
			internal.Logger.Printf(ctx, "push: error processing pending notifications before releasing connection: %v", err)
		}
		c.connPool.Put(ctx, cn)
	}
}

func (c *baseClient) withConn(
	ctx context.Context, fn func(context.Context, *pool.Conn) error,
) error {
	cn, err := c.getConn(ctx)
	if err != nil {
		return err
	}

	var fnErr error
	defer func() {
		c.releaseConn(ctx, cn, fnErr)
	}()

	fnErr = fn(ctx, cn)

	return fnErr
}

func (c *baseClient) dial(ctx context.Context, network, addr string) (net.Conn, error) {
	return c.opt.Dialer(ctx, network, addr)
}

func (c *baseClient) process(ctx context.Context, cmd Cmder) error {
	var lastErr error
	for attempt := 0; attempt <= c.opt.MaxRetries; attempt++ {
		attempt := attempt

		retry, err := c._process(ctx, cmd, attempt)
		if err == nil || !retry {
			return err
		}

		lastErr = err
	}
	return lastErr
}

func (c *baseClient) assertUnstableCommand(cmd Cmder) (bool, error) {
	switch cmd.(type) {
	case *AggregateCmd, *FTInfoCmd, *FTSpellCheckCmd, *FTSearchCmd, *FTSynDumpCmd:
		if c.opt.UnstableResp3 {
			return true, nil
		} else {
			return false, fmt.Errorf("RESP3 responses for this command are disabled because they may still change. Please set the flag UnstableResp3. See the README and the release notes for guidance")
		}
	default:
		return false, nil
	}
}

func (c *baseClient) _process(ctx context.Context, cmd Cmder, attempt int) (bool, error) {
	if attempt > 0 {
		if err := internal.Sleep(ctx, c.retryBackoff(attempt)); err != nil {
			return false, err
		}
	}

	retryTimeout := uint32(0)
	if err := c.withConn(ctx, func(ctx context.Context, cn *pool.Conn) error {
		// Process any pending push notifications before executing the command
		if err := c.processPushNotifications(ctx, cn); err != nil {
			internal.Logger.Printf(ctx, "push: error processing pending notifications before command: %v", err)
		}

		if err := cn.WithWriter(c.context(ctx), c.opt.WriteTimeout, func(wr *proto.Writer) error {
			return writeCmd(wr, cmd)
		}); err != nil {
			atomic.StoreUint32(&retryTimeout, 1)
			return err
		}
		readReplyFunc := cmd.readReply
		// Apply unstable RESP3 search module.
		if c.opt.Protocol != 2 {
			useRawReply, err := c.assertUnstableCommand(cmd)
			if err != nil {
				return err
			}
			if useRawReply {
				readReplyFunc = cmd.readRawReply
			}
		}
		if err := cn.WithReader(c.context(ctx), c.cmdTimeout(cmd), func(rd *proto.Reader) error {
			// To be sure there are no buffered push notifications, we process them before reading the reply
			if err := c.processPendingPushNotificationWithReader(ctx, cn, rd); err != nil {
				internal.Logger.Printf(ctx, "push: error processing pending notifications before reading reply: %v", err)
			}
			return readReplyFunc(rd)
		}); err != nil {
			if cmd.readTimeout() == nil {
				atomic.StoreUint32(&retryTimeout, 1)
			} else {
				atomic.StoreUint32(&retryTimeout, 0)
			}
			return err
		}

		return nil
	}); err != nil {
		retry := shouldRetry(err, atomic.LoadUint32(&retryTimeout) == 1)
		return retry, err
	}

	return false, nil
}

func (c *baseClient) retryBackoff(attempt int) time.Duration {
	return internal.RetryBackoff(attempt, c.opt.MinRetryBackoff, c.opt.MaxRetryBackoff)
}

func (c *baseClient) cmdTimeout(cmd Cmder) time.Duration {
	if timeout := cmd.readTimeout(); timeout != nil {
		t := *timeout
		if t == 0 {
			return 0
		}
		return t + 10*time.Second
	}
	return c.opt.ReadTimeout
}

// context returns the context for the current connection.
// If the context timeout is enabled, it returns the original context.
// Otherwise, it returns a new background context.
func (c *baseClient) context(ctx context.Context) context.Context {
	if c.opt.ContextTimeoutEnabled {
		return ctx
	}
	return context.Background()
}

// createInitConnFunc creates a connection initialization function that can be used for reconnections.
func (c *baseClient) createInitConnFunc() func(context.Context, *pool.Conn) error {
	return func(ctx context.Context, cn *pool.Conn) error {
		return c.initConn(ctx, cn)
	}
}

// enableMaintNotificationsUpgrades initializes the maintnotifications upgrade manager and pool hook.
// This function is called during client initialization.
// will register push notification handlers for all maintenance upgrade events.
// will start background workers for handoff processing in the pool hook.
func (c *baseClient) enableMaintNotificationsUpgrades() error {
	// Create client adapter
	clientAdapterInstance := newClientAdapter(c)

	// Create maintnotifications manager directly
	manager, err := maintnotifications.NewManager(clientAdapterInstance, c.connPool, c.opt.MaintNotificationsConfig)
	if err != nil {
		return err
	}
	// Set the manager reference and initialize pool hook
	c.maintNotificationsManagerLock.Lock()
	c.maintNotificationsManager = manager
	c.maintNotificationsManagerLock.Unlock()

	// Initialize pool hook (safe to call without lock since manager is now set)
	manager.InitPoolHook(c.dialHook)
	return nil
}

func (c *baseClient) disableMaintNotificationsUpgrades() error {
	c.maintNotificationsManagerLock.Lock()
	defer c.maintNotificationsManagerLock.Unlock()

	// Close the maintnotifications manager
	if c.maintNotificationsManager != nil {
		// Closing the manager will also shutdown the pool hook
		// and remove it from the pool
		c.maintNotificationsManager.Close()
		c.maintNotificationsManager = nil
	}
	return nil
}

// Close closes the client, releasing any open resources.
//
// It is rare to Close a Client, as the Client is meant to be
// long-lived and shared between many goroutines.
func (c *baseClient) Close() error {
	var firstErr error

	// Close maintnotifications manager first
	if err := c.disableMaintNotificationsUpgrades(); err != nil {
		firstErr = err
	}

	if c.onClose != nil {
		if err := c.onClose(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if c.connPool != nil {
		if err := c.connPool.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if c.pubSubPool != nil {
		if err := c.pubSubPool.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (c *baseClient) getAddr() string {
	return c.opt.Addr
}

func (c *baseClient) processPipeline(ctx context.Context, cmds []Cmder) error {
	if err := c.generalProcessPipeline(ctx, cmds, c.pipelineProcessCmds); err != nil {
		return err
	}
	return cmdsFirstErr(cmds)
}

func (c *baseClient) processTxPipeline(ctx context.Context, cmds []Cmder) error {
	if err := c.generalProcessPipeline(ctx, cmds, c.txPipelineProcessCmds); err != nil {
		return err
	}
	return cmdsFirstErr(cmds)
}

type pipelineProcessor func(context.Context, *pool.Conn, []Cmder) (bool, error)

func (c *baseClient) generalProcessPipeline(
	ctx context.Context, cmds []Cmder, p pipelineProcessor,
) error {
	var lastErr error
	for attempt := 0; attempt <= c.opt.MaxRetries; attempt++ {
		if attempt > 0 {
			if err := internal.Sleep(ctx, c.retryBackoff(attempt)); err != nil {
				setCmdsErr(cmds, err)
				return err
			}
		}

		// Enable retries by default to retry dial errors returned by withConn.
		canRetry := true
		lastErr = c.withConn(ctx, func(ctx context.Context, cn *pool.Conn) error {
			// Process any pending push notifications before executing the pipeline
			if err := c.processPushNotifications(ctx, cn); err != nil {
				internal.Logger.Printf(ctx, "push: error processing pending notifications before processing pipeline: %v", err)
			}
			var err error
			canRetry, err = p(ctx, cn, cmds)
			return err
		})
		if lastErr == nil || !canRetry || !shouldRetry(lastErr, true) {
			// The error should be set here only when failing to obtain the conn.
			if !isRedisError(lastErr) {
				setCmdsErr(cmds, lastErr)
			}
			return lastErr
		}
	}
	return lastErr
}

func (c *baseClient) pipelineProcessCmds(
	ctx context.Context, cn *pool.Conn, cmds []Cmder,
) (bool, error) {
	// Process any pending push notifications before executing the pipeline
	if err := c.processPushNotifications(ctx, cn); err != nil {
		internal.Logger.Printf(ctx, "push: error processing pending notifications before writing pipeline: %v", err)
	}

	if err := cn.WithWriter(c.context(ctx), c.opt.WriteTimeout, func(wr *proto.Writer) error {
		return writeCmds(wr, cmds)
	}); err != nil {
		setCmdsErr(cmds, err)
		return true, err
	}

	if err := cn.WithReader(c.context(ctx), c.opt.ReadTimeout, func(rd *proto.Reader) error {
		// read all replies
		return c.pipelineReadCmds(ctx, cn, rd, cmds)
	}); err != nil {
		return true, err
	}

	return false, nil
}

func (c *baseClient) pipelineReadCmds(ctx context.Context, cn *pool.Conn, rd *proto.Reader, cmds []Cmder) error {
	for i, cmd := range cmds {
		// To be sure there are no buffered push notifications, we process them before reading the reply
		if err := c.processPendingPushNotificationWithReader(ctx, cn, rd); err != nil {
			internal.Logger.Printf(ctx, "push: error processing pending notifications before reading reply: %v", err)
		}
		err := cmd.readReply(rd)
		cmd.SetErr(err)
		if err != nil && !isRedisError(err) {
			setCmdsErr(cmds[i+1:], err)
			return err
		}
	}
	// Retry errors like "LOADING redis is loading the dataset in memory".
	return cmds[0].Err()
}

func (c *baseClient) txPipelineProcessCmds(
	ctx context.Context, cn *pool.Conn, cmds []Cmder,
) (bool, error) {
	// Process any pending push notifications before executing the transaction pipeline
	if err := c.processPushNotifications(ctx, cn); err != nil {
		internal.Logger.Printf(ctx, "push: error processing pending notifications before transaction: %v", err)
	}

	if err := cn.WithWriter(c.context(ctx), c.opt.WriteTimeout, func(wr *proto.Writer) error {
		return writeCmds(wr, cmds)
	}); err != nil {
		setCmdsErr(cmds, err)
		return true, err
	}

	if err := cn.WithReader(c.context(ctx), c.opt.ReadTimeout, func(rd *proto.Reader) error {
		statusCmd := cmds[0].(*StatusCmd)
		// Trim multi and exec.
		trimmedCmds := cmds[1 : len(cmds)-1]

		if err := c.txPipelineReadQueued(ctx, cn, rd, statusCmd, trimmedCmds); err != nil {
			setCmdsErr(cmds, err)
			return err
		}

		// Read replies.
		return c.pipelineReadCmds(ctx, cn, rd, trimmedCmds)
	}); err != nil {
		return false, err
	}

	return false, nil
}

// txPipelineReadQueued reads queued replies from the Redis server.
// It returns an error if the server returns an error or if the number of replies does not match the number of commands.
func (c *baseClient) txPipelineReadQueued(ctx context.Context, cn *pool.Conn, rd *proto.Reader, statusCmd *StatusCmd, cmds []Cmder) error {
	// To be sure there are no buffered push notifications, we process them before reading the reply
	if err := c.processPendingPushNotificationWithReader(ctx, cn, rd); err != nil {
		internal.Logger.Printf(ctx, "push: error processing pending notifications before reading reply: %v", err)
	}
	// Parse +OK.
	if err := statusCmd.readReply(rd); err != nil {
		return err
	}

	// Parse +QUEUED.
	for _, cmd := range cmds {
		// To be sure there are no buffered push notifications, we process them before reading the reply
		if err := c.processPendingPushNotificationWithReader(ctx, cn, rd); err != nil {
			internal.Logger.Printf(ctx, "push: error processing pending notifications before reading reply: %v", err)
		}
		if err := statusCmd.readReply(rd); err != nil {
			cmd.SetErr(err)
			if !isRedisError(err) {
				return err
			}
		}
	}

	// To be sure there are no buffered push notifications, we process them before reading the reply
	if err := c.processPendingPushNotificationWithReader(ctx, cn, rd); err != nil {
		internal.Logger.Printf(ctx, "push: error processing pending notifications before reading reply: %v", err)
	}
	// Parse number of replies.
	line, err := rd.ReadLine()
	if err != nil {
		if err == Nil {
			err = TxFailedErr
		}
		return err
	}

	if line[0] != proto.RespArray {
		return fmt.Errorf("redis: expected '*', but got line %q", line)
	}

	return nil
}

//------------------------------------------------------------------------------

// Client is a Redis client representing a pool of zero or more underlying connections.
// It's safe for concurrent use by multiple goroutines.
//
// Client creates and frees connections automatically; it also maintains a free pool
// of idle connections. You can control the pool size with Config.PoolSize option.
type Client struct {
	*baseClient
	cmdable
}

// NewClient returns a client to the Redis Server specified by Options.
func NewClient(opt *Options) *Client {
	if opt == nil {
		panic("redis: NewClient nil options")
	}
	// clone to not share options with the caller
	opt = opt.clone()
	opt.init()

	// Push notifications are always enabled for RESP3 (cannot be disabled)

	c := Client{
		baseClient: &baseClient{
			opt: opt,
		},
	}
	c.init()

	// Initialize push notification processor using shared helper
	// Use void processor for RESP2 connections (push notifications not available)
	c.pushProcessor = initializePushProcessor(opt)
	// set opt push processor for child clients
	c.opt.PushNotificationProcessor = c.pushProcessor

	// Create connection pools
	var err error
	c.connPool, err = newConnPool(opt, c.dialHook)
	if err != nil {
		panic(fmt.Errorf("redis: failed to create connection pool: %w", err))
	}
	c.pubSubPool, err = newPubSubPool(opt, c.dialHook)
	if err != nil {
		panic(fmt.Errorf("redis: failed to create pubsub pool: %w", err))
	}

	if opt.StreamingCredentialsProvider != nil {
		c.streamingCredentialsManager = streaming.NewManager(c.connPool, c.opt.PoolTimeout)
		c.connPool.AddPoolHook(c.streamingCredentialsManager.PoolHook())
	}

	// Initialize maintnotifications first if enabled and protocol is RESP3
	if opt.MaintNotificationsConfig != nil && opt.MaintNotificationsConfig.Mode != maintnotifications.ModeDisabled && opt.Protocol == 3 {
		err := c.enableMaintNotificationsUpgrades()
		if err != nil {
			internal.Logger.Printf(context.Background(), "failed to initialize maintnotifications: %v", err)
			if opt.MaintNotificationsConfig.Mode == maintnotifications.ModeEnabled {
				/*
					Design decision: panic here to fail fast if maintnotifications cannot be enabled when explicitly requested.
					We choose to panic instead of returning an error to avoid breaking the existing client API, which does not expect
					an error from NewClient. This ensures that misconfiguration or critical initialization failures are surfaced
					immediately, rather than allowing the client to continue in a partially initialized or inconsistent state.
					Clients relying on maintnotifications should be aware that initialization errors will cause a panic, and should
					handle this accordingly (e.g., via recover or by validating configuration before calling NewClient).
					This approach is only used when MaintNotificationsConfig.Mode is MaintNotificationsEnabled, indicating that maintnotifications
					upgrades are required for correct operation. In other modes, initialization failures are logged but do not panic.
				*/
				panic(fmt.Errorf("failed to enable maintnotifications: %w", err))
			}
		}
	}

	return &c
}

func (c *Client) init() {
	c.cmdable = c.Process
	c.initHooks(hooks{
		dial:       c.baseClient.dial,
		process:    c.baseClient.process,
		pipeline:   c.baseClient.processPipeline,
		txPipeline: c.baseClient.processTxPipeline,
	})
}

func (c *Client) WithTimeout(timeout time.Duration) *Client {
	clone := *c
	clone.baseClient = c.baseClient.withTimeout(timeout)
	clone.init()
	return &clone
}

func (c *Client) Conn() *Conn {
	return newConn(c.opt, pool.NewStickyConnPool(c.connPool), &c.hooksMixin)
}

func (c *Client) Process(ctx context.Context, cmd Cmder) error {
	err := c.processHook(ctx, cmd)
	cmd.SetErr(err)
	return err
}

// Options returns read-only Options that were used to create the client.
func (c *Client) Options() *Options {
	return c.opt
}

// GetMaintNotificationsManager returns the maintnotifications manager instance for monitoring and control.
// Returns nil if maintnotifications are not enabled.
func (c *Client) GetMaintNotificationsManager() *maintnotifications.Manager {
	c.maintNotificationsManagerLock.RLock()
	defer c.maintNotificationsManagerLock.RUnlock()
	return c.maintNotificationsManager
}

// initializePushProcessor initializes the push notification processor for any client type.
// This is a shared helper to avoid duplication across NewClient, NewFailoverClient, and NewSentinelClient.
func initializePushProcessor(opt *Options) push.NotificationProcessor {
	// Always use custom processor if provided
	if opt.PushNotificationProcessor != nil {
		return opt.PushNotificationProcessor
	}

	// Push notifications are always enabled for RESP3, disabled for RESP2
	if opt.Protocol == 3 {
		// Create default processor for RESP3 connections
		return NewPushNotificationProcessor()
	}

	// Create void processor for RESP2 connections (push notifications not available)
	return NewVoidPushNotificationProcessor()
}

// RegisterPushNotificationHandler registers a handler for a specific push notification name.
// Returns an error if a handler is already registered for this push notification name.
// If protected is true, the handler cannot be unregistered.
func (c *Client) RegisterPushNotificationHandler(pushNotificationName string, handler push.NotificationHandler, protected bool) error {
	return c.pushProcessor.RegisterHandler(pushNotificationName, handler, protected)
}

// GetPushNotificationHandler returns the handler for a specific push notification name.
// Returns nil if no handler is registered for the given name.
func (c *Client) GetPushNotificationHandler(pushNotificationName string) push.NotificationHandler {
	return c.pushProcessor.GetHandler(pushNotificationName)
}

type PoolStats pool.Stats

// PoolStats returns connection pool stats.
func (c *Client) PoolStats() *PoolStats {
	stats := c.connPool.Stats()
	stats.PubSubStats = *(c.pubSubPool.Stats())
	return (*PoolStats)(stats)
}

func (c *Client) Pipelined(ctx context.Context, fn func(Pipeliner) error) ([]Cmder, error) {
	return c.Pipeline().Pipelined(ctx, fn)
}

func (c *Client) Pipeline() Pipeliner {
	pipe := Pipeline{
		exec: pipelineExecer(c.processPipelineHook),
	}
	pipe.init()
	return &pipe
}

func (c *Client) TxPipelined(ctx context.Context, fn func(Pipeliner) error) ([]Cmder, error) {
	return c.TxPipeline().Pipelined(ctx, fn)
}

// TxPipeline acts like Pipeline, but wraps queued commands with MULTI/EXEC.
func (c *Client) TxPipeline() Pipeliner {
	pipe := Pipeline{
		exec: func(ctx context.Context, cmds []Cmder) error {
			cmds = wrapMultiExec(ctx, cmds)
			return c.processTxPipelineHook(ctx, cmds)
		},
	}
	pipe.init()
	return &pipe
}

func (c *Client) pubSub() *PubSub {
	pubsub := &PubSub{
		opt: c.opt,
		newConn: func(ctx context.Context, addr string, channels []string) (*pool.Conn, error) {
			cn, err := c.pubSubPool.NewConn(ctx, c.opt.Network, addr, channels)
			if err != nil {
				return nil, err
			}
			// will return nil if already initialized
			err = c.initConn(ctx, cn)
			if err != nil {
				_ = cn.Close()
				return nil, err
			}
			// Track connection in PubSubPool
			c.pubSubPool.TrackConn(cn)
			return cn, nil
		},
		closeConn: func(cn *pool.Conn) error {
			// Untrack connection from PubSubPool
			c.pubSubPool.UntrackConn(cn)
			_ = cn.Close()
			return nil
		},
		pushProcessor: c.pushProcessor,
	}
	pubsub.init()

	return pubsub
}

// Subscribe subscribes the client to the specified channels.
// Channels can be omitted to create empty subscription.
// Note that this method does not wait on a response from Redis, so the
// subscription may not be active immediately. To force the connection to wait,
// you may call the Receive() method on the returned *PubSub like so:
//
//	sub := client.Subscribe(queryResp)
//	iface, err := sub.Receive()
//	if err != nil {
//	    // handle error
//	}
//
//	// Should be *Subscription, but others are possible if other actions have been
//	// taken on sub since it was created.
//	switch iface.(type) {
//	case *Subscription:
//	    // subscribe succeeded
//	case *Message:
//	    // received first message
//	case *Pong:
//	    // pong received
//	default:
//	    // handle error
//	}
//
//	ch := sub.Channel()
func (c *Client) Subscribe(ctx context.Context, channels ...string) *PubSub {
	pubsub := c.pubSub()
	if len(channels) > 0 {
		_ = pubsub.Subscribe(ctx, channels...)
	}
	return pubsub
}

// PSubscribe subscribes the client to the given patterns.
// Patterns can be omitted to create empty subscription.
func (c *Client) PSubscribe(ctx context.Context, channels ...string) *PubSub {
	pubsub := c.pubSub()
	if len(channels) > 0 {
		_ = pubsub.PSubscribe(ctx, channels...)
	}
	return pubsub
}

// SSubscribe Subscribes the client to the specified shard channels.
// Channels can be omitted to create empty subscription.
func (c *Client) SSubscribe(ctx context.Context, channels ...string) *PubSub {
	pubsub := c.pubSub()
	if len(channels) > 0 {
		_ = pubsub.SSubscribe(ctx, channels...)
	}
	return pubsub
}

//------------------------------------------------------------------------------

// Conn represents a single Redis connection rather than a pool of connections.
// Prefer running commands from Client unless there is a specific need
// for a continuous single Redis connection.
type Conn struct {
	baseClient
	cmdable
	statefulCmdable
}

// newConn is a helper func to create a new Conn instance.
// the Conn instance is not thread-safe and should not be shared between goroutines.
// the parentHooks will be cloned, no need to clone before passing it.
func newConn(opt *Options, connPool pool.Pooler, parentHooks *hooksMixin) *Conn {
	c := Conn{
		baseClient: baseClient{
			opt:      opt,
			connPool: connPool,
		},
	}

	if parentHooks != nil {
		c.hooksMixin = parentHooks.clone()
	}

	// Initialize push notification processor using shared helper
	// Use void processor for RESP2 connections (push notifications not available)
	c.pushProcessor = initializePushProcessor(opt)

	c.cmdable = c.Process
	c.statefulCmdable = c.Process
	c.initHooks(hooks{
		dial:       c.baseClient.dial,
		process:    c.baseClient.process,
		pipeline:   c.baseClient.processPipeline,
		txPipeline: c.baseClient.processTxPipeline,
	})

	return &c
}

func (c *Conn) Process(ctx context.Context, cmd Cmder) error {
	err := c.processHook(ctx, cmd)
	cmd.SetErr(err)
	return err
}

// RegisterPushNotificationHandler registers a handler for a specific push notification name.
// Returns an error if a handler is already registered for this push notification name.
// If protected is true, the handler cannot be unregistered.
func (c *Conn) RegisterPushNotificationHandler(pushNotificationName string, handler push.NotificationHandler, protected bool) error {
	return c.pushProcessor.RegisterHandler(pushNotificationName, handler, protected)
}

func (c *Conn) Pipelined(ctx context.Context, fn func(Pipeliner) error) ([]Cmder, error) {
	return c.Pipeline().Pipelined(ctx, fn)
}

func (c *Conn) Pipeline() Pipeliner {
	pipe := Pipeline{
		exec: c.processPipelineHook,
	}
	pipe.init()
	return &pipe
}

func (c *Conn) TxPipelined(ctx context.Context, fn func(Pipeliner) error) ([]Cmder, error) {
	return c.TxPipeline().Pipelined(ctx, fn)
}

// TxPipeline acts like Pipeline, but wraps queued commands with MULTI/EXEC.
func (c *Conn) TxPipeline() Pipeliner {
	pipe := Pipeline{
		exec: func(ctx context.Context, cmds []Cmder) error {
			cmds = wrapMultiExec(ctx, cmds)
			return c.processTxPipelineHook(ctx, cmds)
		},
	}
	pipe.init()
	return &pipe
}

// processPushNotifications processes all pending push notifications on a connection
// This ensures that cluster topology changes are handled immediately before the connection is used
// This method should be called by the client before using WithReader for command execution
//
// Performance optimization: Skip the expensive MaybeHasData() syscall if a health check
// was performed recently (within 5 seconds). The health check already verified the connection
// is healthy and checked for unexpected data (push notifications).
func (c *baseClient) processPushNotifications(ctx context.Context, cn *pool.Conn) error {
	// Only process push notifications for RESP3 connections with a processor
	if c.opt.Protocol != 3 || c.pushProcessor == nil {
		return nil
	}

	// Performance optimization: Skip MaybeHasData() syscall if health check was recent
	// If the connection was health-checked within the last 5 seconds, we can skip the
	// expensive syscall since the health check already verified no unexpected data.
	// This is safe because:
	// 0. lastHealthCheckNs is set in pool/conn.go:putConn() after a successful health check
	// 1. Health check (connCheck) uses the same syscall (Recvfrom with MSG_PEEK)
	// 2. If push notifications arrived, they would have been detected by health check
	// 3. 5 seconds is short enough that connection state is still fresh
	// 4. Push notifications will be processed by the next WithReader call
	// used it is set on getConn, so we should use another timer (lastPutAt?)
	lastHealthCheckNs := cn.LastPutAtNs()
	if lastHealthCheckNs > 0 {
		// Use pool's cached time to avoid expensive time.Now() syscall
		nowNs := pool.GetCachedTimeNs()
		if nowNs-lastHealthCheckNs < int64(5*time.Second) {
			// Recent health check confirmed no unexpected data, skip the syscall
			return nil
		}
	}

	// Check if there is any data to read before processing
	// This is an optimization on UNIX systems where MaybeHasData is a syscall
	// On Windows, MaybeHasData always returns true, so this check is a no-op
	if !cn.MaybeHasData() {
		return nil
	}

	// Use WithReader to access the reader and process push notifications
	// This is critical for maintnotifications to work properly
	// NOTE: almost no timeouts are set for this read, so it should not block
	// longer than necessary, 10us should be plenty of time to read if there are any push notifications
	// on the socket.
	return cn.WithReader(ctx, 10*time.Microsecond, func(rd *proto.Reader) error {
		// Create handler context with client, connection pool, and connection information
		handlerCtx := c.pushNotificationHandlerContext(cn)
		return c.pushProcessor.ProcessPendingNotifications(ctx, handlerCtx, rd)
	})
}

// processPendingPushNotificationWithReader processes all pending push notifications on a connection
// This method should be called by the client in WithReader before reading the reply
func (c *baseClient) processPendingPushNotificationWithReader(ctx context.Context, cn *pool.Conn, rd *proto.Reader) error {
	// if we have the reader, we don't need to check for data on the socket, we are waiting
	// for either a reply or a push notification, so we can block until we get a reply or reach the timeout
	if c.opt.Protocol != 3 || c.pushProcessor == nil {
		return nil
	}

	// Create handler context with client, connection pool, and connection information
	handlerCtx := c.pushNotificationHandlerContext(cn)
	return c.pushProcessor.ProcessPendingNotifications(ctx, handlerCtx, rd)
}

// pushNotificationHandlerContext creates a handler context for push notification processing
func (c *baseClient) pushNotificationHandlerContext(cn *pool.Conn) push.NotificationHandlerContext {
	return push.NotificationHandlerContext{
		Client:   c,
		ConnPool: c.connPool,
		Conn:     cn, // Wrap in adapter for easier interface access
	}
}
