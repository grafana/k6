package fasthttp

import (
	"sync"
	"sync/atomic"
	"time"
)

// BalancingClient is the interface for clients, which may be passed
// to LBClient.Clients.
type BalancingClient interface {
	DoDeadline(req *Request, resp *Response, deadline time.Time) error
	PendingRequests() int
}

// LBClient balances requests among available LBClient.Clients.
//
// It has the following features:
//
//   - Balances load among available clients using 'least loaded' + 'least total'
//     hybrid technique.
//   - Dynamically decreases load on unhealthy clients.
//
// It is forbidden copying LBClient instances. Create new instances instead.
//
// It is safe calling LBClient methods from concurrently running goroutines.
type LBClient struct {
	noCopy noCopy

	// Clients must contain non-zero clients list.
	// Incoming requests are balanced among these clients.
	Clients []BalancingClient

	// HealthCheck is a callback called after each request.
	//
	// The request, response and the error returned by the client
	// is passed to HealthCheck, so the callback may determine whether
	// the client is healthy.
	//
	// Load on the current client is decreased if HealthCheck returns false.
	//
	// By default HealthCheck returns false if err != nil.
	HealthCheck func(req *Request, resp *Response, err error) bool

	// Timeout is the request timeout used when calling LBClient.Do.
	//
	// DefaultLBClientTimeout is used by default.
	Timeout time.Duration

	cs []*lbClient

	once sync.Once
	mu   sync.RWMutex
}

// DefaultLBClientTimeout is the default request timeout used by LBClient
// when calling LBClient.Do.
//
// The timeout may be overridden via LBClient.Timeout.
const DefaultLBClientTimeout = time.Second

// DoDeadline calls DoDeadline on the least loaded client
func (cc *LBClient) DoDeadline(req *Request, resp *Response, deadline time.Time) error {
	return cc.get().DoDeadline(req, resp, deadline)
}

// DoTimeout calculates deadline and calls DoDeadline on the least loaded client
func (cc *LBClient) DoTimeout(req *Request, resp *Response, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	return cc.get().DoDeadline(req, resp, deadline)
}

// Do calculates timeout using LBClient.Timeout and calls DoTimeout
// on the least loaded client.
func (cc *LBClient) Do(req *Request, resp *Response) error {
	timeout := cc.Timeout
	if timeout <= 0 {
		timeout = DefaultLBClientTimeout
	}
	return cc.DoTimeout(req, resp, timeout)
}

func (cc *LBClient) init() {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	if len(cc.Clients) == 0 {
		// developer sanity-check
		panic("BUG: LBClient.Clients cannot be empty")
	}
	for _, c := range cc.Clients {
		cc.cs = append(cc.cs, &lbClient{
			c:           c,
			healthCheck: cc.HealthCheck,
		})
	}
}

// AddClient adds a new client to the balanced clients
// returns the new total number of clients
func (cc *LBClient) AddClient(c BalancingClient) int {
	cc.mu.Lock()
	cc.cs = append(cc.cs, &lbClient{
		c:           c,
		healthCheck: cc.HealthCheck,
	})
	cc.mu.Unlock()
	return len(cc.cs)
}

// RemoveClients removes clients using the provided callback
// if rc returns true, the passed client will be removed
// returns the new total number of clients
func (cc *LBClient) RemoveClients(rc func(BalancingClient) bool) int {
	cc.mu.Lock()
	n := 0
	for idx, cs := range cc.cs {
		cc.cs[idx] = nil
		if rc(cs.c) {
			continue
		}
		cc.cs[n] = cs
		n++
	}
	cc.cs = cc.cs[:n]

	cc.mu.Unlock()
	return len(cc.cs)
}

func (cc *LBClient) get() *lbClient {
	cc.once.Do(cc.init)

	cc.mu.RLock()
	cs := cc.cs

	minC := cs[0]
	minN := minC.PendingRequests()
	minT := atomic.LoadUint64(&minC.total)
	for _, c := range cs[1:] {
		n := c.PendingRequests()
		t := atomic.LoadUint64(&c.total)
		if n < minN || (n == minN && t < minT) {
			minC = c
			minN = n
			minT = t
		}
	}
	cc.mu.RUnlock()
	return minC
}

type lbClient struct {
	c           BalancingClient
	healthCheck func(req *Request, resp *Response, err error) bool
	penalty     uint32

	// total amount of requests handled.
	total uint64
}

func (c *lbClient) DoDeadline(req *Request, resp *Response, deadline time.Time) error {
	err := c.c.DoDeadline(req, resp, deadline)
	if !c.isHealthy(req, resp, err) && c.incPenalty() {
		// Penalize the client returning error, so the next requests
		// are routed to another clients.
		time.AfterFunc(penaltyDuration, c.decPenalty)
	} else {
		atomic.AddUint64(&c.total, 1)
	}
	return err
}

func (c *lbClient) PendingRequests() int {
	n := c.c.PendingRequests()
	m := atomic.LoadUint32(&c.penalty)
	return n + int(m)
}

func (c *lbClient) isHealthy(req *Request, resp *Response, err error) bool {
	if c.healthCheck == nil {
		return err == nil
	}
	return c.healthCheck(req, resp, err)
}

func (c *lbClient) incPenalty() bool {
	m := atomic.AddUint32(&c.penalty, 1)
	if m > maxPenalty {
		c.decPenalty()
		return false
	}
	return true
}

func (c *lbClient) decPenalty() {
	atomic.AddUint32(&c.penalty, ^uint32(0))
}

const (
	maxPenalty = 300

	penaltyDuration = 3 * time.Second
)
