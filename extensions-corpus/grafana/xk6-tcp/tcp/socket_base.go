package tcp

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/grafana/sobek"
	"github.com/sirupsen/logrus"
	"go.k6.io/k6/v2/js/common"
	"go.k6.io/k6/v2/js/modules"
	"go.k6.io/k6/v2/lib"
)

var errInvalidType = errors.New("invalid type")

type socketOptions struct {
	Tags map[string]string
}

type socketEndpoints struct {
	remotePort int
	remoteIP   string
	remoteAddr string
	localPort  int
	localIP    string
}

type socket struct {
	this *sobek.Object

	conn net.Conn

	log logrus.FieldLogger

	socketOpts  *socketOptions
	connectOpts *connectOptions

	handlers sync.Map

	vu       modules.VU
	callChan chan func() error
	cancel   context.CancelFunc

	metrics *tcpMetrics

	connectTime time.Time

	endpoints socketEndpoints

	totalWritten int64
	totalRead    int64

	state socketState

	timeout time.Duration

	mu sync.RWMutex

	// readBuf is reused for each read operation to avoid allocations
	readBuf [4096]byte

	// bufferPool provides pooled buffers for event data to reduce GC pressure
	bufferPool *lib.BufferPool

	// destroyOnce ensures cleanup happens exactly once
	destroyOnce sync.Once
}

func newSocket(log logrus.FieldLogger, vu modules.VU, metrics *tcpMetrics) *socket {
	s := new(socket)
	s.log = log
	s.vu = vu
	s.callChan = make(chan func() error)

	s.metrics = metrics
	s.state = socketStateDisconnected
	s.bufferPool = lib.NewBufferPool()

	return s
}

func (m *module) socket(call sobek.ConstructorCall) *sobek.Object {
	toValue := m.vu.Runtime().ToValue

	s := newSocket(m.log, m.vu, m.metrics)
	s.this = call.This
	must := func(err error) {
		if err != nil {
			common.Throw(m.vu.Runtime(), err)
		}
	}

	s.socketOpts = new(socketOptions)

	if len(call.Arguments) > 0 {
		must(m.vu.Runtime().ExportTo(call.Arguments[0], &s.socketOpts))
	}

	must(s.this.Set("connect", toValue(s.connect)))
	must(s.this.Set("write", toValue(s.write)))
	must(s.this.Set("destroy", toValue(s.destroy)))
	must(s.this.Set("setTimeout", toValue(s.setTimeout)))
	must(s.this.Set("on", toValue(s.on)))

	must(s.this.DefineAccessorProperty("ready_state", toValue(s.readyState), nil, sobek.FLAG_FALSE, sobek.FLAG_FALSE))
	must(s.this.DefineAccessorProperty("bytes_written", toValue(s.bytesWritten), nil, sobek.FLAG_FALSE, sobek.FLAG_FALSE))
	must(s.this.DefineAccessorProperty("bytes_read", toValue(s.bytesRead), nil, sobek.FLAG_FALSE, sobek.FLAG_FALSE))
	must(s.this.DefineAccessorProperty("local_ip", toValue(s.localIP), nil, sobek.FLAG_FALSE, sobek.FLAG_FALSE))
	must(s.this.DefineAccessorProperty("local_port", toValue(s.localPort), nil, sobek.FLAG_FALSE, sobek.FLAG_FALSE))
	must(s.this.DefineAccessorProperty("remote_ip", toValue(s.remoteIP), nil, sobek.FLAG_FALSE, sobek.FLAG_FALSE))
	must(s.this.DefineAccessorProperty("remote_port", toValue(s.remotePort), nil, sobek.FLAG_FALSE, sobek.FLAG_FALSE))
	must(s.this.DefineAccessorProperty("connected", toValue(s.isConnected), nil, sobek.FLAG_FALSE, sobek.FLAG_FALSE))

	// Create a cancellable context for this socket's lifecycle
	ctx, cancel := context.WithCancel(m.vu.Context())
	s.cancel = cancel

	go s.loop(ctx)

	return nil
}

func (s *socket) bytesWritten() int64 {
	return atomic.LoadInt64(&s.totalWritten)
}

func (s *socket) bytesRead() int64 {
	return atomic.LoadInt64(&s.totalRead)
}

func (s *socket) localIP() sobek.Value {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.conn == nil {
		return sobek.Undefined()
	}

	return s.vu.Runtime().ToValue(s.endpoints.localIP)
}

func (s *socket) localPort() sobek.Value {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.conn == nil {
		return sobek.Undefined()
	}

	return s.vu.Runtime().ToValue(s.endpoints.localPort)
}

func (s *socket) remoteIP() sobek.Value {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.conn == nil {
		return sobek.Undefined()
	}

	return s.vu.Runtime().ToValue(s.endpoints.remoteIP)
}

func (s *socket) remotePort() sobek.Value {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.conn == nil {
		return sobek.Undefined()
	}

	return s.vu.Runtime().ToValue(s.endpoints.remotePort)
}
