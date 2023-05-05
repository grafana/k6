package fasthttputil

import (
	"errors"
	"net"
	"sync"
)

// ErrInmemoryListenerClosed indicates that the InmemoryListener is already closed.
var ErrInmemoryListenerClosed = errors.New("InmemoryListener is already closed: use of closed network connection")

// InmemoryListener provides in-memory dialer<->net.Listener implementation.
//
// It may be used either for fast in-process client<->server communications
// without network stack overhead or for client<->server tests.
type InmemoryListener struct {
	lock         sync.Mutex
	closed       bool
	conns        chan acceptConn
	listenerAddr net.Addr
	addrLock     sync.RWMutex
}

type acceptConn struct {
	conn     net.Conn
	accepted chan struct{}
}

// NewInmemoryListener returns new in-memory dialer<->net.Listener.
func NewInmemoryListener() *InmemoryListener {
	return &InmemoryListener{
		conns: make(chan acceptConn, 1024),
	}
}

// SetLocalAddr sets the (simulated) local address for the listener.
func (ln *InmemoryListener) SetLocalAddr(localAddr net.Addr) {
	ln.addrLock.Lock()
	defer ln.addrLock.Unlock()

	ln.listenerAddr = localAddr
}

// Accept implements net.Listener's Accept.
//
// It is safe calling Accept from concurrently running goroutines.
//
// Accept returns new connection per each Dial call.
func (ln *InmemoryListener) Accept() (net.Conn, error) {
	c, ok := <-ln.conns
	if !ok {
		return nil, ErrInmemoryListenerClosed
	}
	close(c.accepted)
	return c.conn, nil
}

// Close implements net.Listener's Close.
func (ln *InmemoryListener) Close() error {
	var err error

	ln.lock.Lock()
	if !ln.closed {
		close(ln.conns)
		ln.closed = true
	} else {
		err = ErrInmemoryListenerClosed
	}
	ln.lock.Unlock()
	return err
}

type inmemoryAddr int

func (inmemoryAddr) Network() string {
	return "inmemory"
}

func (inmemoryAddr) String() string {
	return "InmemoryListener"
}

// Addr implements net.Listener's Addr.
func (ln *InmemoryListener) Addr() net.Addr {
	ln.addrLock.RLock()
	defer ln.addrLock.RUnlock()

	if ln.listenerAddr != nil {
		return ln.listenerAddr
	}

	return inmemoryAddr(0)
}

// Dial creates new client<->server connection.
// Just like a real Dial it only returns once the server
// has accepted the connection.
//
// It is safe calling Dial from concurrently running goroutines.
func (ln *InmemoryListener) Dial() (net.Conn, error) {
	return ln.DialWithLocalAddr(nil)
}

// DialWithLocalAddr creates new client<->server connection.
// Just like a real Dial it only returns once the server
// has accepted the connection. The local address of the
// client connection can be set with local.
//
// It is safe calling Dial from concurrently running goroutines.
func (ln *InmemoryListener) DialWithLocalAddr(local net.Addr) (net.Conn, error) {
	pc := NewPipeConns()

	pc.SetAddresses(local, ln.Addr(), ln.Addr(), local)

	cConn := pc.Conn1()
	sConn := pc.Conn2()
	ln.lock.Lock()
	accepted := make(chan struct{})
	if !ln.closed {
		ln.conns <- acceptConn{sConn, accepted}
		// Wait until the connection has been accepted.
		<-accepted
	} else {
		_ = sConn.Close()
		_ = cConn.Close()
		cConn = nil
	}
	ln.lock.Unlock()

	if cConn == nil {
		return nil, ErrInmemoryListenerClosed
	}
	return cConn, nil
}
