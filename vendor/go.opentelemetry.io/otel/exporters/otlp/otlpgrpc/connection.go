// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package otlpgrpc

import (
	"context"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type connection struct {
	// Ensure pointer is 64-bit aligned for atomic operations on both 32 and 64 bit machines.
	lastConnectErrPtr unsafe.Pointer

	// mu protects the connection as it is accessed by the
	// exporter goroutines and background connection goroutine
	mu sync.Mutex
	cc *grpc.ClientConn

	// these fields are read-only after constructor is finished
	cfg                  config
	metadata             metadata.MD
	newConnectionHandler func(cc *grpc.ClientConn)

	// these channels are created once
	disconnectedCh             chan bool
	backgroundConnectionDoneCh chan struct{}
	stopCh                     chan struct{}

	// this is for tests, so they can replace the closing
	// routine without a worry of modifying some global variable
	// or changing it back to original after the test is done
	closeBackgroundConnectionDoneCh func(ch chan struct{})
}

func newConnection(cfg config, handler func(cc *grpc.ClientConn)) *connection {
	c := new(connection)
	c.newConnectionHandler = handler
	c.cfg = cfg
	if len(c.cfg.headers) > 0 {
		c.metadata = metadata.New(c.cfg.headers)
	}
	c.closeBackgroundConnectionDoneCh = func(ch chan struct{}) {
		close(ch)
	}
	return c
}

func (c *connection) startConnection(ctx context.Context) {
	c.stopCh = make(chan struct{})
	c.disconnectedCh = make(chan bool)
	c.backgroundConnectionDoneCh = make(chan struct{})

	if err := c.connect(ctx); err == nil {
		c.setStateConnected()
	} else {
		c.setStateDisconnected(err)
	}
	go c.indefiniteBackgroundConnection()
}

func (c *connection) lastConnectError() error {
	errPtr := (*error)(atomic.LoadPointer(&c.lastConnectErrPtr))
	if errPtr == nil {
		return nil
	}
	return *errPtr
}

func (c *connection) saveLastConnectError(err error) {
	var errPtr *error
	if err != nil {
		errPtr = &err
	}
	atomic.StorePointer(&c.lastConnectErrPtr, unsafe.Pointer(errPtr))
}

func (c *connection) setStateDisconnected(err error) {
	c.saveLastConnectError(err)
	select {
	case c.disconnectedCh <- true:
	default:
	}
	c.newConnectionHandler(nil)
}

func (c *connection) setStateConnected() {
	c.saveLastConnectError(nil)
}

func (c *connection) connected() bool {
	return c.lastConnectError() == nil
}

const defaultConnReattemptPeriod = 10 * time.Second

func (c *connection) indefiniteBackgroundConnection() {
	defer func() {
		c.closeBackgroundConnectionDoneCh(c.backgroundConnectionDoneCh)
	}()

	connReattemptPeriod := c.cfg.reconnectionPeriod
	if connReattemptPeriod <= 0 {
		connReattemptPeriod = defaultConnReattemptPeriod
	}

	// No strong seeding required, nano time can
	// already help with pseudo uniqueness.
	rng := rand.New(rand.NewSource(time.Now().UnixNano() + rand.Int63n(1024)))

	// maxJitterNanos: 70% of the connectionReattemptPeriod
	maxJitterNanos := int64(0.7 * float64(connReattemptPeriod))

	for {
		// Otherwise these will be the normal scenarios to enable
		// reconnection if we trip out.
		// 1. If we've stopped, return entirely
		// 2. Otherwise block until we are disconnected, and
		//    then retry connecting
		select {
		case <-c.stopCh:
			return

		case <-c.disconnectedCh:
			// Quickly check if we haven't stopped at the
			// same time.
			select {
			case <-c.stopCh:
				return

			default:
			}

			// Normal scenario that we'll wait for
		}

		if err := c.connect(context.Background()); err == nil {
			c.setStateConnected()
		} else {
			c.setStateDisconnected(err)
		}

		// Apply some jitter to avoid lockstep retrials of other
		// collector-exporters. Lockstep retrials could result in an
		// innocent DDOS, by clogging the machine's resources and network.
		jitter := time.Duration(rng.Int63n(maxJitterNanos))
		select {
		case <-c.stopCh:
			return
		case <-time.After(connReattemptPeriod + jitter):
		}
	}
}

func (c *connection) connect(ctx context.Context) error {
	cc, err := c.dialToCollector(ctx)
	if err != nil {
		return err
	}
	c.setConnection(cc)
	c.newConnectionHandler(cc)
	return nil
}

// setConnection sets cc as the client connection and returns true if
// the connection state changed.
func (c *connection) setConnection(cc *grpc.ClientConn) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If previous clientConn is same as the current then just return.
	// This doesn't happen right now as this func is only called with new ClientConn.
	// It is more about future-proofing.
	if c.cc == cc {
		return false
	}

	// If the previous clientConn was non-nil, close it
	if c.cc != nil {
		_ = c.cc.Close()
	}
	c.cc = cc
	return true
}

func (c *connection) dialToCollector(ctx context.Context) (*grpc.ClientConn, error) {
	endpoint := c.cfg.collectorEndpoint

	dialOpts := []grpc.DialOption{}
	if c.cfg.serviceConfig != "" {
		dialOpts = append(dialOpts, grpc.WithDefaultServiceConfig(c.cfg.serviceConfig))
	}
	if c.cfg.clientCredentials != nil {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(c.cfg.clientCredentials))
	} else if c.cfg.canDialInsecure {
		dialOpts = append(dialOpts, grpc.WithInsecure())
	}
	if c.cfg.compressor != "" {
		dialOpts = append(dialOpts, grpc.WithDefaultCallOptions(grpc.UseCompressor(c.cfg.compressor)))
	}
	if len(c.cfg.dialOptions) != 0 {
		dialOpts = append(dialOpts, c.cfg.dialOptions...)
	}

	ctx, cancel := c.contextWithStop(ctx)
	defer cancel()
	ctx = c.contextWithMetadata(ctx)
	return grpc.DialContext(ctx, endpoint, dialOpts...)
}

func (c *connection) contextWithMetadata(ctx context.Context) context.Context {
	if c.metadata.Len() > 0 {
		return metadata.NewOutgoingContext(ctx, c.metadata)
	}
	return ctx
}

func (c *connection) shutdown(ctx context.Context) error {
	close(c.stopCh)
	// Ensure that the backgroundConnector returns
	select {
	case <-c.backgroundConnectionDoneCh:
	case <-ctx.Done():
		return ctx.Err()
	}

	c.mu.Lock()
	cc := c.cc
	c.cc = nil
	c.mu.Unlock()

	if cc != nil {
		return cc.Close()
	}

	return nil
}

func (c *connection) contextWithStop(ctx context.Context) (context.Context, context.CancelFunc) {
	// Unify the parent context Done signal with the connection's
	// stop channel.
	ctx, cancel := context.WithCancel(ctx)
	go func(ctx context.Context, cancel context.CancelFunc) {
		select {
		case <-ctx.Done():
			// Nothing to do, either cancelled or deadline
			// happened.
		case <-c.stopCh:
			cancel()
		}
	}(ctx, cancel)
	return ctx, cancel
}
