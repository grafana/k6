package tcp

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"time"

	"github.com/grafana/sobek"
	extensionapi "go.k6.io/k6-extension-api"
)

const defaultHost = "localhost"

type connectOptions struct {
	Port int               `js:"port"`
	Host string            `js:"host"`
	TLS  bool              `js:"tls"`
	Tags map[string]string `js:"tags"`
}

func (co *connectOptions) address() string {
	return net.JoinHostPort(co.Host, strconv.Itoa(co.Port))
}

func (s *socket) connect(portOrOptions sobek.Value, hostOrEmpty sobek.Value) (*sobek.Promise, error) {
	promise, resolver := newPromise(s.vu)

	if err := s.connectPrepare(portOrOptions, hostOrEmpty); err != nil {
		s.rejectWithTCPError(resolver, err, "connect", s.tags())

		return promise, nil
	}

	go func() {
		if err := s.connectExecute(); err != nil {
			s.rejectWithTCPError(resolver, err, "connect", s.tags())

			return
		}

		resolver.Resolve(nil)
	}()

	return promise, nil
}

func (s *socket) connectPrepare(portOrOptions sobek.Value, hostOrEmpty sobek.Value) error {
	var opts *connectOptions

	switch portOrOptions.ExportType() {
	case reflect.TypeFor[int64](), reflect.TypeFor[string]():
		opts = &connectOptions{
			Port: int(portOrOptions.ToInteger()),
			Host: defaultHost,
		}

	case reflect.TypeFor[map[string]any]():
		if err := s.vu.Runtime().ExportTo(portOrOptions, &opts); err != nil {
			return err
		}

		if len(opts.Host) == 0 {
			opts.Host = defaultHost
		}

		hostOrEmpty = nil

	default:
		return fmt.Errorf("%w: expected integer or object", errInvalidType)
	}

	if hostOrEmpty != nil && !sobek.IsUndefined(hostOrEmpty) && !sobek.IsNull(hostOrEmpty) {
		opts.Host = hostOrEmpty.String()
	}

	s.connectOpts = opts

	return nil
}

func (s *socket) connectExecute() error {
	s.mu.Lock()

	s.log.Debug("Connecting to TCP server", "address", s.connectOpts.address())

	tags := s.tags()

	s.state = socketStateOpening

	err := s.addDurationMetricsFor(s.metrics.tcpResolving, tags, s.resolve)
	if err != nil {
		s.state = socketStateDisconnected
		s.mu.Unlock()

		return err
	}

	err = s.addDurationMetricsFor(s.metrics.tcpConnecting, tags, s.dial)
	if err != nil {
		s.state = socketStateDisconnected
		s.mu.Unlock()

		return err
	}

	// Release mutex before firing events and starting read goroutine
	s.mu.Unlock()

	// Queue connect before the read goroutine can enqueue later lifecycle events.
	s.fire("connect")

	// Start read goroutine after connect has been queued.
	go s.read()

	s.addCounterMetrics(s.metrics.tcpSockets, tags)

	return nil
}

func (s *socket) resolve() error {
	network, err := networkFor(s.vu)
	if err != nil {
		return err
	}
	ips, err := network.LookupHost(s.vu.Context(), s.connectOpts.Host)
	if err != nil {
		return err
	}
	if len(ips) == 0 {
		return fmt.Errorf("TCP resolver returned no addresses for %q", s.connectOpts.Host)
	}
	ip := net.ParseIP(ips[0])
	if ip == nil {
		return fmt.Errorf("TCP resolver returned an invalid address %q", ips[0])
	}

	s.endpoints.remoteIP = ip.String()
	s.endpoints.remotePort = s.connectOpts.Port
	s.endpoints.remoteAddr = net.JoinHostPort(s.endpoints.remoteIP, strconv.Itoa(s.connectOpts.Port))

	return nil
}

func (s *socket) dial() error {
	network, err := networkFor(s.vu)
	if err != nil {
		return err
	}
	conn, err := network.DialContext(s.vu.Context(), "tcp", s.endpoints.remoteAddr)
	if err != nil {
		return err
	}

	// Wrap with TLS if enabled
	if s.connectOpts.TLS {
		tlsConn, err := s.wrapTLS(conn)
		if err != nil {
			return err
		}

		conn = tlsConn
	}

	localAddr := conn.LocalAddr()
	if tcpAddr, ok := localAddr.(*net.TCPAddr); ok {
		s.endpoints.localIP = tcpAddr.IP.String()
		s.endpoints.localPort = tcpAddr.Port
	}

	s.state = socketStateOpen
	s.conn = conn
	s.connectTime = time.Now()

	// Set read deadline if timeout is configured
	if s.timeout > 0 {
		if err := conn.SetReadDeadline(s.connectTime.Add(s.timeout)); err != nil {
			return err
		}
	}

	return nil
}

func (s *socket) wrapTLS(conn net.Conn) (net.Conn, error) {
	tlsCapability, ok := s.vu.(extensionapi.TLS)
	if !ok {
		_ = conn.Close()
		return nil, errors.New("TCP TLS connections are not allowed in the init context")
	}
	tlsConn, err := tlsCapability.TLSClient(s.vu.Context(), conn, &tls.Config{
		ServerName: s.connectOpts.Host,
		NextProtos: []string{"http/1.1"},
	})
	if err != nil {
		return nil, fmt.Errorf("TLS handshake failed: %w", err)
	}
	s.log.Debug("TLS handshake completed", "address", s.endpoints.remoteAddr)
	return tlsConn, nil
}

// destroy closes the connection and cleans up resources.
// Safe to call multiple times - cleanup happens exactly once.
func (s *socket) destroy() {
	s.destroyOnce.Do(func() {
		// Close connection and update state
		s.mu.Lock()
		s.state = socketStateDestroyed
		conn := s.conn
		s.conn = nil
		duration := time.Since(s.connectTime)
		tags := s.tags()
		s.mu.Unlock()

		// Close connection outside lock
		if conn != nil {
			_ = conn.Close()

			s.addDurationMetrics(duration, s.metrics.tcpDuration, tags)
		}

		// Fire close event
		s.fire("close")

		// Cancel context to signal loops to stop
		s.cancel()
	})
}
