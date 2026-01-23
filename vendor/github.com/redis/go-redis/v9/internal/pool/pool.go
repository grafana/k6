package pool

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9/internal"
	"github.com/redis/go-redis/v9/internal/proto"
	"github.com/redis/go-redis/v9/internal/util"
)

var (
	// ErrClosed performs any operation on the closed client will return this error.
	ErrClosed = errors.New("redis: client is closed")

	// ErrPoolExhausted is returned from a pool connection method
	// when the maximum number of database connections in the pool has been reached.
	ErrPoolExhausted = errors.New("redis: connection pool exhausted")

	// ErrPoolTimeout timed out waiting to get a connection from the connection pool.
	ErrPoolTimeout = errors.New("redis: connection pool timeout")

	// ErrConnUnusableTimeout is returned when a connection is not usable and we timed out trying to mark it as unusable.
	ErrConnUnusableTimeout = errors.New("redis: timed out trying to mark connection as unusable")

	// errHookRequestedRemoval is returned when a hook requests connection removal.
	errHookRequestedRemoval = errors.New("hook requested removal")

	// errConnNotPooled is returned when trying to return a non-pooled connection to the pool.
	errConnNotPooled = errors.New("connection not pooled")

	// popAttempts is the maximum number of attempts to find a usable connection
	// when popping from the idle connection pool. This handles cases where connections
	// are temporarily marked as unusable (e.g., during maintenanceNotifications upgrades or network issues).
	// Value of 50 provides sufficient resilience without excessive overhead.
	// This is capped by the idle connection count, so we won't loop excessively.
	popAttempts = 50

	// getAttempts is the maximum number of attempts to get a connection that passes
	// hook validation (e.g., maintenanceNotifications upgrade hooks). This protects against race conditions
	// where hooks might temporarily reject connections during cluster transitions.
	// Value of 3 balances resilience with performance - most hook rejections resolve quickly.
	getAttempts = 3

	minTime      = time.Unix(-2208988800, 0) // Jan 1, 1900
	maxTime      = minTime.Add(1<<63 - 1)
	noExpiration = maxTime
)

// Stats contains pool state information and accumulated stats.
type Stats struct {
	Hits           uint32 // number of times free connection was found in the pool
	Misses         uint32 // number of times free connection was NOT found in the pool
	Timeouts       uint32 // number of times a wait timeout occurred
	WaitCount      uint32 // number of times a connection was waited
	Unusable       uint32 // number of times a connection was found to be unusable
	WaitDurationNs int64  // total time spent for waiting a connection in nanoseconds

	TotalConns uint32 // number of total connections in the pool
	IdleConns  uint32 // number of idle connections in the pool
	StaleConns uint32 // number of stale connections removed from the pool

	PubSubStats PubSubStats
}

type Pooler interface {
	NewConn(context.Context) (*Conn, error)
	CloseConn(*Conn) error

	Get(context.Context) (*Conn, error)
	Put(context.Context, *Conn)
	Remove(context.Context, *Conn, error)

	Len() int
	IdleLen() int
	Stats() *Stats

	// Size returns the maximum pool size (capacity).
	// This is used by the streaming credentials manager to size the re-auth worker pool.
	Size() int

	AddPoolHook(hook PoolHook)
	RemovePoolHook(hook PoolHook)

	// RemoveWithoutTurn removes a connection from the pool without freeing a turn.
	// This should be used when removing a connection from a context that didn't acquire
	// a turn via Get() (e.g., background workers, cleanup tasks).
	// For normal removal after Get(), use Remove() instead.
	RemoveWithoutTurn(context.Context, *Conn, error)

	Close() error
}

type Options struct {
	Dialer          func(context.Context) (net.Conn, error)
	ReadBufferSize  int
	WriteBufferSize int

	PoolFIFO                 bool
	PoolSize                 int32
	MaxConcurrentDials       int
	DialTimeout              time.Duration
	PoolTimeout              time.Duration
	MinIdleConns             int32
	MaxIdleConns             int32
	MaxActiveConns           int32
	ConnMaxIdleTime          time.Duration
	ConnMaxLifetime          time.Duration
	PushNotificationsEnabled bool

	// DialerRetries is the maximum number of retry attempts when dialing fails.
	// Default: 5
	DialerRetries int

	// DialerRetryTimeout is the backoff duration between retry attempts.
	// Default: 100ms
	DialerRetryTimeout time.Duration
}

type lastDialErrorWrap struct {
	err error
}

type ConnPool struct {
	cfg *Options

	dialErrorsNum uint32 // atomic
	lastDialError atomic.Value

	queue           chan struct{}
	dialsInProgress chan struct{}
	dialsQueue      *wantConnQueue
	// Fast semaphore for connection limiting with eventual fairness
	// Uses fast path optimization to avoid timer allocation when tokens are available
	semaphore *internal.FastSemaphore

	connsMu   sync.Mutex
	conns     map[uint64]*Conn
	idleConns []*Conn

	poolSize            atomic.Int32
	idleConnsLen        atomic.Int32
	idleCheckInProgress atomic.Bool
	idleCheckNeeded     atomic.Bool

	stats          Stats
	waitDurationNs atomic.Int64

	_closed uint32 // atomic

	// Pool hooks manager for flexible connection processing
	// Using atomic.Pointer for lock-free reads in hot paths (Get/Put)
	hookManager atomic.Pointer[PoolHookManager]
}

var _ Pooler = (*ConnPool)(nil)

func NewConnPool(opt *Options) *ConnPool {
	p := &ConnPool{
		cfg:             opt,
		semaphore:       internal.NewFastSemaphore(opt.PoolSize),
		queue:           make(chan struct{}, opt.PoolSize),
		conns:           make(map[uint64]*Conn),
		dialsInProgress: make(chan struct{}, opt.MaxConcurrentDials),
		dialsQueue:      newWantConnQueue(),
		idleConns:       make([]*Conn, 0, opt.PoolSize),
	}

	// Only create MinIdleConns if explicitly requested (> 0)
	// This avoids creating connections during pool initialization for tests
	if opt.MinIdleConns > 0 {
		p.connsMu.Lock()
		p.checkMinIdleConns()
		p.connsMu.Unlock()
	}

	return p
}

// initializeHooks sets up the pool hooks system.
func (p *ConnPool) initializeHooks() {
	manager := NewPoolHookManager()
	p.hookManager.Store(manager)
}

// AddPoolHook adds a pool hook to the pool.
func (p *ConnPool) AddPoolHook(hook PoolHook) {
	// Lock-free read of current manager
	manager := p.hookManager.Load()
	if manager == nil {
		p.initializeHooks()
		manager = p.hookManager.Load()
	}

	// Create new manager with added hook
	newManager := manager.Clone()
	newManager.AddHook(hook)

	// Atomically swap to new manager
	p.hookManager.Store(newManager)
}

// RemovePoolHook removes a pool hook from the pool.
func (p *ConnPool) RemovePoolHook(hook PoolHook) {
	manager := p.hookManager.Load()
	if manager != nil {
		// Create new manager with removed hook
		newManager := manager.Clone()
		newManager.RemoveHook(hook)

		// Atomically swap to new manager
		p.hookManager.Store(newManager)
	}
}

func (p *ConnPool) checkMinIdleConns() {
	// If a check is already in progress, mark that we need another check and return
	if !p.idleCheckInProgress.CompareAndSwap(false, true) {
		p.idleCheckNeeded.Store(true)
		return
	}

	if p.cfg.MinIdleConns == 0 {
		p.idleCheckInProgress.Store(false)
		return
	}

	// Keep checking until no more checks are needed
	// This handles the case where multiple Remove() calls happen concurrently
	for {
		// Clear the "check needed" flag before we start
		p.idleCheckNeeded.Store(false)

		// Only create idle connections if we haven't reached the total pool size limit
		// MinIdleConns should be a subset of PoolSize, not additional connections
		for p.poolSize.Load() < p.cfg.PoolSize && p.idleConnsLen.Load() < p.cfg.MinIdleConns {
			// Try to acquire a semaphore token
			if !p.semaphore.TryAcquire() {
				// Semaphore is full, can't create more connections
				p.idleCheckInProgress.Store(false)
				return
			}

			p.poolSize.Add(1)
			p.idleConnsLen.Add(1)
			go func() {
				defer func() {
					if err := recover(); err != nil {
						p.poolSize.Add(-1)
						p.idleConnsLen.Add(-1)

						p.freeTurn()
						internal.Logger.Printf(context.Background(), "addIdleConn panic: %+v", err)
					}
				}()

				err := p.addIdleConn()
				if err != nil && err != ErrClosed {
					p.poolSize.Add(-1)
					p.idleConnsLen.Add(-1)
				}
				p.freeTurn()
			}()
		}

		// If no one requested another check while we were working, we're done
		if !p.idleCheckNeeded.Load() {
			p.idleCheckInProgress.Store(false)
			return
		}

		// Otherwise, loop again to handle the new requests
	}
}

func (p *ConnPool) addIdleConn() error {
	ctx, cancel := context.WithTimeout(context.Background(), p.cfg.DialTimeout)
	defer cancel()

	cn, err := p.dialConn(ctx, true)
	if err != nil {
		return err
	}

	// NOTE: Connection is in CREATED state and will be initialized by redis.go:initConn()
	// when first acquired from the pool. Do NOT transition to IDLE here - that happens
	// after initialization completes.

	p.connsMu.Lock()
	defer p.connsMu.Unlock()

	// It is not allowed to add new connections to the closed connection pool.
	if p.closed() {
		_ = cn.Close()
		return ErrClosed
	}

	p.conns[cn.GetID()] = cn
	p.idleConns = append(p.idleConns, cn)
	return nil
}

// NewConn creates a new connection and returns it to the user.
// This will still obey MaxActiveConns but will not include it in the pool and won't increase the pool size.
//
// NOTE: If you directly get a connection from the pool, it won't be pooled and won't support maintnotifications upgrades.
func (p *ConnPool) NewConn(ctx context.Context) (*Conn, error) {
	return p.newConn(ctx, false)
}

func (p *ConnPool) newConn(ctx context.Context, pooled bool) (*Conn, error) {
	if p.closed() {
		return nil, ErrClosed
	}

	if p.cfg.MaxActiveConns > 0 && p.poolSize.Load() >= p.cfg.MaxActiveConns {
		return nil, ErrPoolExhausted
	}

	dialCtx, cancel := context.WithTimeout(ctx, p.cfg.DialTimeout)
	defer cancel()
	cn, err := p.dialConn(dialCtx, pooled)
	if err != nil {
		return nil, err
	}

	// NOTE: Connection is in CREATED state and will be initialized by redis.go:initConn()
	// when first used. Do NOT transition to IDLE here - that happens after initialization completes.
	// The state machine flow is: CREATED → INITIALIZING (in initConn) → IDLE (after init success)

	if p.cfg.MaxActiveConns > 0 && p.poolSize.Load() > p.cfg.MaxActiveConns {
		_ = cn.Close()
		return nil, ErrPoolExhausted
	}

	p.connsMu.Lock()
	defer p.connsMu.Unlock()
	if p.closed() {
		_ = cn.Close()
		return nil, ErrClosed
	}
	// Check if pool was closed while we were waiting for the lock
	if p.conns == nil {
		p.conns = make(map[uint64]*Conn)
	}
	p.conns[cn.GetID()] = cn

	if pooled {
		// If pool is full remove the cn on next Put.
		currentPoolSize := p.poolSize.Load()
		if currentPoolSize >= p.cfg.PoolSize {
			cn.pooled = false
		} else {
			p.poolSize.Add(1)
		}
	}

	return cn, nil
}

func (p *ConnPool) dialConn(ctx context.Context, pooled bool) (*Conn, error) {
	if p.closed() {
		return nil, ErrClosed
	}

	if atomic.LoadUint32(&p.dialErrorsNum) >= uint32(p.cfg.PoolSize) {
		return nil, p.getLastDialError()
	}

	// Retry dialing with backoff
	// the context timeout is already handled by the context passed in
	// so we may never reach the max retries, higher values don't hurt
	maxRetries := p.cfg.DialerRetries
	if maxRetries <= 0 {
		maxRetries = 5 // Default value
	}
	backoffDuration := p.cfg.DialerRetryTimeout
	if backoffDuration <= 0 {
		backoffDuration = 100 * time.Millisecond // Default value
	}

	var lastErr error
	shouldLoop := true
	// when the timeout is reached, we should stop retrying
	// but keep the lastErr to return to the caller
	// instead of a generic context deadline exceeded error
	attempt := 0
	for attempt = 0; (attempt < maxRetries) && shouldLoop; attempt++ {
		netConn, err := p.cfg.Dialer(ctx)
		if err != nil {
			lastErr = err
			// Add backoff delay for retry attempts
			// (not for the first attempt, do at least one)
			select {
			case <-ctx.Done():
				shouldLoop = false
			case <-time.After(backoffDuration):
				// Continue with retry
			}
			continue
		}

		// Success - create connection
		cn := NewConnWithBufferSize(netConn, p.cfg.ReadBufferSize, p.cfg.WriteBufferSize)
		cn.pooled = pooled
		if p.cfg.ConnMaxLifetime > 0 {
			cn.expiresAt = time.Now().Add(p.cfg.ConnMaxLifetime)
		} else {
			cn.expiresAt = noExpiration
		}

		return cn, nil
	}

	internal.Logger.Printf(ctx, "redis: connection pool: failed to dial after %d attempts: %v", attempt, lastErr)
	// All retries failed - handle error tracking
	p.setLastDialError(lastErr)
	if atomic.AddUint32(&p.dialErrorsNum, 1) == uint32(p.cfg.PoolSize) {
		go p.tryDial()
	}
	return nil, lastErr
}

func (p *ConnPool) tryDial() {
	for {
		if p.closed() {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), p.cfg.DialTimeout)

		conn, err := p.cfg.Dialer(ctx)
		if err != nil {
			p.setLastDialError(err)
			time.Sleep(time.Second)
			cancel()
			continue
		}

		atomic.StoreUint32(&p.dialErrorsNum, 0)
		_ = conn.Close()
		cancel()
		return
	}
}

func (p *ConnPool) setLastDialError(err error) {
	p.lastDialError.Store(&lastDialErrorWrap{err: err})
}

func (p *ConnPool) getLastDialError() error {
	err, _ := p.lastDialError.Load().(*lastDialErrorWrap)
	if err != nil {
		return err.err
	}
	return nil
}

// Get returns existed connection from the pool or creates a new one.
func (p *ConnPool) Get(ctx context.Context) (*Conn, error) {
	return p.getConn(ctx)
}

// getConn returns a connection from the pool.
func (p *ConnPool) getConn(ctx context.Context) (*Conn, error) {
	var cn *Conn
	var err error

	if p.closed() {
		return nil, ErrClosed
	}

	if err := p.waitTurn(ctx); err != nil {
		return nil, err
	}

	// Use cached time for health checks (max 50ms staleness is acceptable)
	nowNs := getCachedTimeNs()

	// Lock-free atomic read - no mutex overhead!
	hookManager := p.hookManager.Load()

	for attempts := 0; attempts < getAttempts; attempts++ {

		p.connsMu.Lock()
		cn, err = p.popIdle()
		p.connsMu.Unlock()

		if err != nil {
			p.freeTurn()
			return nil, err
		}

		if cn == nil {
			break
		}

		if !p.isHealthyConn(cn, nowNs) {
			_ = p.CloseConn(cn)
			continue
		}

		// Process connection using the hooks system
		// Combine error and rejection checks to reduce branches
		if hookManager != nil {
			acceptConn, err := hookManager.ProcessOnGet(ctx, cn, false)
			if err != nil || !acceptConn {
				if err != nil {
					internal.Logger.Printf(ctx, "redis: connection pool: failed to process idle connection by hook: %v", err)
					_ = p.CloseConn(cn)
				} else {
					internal.Logger.Printf(ctx, "redis: connection pool: conn[%d] rejected by hook, returning to pool", cn.GetID())
					// Return connection to pool without freeing the turn that this Get() call holds.
					// We use putConnWithoutTurn() to run all the Put hooks and logic without freeing a turn.
					p.putConnWithoutTurn(ctx, cn)
					cn = nil
				}
				continue
			}
		}

		atomic.AddUint32(&p.stats.Hits, 1)
		return cn, nil
	}

	atomic.AddUint32(&p.stats.Misses, 1)

	newcn, err := p.queuedNewConn(ctx)
	if err != nil {
		return nil, err
	}

	// Process connection using the hooks system
	if hookManager != nil {
		acceptConn, err := hookManager.ProcessOnGet(ctx, newcn, true)
		// both errors and accept=false mean a hook rejected the connection
		// this should not happen with a new connection, but we handle it gracefully
		if err != nil || !acceptConn {
			// Failed to process connection, discard it
			internal.Logger.Printf(ctx, "redis: connection pool: failed to process new connection conn[%d] by hook: accept=%v, err=%v", newcn.GetID(), acceptConn, err)
			_ = p.CloseConn(newcn)
			return nil, err
		}
	}
	return newcn, nil
}

func (p *ConnPool) queuedNewConn(ctx context.Context) (*Conn, error) {
	select {
	case p.dialsInProgress <- struct{}{}:
		// Got permission, proceed to create connection
	case <-ctx.Done():
		p.freeTurn()
		return nil, ctx.Err()
	}

	dialCtx, cancel := context.WithTimeout(context.Background(), p.cfg.DialTimeout)

	w := &wantConn{
		ctx:       dialCtx,
		cancelCtx: cancel,
		result:    make(chan wantConnResult, 1),
	}
	var err error
	defer func() {
		if err != nil {
			if cn := w.cancel(); cn != nil && p.putIdleConn(ctx, cn) {
				p.freeTurn()
			}
		}
	}()

	p.dialsQueue.enqueue(w)

	go func(w *wantConn) {
		var freeTurnCalled bool
		defer func() {
			if err := recover(); err != nil {
				if !freeTurnCalled {
					p.freeTurn()
				}
				internal.Logger.Printf(context.Background(), "queuedNewConn panic: %+v", err)
			}
		}()

		defer w.cancelCtx()
		defer func() { <-p.dialsInProgress }() // Release connection creation permission

		dialCtx := w.getCtxForDial()
		cn, cnErr := p.newConn(dialCtx, true)
		if cnErr != nil {
			w.tryDeliver(nil, cnErr) // deliver error to caller, notify connection creation failed
			p.freeTurn()
			freeTurnCalled = true
			return
		}

		delivered := w.tryDeliver(cn, cnErr)
		if !delivered && p.putIdleConn(dialCtx, cn) {
			p.freeTurn()
			freeTurnCalled = true
		}
	}(w)

	select {
	case <-ctx.Done():
		err = ctx.Err()
		return nil, err
	case result := <-w.result:
		err = result.err
		return result.cn, err
	}
}

// putIdleConn puts a connection back to the pool or passes it to the next waiting request.
//
// It returns true if the connection was put back to the pool,
// which means the turn needs to be freed directly by the caller,
// or false if the connection was passed to the next waiting request,
// which means the turn will be freed by the waiting goroutine after it returns.
func (p *ConnPool) putIdleConn(ctx context.Context, cn *Conn) bool {
	for {
		w, ok := p.dialsQueue.dequeue()
		if !ok {
			break
		}
		if w.tryDeliver(cn, nil) {
			return false
		}
	}

	p.connsMu.Lock()
	defer p.connsMu.Unlock()

	if p.closed() {
		_ = cn.Close()
		return true
	}

	// poolSize is increased in newConn
	p.idleConns = append(p.idleConns, cn)
	p.idleConnsLen.Add(1)

	return true
}

func (p *ConnPool) waitTurn(ctx context.Context) error {
	// Fast path: check context first
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Fast path: try to acquire without blocking
	if p.semaphore.TryAcquire() {
		return nil
	}

	// Slow path: need to wait
	start := time.Now()
	err := p.semaphore.Acquire(ctx, p.cfg.PoolTimeout, ErrPoolTimeout)

	switch err {
	case nil:
		// Successfully acquired after waiting
		p.waitDurationNs.Add(time.Now().UnixNano() - start.UnixNano())
		atomic.AddUint32(&p.stats.WaitCount, 1)
	case ErrPoolTimeout:
		atomic.AddUint32(&p.stats.Timeouts, 1)
	}

	return err
}

func (p *ConnPool) freeTurn() {
	p.semaphore.Release()
}

func (p *ConnPool) popIdle() (*Conn, error) {
	if p.closed() {
		return nil, ErrClosed
	}
	defer p.checkMinIdleConns()

	n := len(p.idleConns)
	if n == 0 {
		return nil, nil
	}

	var cn *Conn
	attempts := 0

	maxAttempts := util.Min(popAttempts, n)
	for attempts < maxAttempts {
		if len(p.idleConns) == 0 {
			return nil, nil
		}

		if p.cfg.PoolFIFO {
			cn = p.idleConns[0]
			copy(p.idleConns, p.idleConns[1:])
			p.idleConns = p.idleConns[:len(p.idleConns)-1]
		} else {
			idx := len(p.idleConns) - 1
			cn = p.idleConns[idx]
			p.idleConns = p.idleConns[:idx]
		}
		attempts++

		// Hot path optimization: try IDLE → IN_USE or CREATED → IN_USE transition
		// Using inline TryAcquire() method for better performance (avoids pointer dereference)
		if cn.TryAcquire() {
			// Successfully acquired the connection
			p.idleConnsLen.Add(-1)
			break
		}

		// Connection is in UNUSABLE, INITIALIZING, or other state - skip it

		// Connection is not in a valid state (might be UNUSABLE for handoff/re-auth, INITIALIZING, etc.)
		// Put it back in the pool and try the next one
		if p.cfg.PoolFIFO {
			// FIFO: put at end (will be picked up last since we pop from front)
			p.idleConns = append(p.idleConns, cn)
		} else {
			// LIFO: put at beginning (will be picked up last since we pop from end)
			p.idleConns = append([]*Conn{cn}, p.idleConns...)
		}
		cn = nil
	}

	// If we exhausted all attempts without finding a usable connection, return nil
	if attempts > 1 && attempts >= maxAttempts && int32(attempts) >= p.poolSize.Load() {
		internal.Logger.Printf(context.Background(), "redis: connection pool: failed to get a usable connection after %d attempts", attempts)
		return nil, nil
	}

	return cn, nil
}

func (p *ConnPool) Put(ctx context.Context, cn *Conn) {
	p.putConn(ctx, cn, true)
}

// putConnWithoutTurn is an internal method that puts a connection back to the pool
// without freeing a turn. This is used when returning a rejected connection from
// within Get(), where the turn is still held by the Get() call.
func (p *ConnPool) putConnWithoutTurn(ctx context.Context, cn *Conn) {
	p.putConn(ctx, cn, false)
}

// putConn is the internal implementation of Put that optionally frees a turn.
func (p *ConnPool) putConn(ctx context.Context, cn *Conn, freeTurn bool) {
	// Process connection using the hooks system
	shouldPool := true
	shouldRemove := false
	var err error

	if cn.HasBufferedData() {
		// Peek at the reply type to check if it's a push notification
		if replyType, err := cn.PeekReplyTypeSafe(); err != nil || replyType != proto.RespPush {
			// Not a push notification or error peeking, remove connection
			internal.Logger.Printf(ctx, "Conn has unread data (not push notification), removing it")
			p.removeConnInternal(ctx, cn, err, freeTurn)
			return
		}
		// It's a push notification, allow pooling (client will handle it)
	}

	// Lock-free atomic read - no mutex overhead!
	hookManager := p.hookManager.Load()

	if hookManager != nil {
		shouldPool, shouldRemove, err = hookManager.ProcessOnPut(ctx, cn)
		if err != nil {
			internal.Logger.Printf(ctx, "Connection hook error: %v", err)
			p.removeConnInternal(ctx, cn, err, freeTurn)
			return
		}
	}

	// Combine all removal checks into one - reduces branches
	if shouldRemove || !shouldPool {
		p.removeConnInternal(ctx, cn, errHookRequestedRemoval, freeTurn)
		return
	}

	if !cn.pooled {
		p.removeConnInternal(ctx, cn, errConnNotPooled, freeTurn)
		return
	}

	var shouldCloseConn bool

	if p.cfg.MaxIdleConns == 0 || p.idleConnsLen.Load() < p.cfg.MaxIdleConns {
		// Hot path optimization: try fast IN_USE → IDLE transition
		// Using inline Release() method for better performance (avoids pointer dereference)
		transitionedToIdle := cn.Release()

		// Handle unexpected state changes
		if !transitionedToIdle {
			// Fast path failed - hook might have changed state (e.g., to UNUSABLE for handoff)
			// Keep the state set by the hook and pool the connection anyway
			currentState := cn.GetStateMachine().GetState()
			switch currentState {
			case StateUnusable:
				// expected state, don't log it
			case StateClosed:
				internal.Logger.Printf(ctx, "Unexpected conn[%d] state changed by hook to %v, closing it", cn.GetID(), currentState)
				shouldCloseConn = true
				p.removeConnWithLock(cn)
			default:
				// Pool as-is
				internal.Logger.Printf(ctx, "Unexpected conn[%d] state changed by hook to %v, pooling as-is", cn.GetID(), currentState)
			}
		}

		// unusable conns are expected to become usable at some point (background process is reconnecting them)
		// put them at the opposite end of the queue
		// Optimization: if we just transitioned to IDLE, we know it's usable - skip the check
		if !transitionedToIdle && !cn.IsUsable() {
			if p.cfg.PoolFIFO {
				p.connsMu.Lock()
				p.idleConns = append(p.idleConns, cn)
				p.connsMu.Unlock()
			} else {
				p.connsMu.Lock()
				p.idleConns = append([]*Conn{cn}, p.idleConns...)
				p.connsMu.Unlock()
			}
			p.idleConnsLen.Add(1)
		} else if !shouldCloseConn {
			p.connsMu.Lock()
			p.idleConns = append(p.idleConns, cn)
			p.connsMu.Unlock()
			p.idleConnsLen.Add(1)
		}
	} else {
		shouldCloseConn = true
		p.removeConnWithLock(cn)
	}

	if freeTurn {
		p.freeTurn()
	}

	if shouldCloseConn {
		_ = p.closeConn(cn)
	}

	cn.SetLastPutAtNs(getCachedTimeNs())
}

func (p *ConnPool) Remove(ctx context.Context, cn *Conn, reason error) {
	p.removeConnInternal(ctx, cn, reason, true)
}

// RemoveWithoutTurn removes a connection from the pool without freeing a turn.
// This should be used when removing a connection from a context that didn't acquire
// a turn via Get() (e.g., background workers, cleanup tasks).
// For normal removal after Get(), use Remove() instead.
func (p *ConnPool) RemoveWithoutTurn(ctx context.Context, cn *Conn, reason error) {
	p.removeConnInternal(ctx, cn, reason, false)
}

// removeConnInternal is the internal implementation of Remove that optionally frees a turn.
func (p *ConnPool) removeConnInternal(ctx context.Context, cn *Conn, reason error, freeTurn bool) {
	// Lock-free atomic read - no mutex overhead!
	hookManager := p.hookManager.Load()

	if hookManager != nil {
		hookManager.ProcessOnRemove(ctx, cn, reason)
	}

	p.removeConnWithLock(cn)

	if freeTurn {
		p.freeTurn()
	}

	_ = p.closeConn(cn)

	// Check if we need to create new idle connections to maintain MinIdleConns
	p.checkMinIdleConns()
}

func (p *ConnPool) CloseConn(cn *Conn) error {
	p.removeConnWithLock(cn)
	return p.closeConn(cn)
}

func (p *ConnPool) removeConnWithLock(cn *Conn) {
	p.connsMu.Lock()
	defer p.connsMu.Unlock()
	p.removeConn(cn)
}

func (p *ConnPool) removeConn(cn *Conn) {
	cid := cn.GetID()
	delete(p.conns, cid)
	atomic.AddUint32(&p.stats.StaleConns, 1)

	// Decrement pool size counter when removing a connection
	if cn.pooled {
		p.poolSize.Add(-1)
		// this can be idle conn
		for idx, ic := range p.idleConns {
			if ic == cn {
				p.idleConns = append(p.idleConns[:idx], p.idleConns[idx+1:]...)
				p.idleConnsLen.Add(-1)
				break
			}
		}
	}
}

func (p *ConnPool) closeConn(cn *Conn) error {
	return cn.Close()
}

// Len returns total number of connections.
func (p *ConnPool) Len() int {
	p.connsMu.Lock()
	n := len(p.conns)
	p.connsMu.Unlock()
	return n
}

// IdleLen returns number of idle connections.
func (p *ConnPool) IdleLen() int {
	p.connsMu.Lock()
	n := p.idleConnsLen.Load()
	p.connsMu.Unlock()
	return int(n)
}

// Size returns the maximum pool size (capacity).
//
// This is used by the streaming credentials manager to size the re-auth worker pool,
// ensuring that re-auth operations don't exhaust the connection pool.
func (p *ConnPool) Size() int {
	return int(p.cfg.PoolSize)
}

func (p *ConnPool) Stats() *Stats {
	return &Stats{
		Hits:           atomic.LoadUint32(&p.stats.Hits),
		Misses:         atomic.LoadUint32(&p.stats.Misses),
		Timeouts:       atomic.LoadUint32(&p.stats.Timeouts),
		WaitCount:      atomic.LoadUint32(&p.stats.WaitCount),
		Unusable:       atomic.LoadUint32(&p.stats.Unusable),
		WaitDurationNs: p.waitDurationNs.Load(),

		TotalConns: uint32(p.Len()),
		IdleConns:  uint32(p.IdleLen()),
		StaleConns: atomic.LoadUint32(&p.stats.StaleConns),
	}
}

func (p *ConnPool) closed() bool {
	return atomic.LoadUint32(&p._closed) == 1
}

func (p *ConnPool) Filter(fn func(*Conn) bool) error {
	p.connsMu.Lock()
	defer p.connsMu.Unlock()

	var firstErr error
	for _, cn := range p.conns {
		if fn(cn) {
			if err := p.closeConn(cn); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func (p *ConnPool) Close() error {
	if !atomic.CompareAndSwapUint32(&p._closed, 0, 1) {
		return ErrClosed
	}

	var firstErr error
	p.connsMu.Lock()
	for _, cn := range p.conns {
		if err := p.closeConn(cn); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	p.conns = nil
	p.poolSize.Store(0)
	p.idleConns = nil
	p.idleConnsLen.Store(0)
	p.connsMu.Unlock()

	return firstErr
}

func (p *ConnPool) isHealthyConn(cn *Conn, nowNs int64) bool {
	// Performance optimization: check conditions from cheapest to most expensive,
	// and from most likely to fail to least likely to fail.

	// Only fails if ConnMaxLifetime is set AND connection is old.
	// Most pools don't set ConnMaxLifetime, so this rarely fails.
	if p.cfg.ConnMaxLifetime > 0 {
		if cn.expiresAt.UnixNano() < nowNs {
			return false // Connection has exceeded max lifetime
		}
	}

	// Most pools set ConnMaxIdleTime, and idle connections are common.
	// Checking this first allows us to fail fast without expensive syscalls.
	if p.cfg.ConnMaxIdleTime > 0 {
		if nowNs-cn.UsedAtNs() >= int64(p.cfg.ConnMaxIdleTime) {
			return false // Connection has been idle too long
		}
	}

	// Only run this if the cheap checks passed.
	if err := connCheck(cn.getNetConn()); err != nil {
		// If there's unexpected data, it might be push notifications (RESP3)
		if p.cfg.PushNotificationsEnabled && err == errUnexpectedRead {
			// Peek at the reply type to check if it's a push notification
			if replyType, err := cn.rd.PeekReplyType(); err == nil && replyType == proto.RespPush {
				// For RESP3 connections with push notifications, we allow some buffered data
				// The client will process these notifications before using the connection
				internal.Logger.Printf(
					context.Background(),
					"push: conn[%d] has buffered data, likely push notifications - will be processed by client",
					cn.GetID(),
				)

				// Update timestamp for healthy connection
				cn.SetUsedAtNs(nowNs)

				// Connection is healthy, client will handle notifications
				return true
			}
			// Not a push notification - treat as unhealthy
			return false
		}
		// Connection failed health check
		return false
	}

	// Only update UsedAt if connection is healthy (avoids unnecessary atomic store)
	cn.SetUsedAtNs(nowNs)
	return true
}
