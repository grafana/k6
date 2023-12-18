/*
 *
 * Copyright 2017 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package grpc

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/internal/balancer/gracefulswitch"
	"google.golang.org/grpc/internal/channelz"
	"google.golang.org/grpc/internal/grpcsync"
	"google.golang.org/grpc/resolver"
)

// ccBalancerWrapper sits between the ClientConn and the Balancer.
//
// ccBalancerWrapper implements methods corresponding to the ones on the
// balancer.Balancer interface. The ClientConn is free to call these methods
// concurrently and the ccBalancerWrapper ensures that calls from the ClientConn
// to the Balancer happen in order by performing them in the serializer, without
// any mutexes held.
//
// ccBalancerWrapper also implements the balancer.ClientConn interface and is
// passed to the Balancer implementations. It invokes unexported methods on the
// ClientConn to handle these calls from the Balancer.
//
// It uses the gracefulswitch.Balancer internally to ensure that balancer
// switches happen in a graceful manner.
type ccBalancerWrapper struct {
	// The following fields are initialized when the wrapper is created and are
	// read-only afterwards, and therefore can be accessed without a mutex.
	cc               *ClientConn
	opts             balancer.BuildOptions
	serializer       *grpcsync.CallbackSerializer
	serializerCancel context.CancelFunc

	// The following fields are only accessed within the serializer or during
	// initialization.
	curBalancerName string
	balancer        *gracefulswitch.Balancer

	// The following field is protected by mu.  Caller must take cc.mu before
	// taking mu.
	mu     sync.Mutex
	closed bool
}

// newCCBalancerWrapper creates a new balancer wrapper in idle state. The
// underlying balancer is not created until the switchTo() method is invoked.
func newCCBalancerWrapper(cc *ClientConn) *ccBalancerWrapper {
	ctx, cancel := context.WithCancel(cc.ctx)
	ccb := &ccBalancerWrapper{
		cc: cc,
		opts: balancer.BuildOptions{
			DialCreds:        cc.dopts.copts.TransportCredentials,
			CredsBundle:      cc.dopts.copts.CredsBundle,
			Dialer:           cc.dopts.copts.Dialer,
			Authority:        cc.authority,
			CustomUserAgent:  cc.dopts.copts.UserAgent,
			ChannelzParentID: cc.channelzID,
			Target:           cc.parsedTarget,
		},
		serializer:       grpcsync.NewCallbackSerializer(ctx),
		serializerCancel: cancel,
	}
	ccb.balancer = gracefulswitch.NewBalancer(ccb, ccb.opts)
	return ccb
}

// updateClientConnState is invoked by grpc to push a ClientConnState update to
// the underlying balancer.  This is always executed from the serializer, so
// it is safe to call into the balancer here.
func (ccb *ccBalancerWrapper) updateClientConnState(ccs *balancer.ClientConnState) error {
	errCh := make(chan error)
	ok := ccb.serializer.Schedule(func(ctx context.Context) {
		defer close(errCh)
		if ctx.Err() != nil || ccb.balancer == nil {
			return
		}
		err := ccb.balancer.UpdateClientConnState(*ccs)
		if logger.V(2) && err != nil {
			logger.Infof("error from balancer.UpdateClientConnState: %v", err)
		}
		errCh <- err
	})
	if !ok {
		return nil
	}
	return <-errCh
}

// resolverError is invoked by grpc to push a resolver error to the underlying
// balancer.  The call to the balancer is executed from the serializer.
func (ccb *ccBalancerWrapper) resolverError(err error) {
	ccb.serializer.Schedule(func(ctx context.Context) {
		if ctx.Err() != nil || ccb.balancer == nil {
			return
		}
		ccb.balancer.ResolverError(err)
	})
}

// switchTo is invoked by grpc to instruct the balancer wrapper to switch to the
// LB policy identified by name.
//
// ClientConn calls newCCBalancerWrapper() at creation time. Upon receipt of the
// first good update from the name resolver, it determines the LB policy to use
// and invokes the switchTo() method. Upon receipt of every subsequent update
// from the name resolver, it invokes this method.
//
// the ccBalancerWrapper keeps track of the current LB policy name, and skips
// the graceful balancer switching process if the name does not change.
func (ccb *ccBalancerWrapper) switchTo(name string) {
	ccb.serializer.Schedule(func(ctx context.Context) {
		if ctx.Err() != nil || ccb.balancer == nil {
			return
		}
		// TODO: Other languages use case-sensitive balancer registries. We should
		// switch as well. See: https://github.com/grpc/grpc-go/issues/5288.
		if strings.EqualFold(ccb.curBalancerName, name) {
			return
		}
		ccb.buildLoadBalancingPolicy(name)
	})
}

// buildLoadBalancingPolicy performs the following:
//   - retrieve a balancer builder for the given name. Use the default LB
//     policy, pick_first, if no LB policy with name is found in the registry.
//   - instruct the gracefulswitch balancer to switch to the above builder. This
//     will actually build the new balancer.
//   - update the `curBalancerName` field
//
// Must be called from a serializer callback.
func (ccb *ccBalancerWrapper) buildLoadBalancingPolicy(name string) {
	builder := balancer.Get(name)
	if builder == nil {
		channelz.Warningf(logger, ccb.cc.channelzID, "Channel switches to new LB policy %q, since the specified LB policy %q was not registered", PickFirstBalancerName, name)
		builder = newPickfirstBuilder()
	} else {
		channelz.Infof(logger, ccb.cc.channelzID, "Channel switches to new LB policy %q", name)
	}

	if err := ccb.balancer.SwitchTo(builder); err != nil {
		channelz.Errorf(logger, ccb.cc.channelzID, "Channel failed to build new LB policy %q: %v", name, err)
		return
	}
	ccb.curBalancerName = builder.Name()
}

// close initiates async shutdown of the wrapper.  cc.mu must be held when
// calling this function.  To determine the wrapper has finished shutting down,
// the channel should block on ccb.serializer.Done() without cc.mu held.
func (ccb *ccBalancerWrapper) close() {
	ccb.mu.Lock()
	ccb.closed = true
	ccb.mu.Unlock()
	channelz.Info(logger, ccb.cc.channelzID, "ccBalancerWrapper: closing")
	ccb.serializer.Schedule(func(context.Context) {
		if ccb.balancer == nil {
			return
		}
		ccb.balancer.Close()
		ccb.balancer = nil
	})
	ccb.serializerCancel()
}

// exitIdle invokes the balancer's exitIdle method in the serializer.
func (ccb *ccBalancerWrapper) exitIdle() {
	ccb.serializer.Schedule(func(ctx context.Context) {
		if ctx.Err() != nil || ccb.balancer == nil {
			return
		}
		ccb.balancer.ExitIdle()
	})
}

func (ccb *ccBalancerWrapper) NewSubConn(addrs []resolver.Address, opts balancer.NewSubConnOptions) (balancer.SubConn, error) {
	ccb.cc.mu.Lock()
	defer ccb.cc.mu.Unlock()

	ccb.mu.Lock()
	if ccb.closed {
		ccb.mu.Unlock()
		return nil, fmt.Errorf("balancer is being closed; no new SubConns allowed")
	}
	ccb.mu.Unlock()

	if len(addrs) == 0 {
		return nil, fmt.Errorf("grpc: cannot create SubConn with empty address list")
	}
	ac, err := ccb.cc.newAddrConnLocked(addrs, opts)
	if err != nil {
		channelz.Warningf(logger, ccb.cc.channelzID, "acBalancerWrapper: NewSubConn: failed to newAddrConn: %v", err)
		return nil, err
	}
	acbw := &acBalancerWrapper{
		ccb:           ccb,
		ac:            ac,
		producers:     make(map[balancer.ProducerBuilder]*refCountedProducer),
		stateListener: opts.StateListener,
	}
	ac.acbw = acbw
	return acbw, nil
}

func (ccb *ccBalancerWrapper) RemoveSubConn(sc balancer.SubConn) {
	// The graceful switch balancer will never call this.
	logger.Errorf("ccb RemoveSubConn(%v) called unexpectedly, sc")
}

func (ccb *ccBalancerWrapper) UpdateAddresses(sc balancer.SubConn, addrs []resolver.Address) {
	acbw, ok := sc.(*acBalancerWrapper)
	if !ok {
		return
	}
	acbw.UpdateAddresses(addrs)
}

func (ccb *ccBalancerWrapper) UpdateState(s balancer.State) {
	ccb.cc.mu.Lock()
	defer ccb.cc.mu.Unlock()

	ccb.mu.Lock()
	if ccb.closed {
		ccb.mu.Unlock()
		return
	}
	ccb.mu.Unlock()
	// Update picker before updating state.  Even though the ordering here does
	// not matter, it can lead to multiple calls of Pick in the common start-up
	// case where we wait for ready and then perform an RPC.  If the picker is
	// updated later, we could call the "connecting" picker when the state is
	// updated, and then call the "ready" picker after the picker gets updated.

	// Note that there is no need to check if the balancer wrapper was closed,
	// as we know the graceful switch LB policy will not call cc if it has been
	// closed.
	ccb.cc.pickerWrapper.updatePicker(s.Picker)
	ccb.cc.csMgr.updateState(s.ConnectivityState)
}

func (ccb *ccBalancerWrapper) ResolveNow(o resolver.ResolveNowOptions) {
	ccb.cc.mu.RLock()
	defer ccb.cc.mu.RUnlock()

	ccb.mu.Lock()
	if ccb.closed {
		ccb.mu.Unlock()
		return
	}
	ccb.mu.Unlock()
	ccb.cc.resolveNowLocked(o)
}

func (ccb *ccBalancerWrapper) Target() string {
	return ccb.cc.target
}

// acBalancerWrapper is a wrapper on top of ac for balancers.
// It implements balancer.SubConn interface.
type acBalancerWrapper struct {
	ac            *addrConn          // read-only
	ccb           *ccBalancerWrapper // read-only
	stateListener func(balancer.SubConnState)

	mu        sync.Mutex
	producers map[balancer.ProducerBuilder]*refCountedProducer
}

// updateState is invoked by grpc to push a subConn state update to the
// underlying balancer.
func (acbw *acBalancerWrapper) updateState(s connectivity.State, err error) {
	acbw.ccb.serializer.Schedule(func(ctx context.Context) {
		if ctx.Err() != nil || acbw.ccb.balancer == nil {
			return
		}
		// Even though it is optional for balancers, gracefulswitch ensures
		// opts.StateListener is set, so this cannot ever be nil.
		// TODO: delete this comment when UpdateSubConnState is removed.
		acbw.stateListener(balancer.SubConnState{ConnectivityState: s, ConnectionError: err})
	})
}

func (acbw *acBalancerWrapper) String() string {
	return fmt.Sprintf("SubConn(id:%d)", acbw.ac.channelzID.Int())
}

func (acbw *acBalancerWrapper) UpdateAddresses(addrs []resolver.Address) {
	acbw.ac.updateAddrs(addrs)
}

func (acbw *acBalancerWrapper) Connect() {
	go acbw.ac.connect()
}

func (acbw *acBalancerWrapper) Shutdown() {
	acbw.ccb.cc.removeAddrConn(acbw.ac, errConnDrain)
}

// NewStream begins a streaming RPC on the addrConn.  If the addrConn is not
// ready, blocks until it is or ctx expires.  Returns an error when the context
// expires or the addrConn is shut down.
func (acbw *acBalancerWrapper) NewStream(ctx context.Context, desc *StreamDesc, method string, opts ...CallOption) (ClientStream, error) {
	transport, err := acbw.ac.getTransport(ctx)
	if err != nil {
		return nil, err
	}
	return newNonRetryClientStream(ctx, desc, method, transport, acbw.ac, opts...)
}

// Invoke performs a unary RPC.  If the addrConn is not ready, returns
// errSubConnNotReady.
func (acbw *acBalancerWrapper) Invoke(ctx context.Context, method string, args any, reply any, opts ...CallOption) error {
	cs, err := acbw.NewStream(ctx, unaryStreamDesc, method, opts...)
	if err != nil {
		return err
	}
	if err := cs.SendMsg(args); err != nil {
		return err
	}
	return cs.RecvMsg(reply)
}

type refCountedProducer struct {
	producer balancer.Producer
	refs     int    // number of current refs to the producer
	close    func() // underlying producer's close function
}

func (acbw *acBalancerWrapper) GetOrBuildProducer(pb balancer.ProducerBuilder) (balancer.Producer, func()) {
	acbw.mu.Lock()
	defer acbw.mu.Unlock()

	// Look up existing producer from this builder.
	pData := acbw.producers[pb]
	if pData == nil {
		// Not found; create a new one and add it to the producers map.
		p, close := pb.Build(acbw)
		pData = &refCountedProducer{producer: p, close: close}
		acbw.producers[pb] = pData
	}
	// Account for this new reference.
	pData.refs++

	// Return a cleanup function wrapped in a OnceFunc to remove this reference
	// and delete the refCountedProducer from the map if the total reference
	// count goes to zero.
	unref := func() {
		acbw.mu.Lock()
		pData.refs--
		if pData.refs == 0 {
			defer pData.close() // Run outside the acbw mutex
			delete(acbw.producers, pb)
		}
		acbw.mu.Unlock()
	}
	return pData.producer, grpcsync.OnceFunc(unref)
}
