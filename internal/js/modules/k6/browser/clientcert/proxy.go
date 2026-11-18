package clientcert

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"math/big"
	"net"
	"strconv"
	"sync"
	"time"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/log"
)

const (
	// tlsRecordTypeHandshake is the first byte of a TLS ClientHello record.
	tlsRecordTypeHandshake = 0x16

	// firstByteTimeout bounds the wait for the browser's first byte before the connection is
	// tunnelled without inspection.
	firstByteTimeout = 30 * time.Second

	// defaultReplayLimit bounds the h2 bytes recorded before the first upstream byte.
	defaultReplayLimit = 128 << 10

	proxyCertificateCommonName = "k6 client certificate proxy"
)

// dummyCertificate is the certificate the proxy presents to the browser for intercepted
// connections. The browser ignores certificate errors when client certificates are set.
//
//nolint:gochecknoglobals,lll // one throwaway self-signed certificate per process, as Playwright does; regenerating it per context would cost a key generation per iteration
var dummyCertificate = sync.OnceValues(func() (tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, err
	}
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: proxyCertificateCommonName},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.AddDate(1, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, key.Public(), key)
	if err != nil {
		return tls.Certificate{}, err
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}, nil
})

// ProxyConfig configures a Proxy.
type ProxyConfig struct {
	// Certificates maps a normalized origin to its client certificates in load order
	// (PEM entries in option order, then PFX entries). At most one per key type.
	Certificates map[string][]tls.Certificate
	// IgnoreHTTPSErrors skips verification of the servers' certificates.
	IgnoreHTTPSErrors bool
	// RootCAs verifies servers' certificates; nil means the system roots.
	RootCAs *x509.CertPool
}

// Proxy presents client certificates on the browser's behalf.
type Proxy struct {
	cfg    ProxyConfig
	dummy  tls.Certificate
	server *SOCKS5Server
	logger *log.Logger
}

// NewProxy starts a Proxy. dial opens destination connections.
func NewProxy(cfg ProxyConfig, dial DialFunc, logger *log.Logger) (*Proxy, error) {
	dummy, err := dummyCertificate()
	if err != nil {
		return nil, fmt.Errorf("generating the client certificate proxy's certificate: %w", err)
	}
	p := &Proxy{cfg: cfg, dummy: dummy, logger: logger}
	server, err := ListenSOCKS5(dial, p.tunnel, logger)
	if err != nil {
		return nil, err
	}
	p.server = server
	return p, nil
}

// Addr returns the SOCKS5 host:port.
func (p *Proxy) Addr() string {
	return p.server.Addr()
}

// Close closes the SOCKS5 server.
func (p *Proxy) Close() error {
	return p.server.Close()
}

// tunnel intercepts TLS connections to origins with client certificates and pipes everything
// else untouched.
func (p *Proxy) tunnel(ctx context.Context, browser, destination net.Conn, host string, port int) {
	peeked := newPeekConn(browser)
	first, err := peeked.peekFirstByte(firstByteTimeout)
	var netErr net.Error
	if err != nil && (!errors.As(err, &netErr) || !netErr.Timeout()) {
		p.logger.Debugf("Proxy:tunnel", "host:%s port:%d err:%v", host, port, err)
		_ = browser.Close()
		_ = destination.Close()
		return
	}

	certs := p.certificatesFor(host, port)
	intercept := err == nil && first == tlsRecordTypeHandshake && len(certs) > 0
	p.logger.Debugf("Proxy:tunnel", "host:%s port:%d intercept:%t", host, port, intercept)
	if !intercept {
		Pipe(ctx, peeked, destination, host, port)
		return
	}
	p.intercept(ctx, peeked, destination, host, certs)
}

func (p *Proxy) certificatesFor(host string, port int) []tls.Certificate {
	origin, err := NormalizeOrigin("https://" + net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return nil
	}
	return p.cfg.Certificates[origin]
}

// intercept terminates the browser's TLS with the dummy certificate and re-originates it to
// the destination with a client certificate, carrying the browser's ALPN.
func (p *Proxy) intercept(ctx context.Context, browserRaw, upstreamRaw net.Conn, host string, certs []tls.Certificate) {
	var (
		upstream    *tls.Conn
		upstreamErr error
	)
	browser := tls.Server(browserRaw, &tls.Config{
		GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
			upstream, upstreamErr = p.handshakeUpstream(hello, upstreamRaw, host, certs)
			protocol := upstream.ConnectionState().NegotiatedProtocol
			if protocol == "" {
				protocol = "http/1.1"
			}
			return &tls.Config{Certificates: []tls.Certificate{p.dummy}, NextProtos: []string{protocol}}, nil
		},
	})
	if err := browser.HandshakeContext(ctx); err != nil {
		p.logger.Debugf("Proxy:intercept", "host:%s browser handshake err:%v", host, err)
		_ = browserRaw.Close()
		_ = upstreamRaw.Close()
		return
	}

	protocol := browser.ConnectionState().NegotiatedProtocol
	if upstreamErr != nil {
		p.logger.Debugf("Proxy:upstream", "host:%s err:%v", host, upstreamErr)
		if protocol == "h2" {
			serve503H2(browser, browserRaw, nil, upstreamErr)
		} else {
			write503(browser, browserRaw, true, upstreamErr)
		}
		_ = browserRaw.Close()
		_ = upstreamRaw.Close()
		return
	}
	relay(ctx, browser, browserRaw, upstream, upstreamRaw, protocol, defaultReplayLimit)
}

// handshakeUpstream performs the TLS handshake with the destination, offering the browser's
// ALPN protocols and presenting a client certificate. It returns the connection even when the
// handshake fails, so that its negotiated protocol can still be read.
func (p *Proxy) handshakeUpstream(
	hello *tls.ClientHelloInfo, conn net.Conn, host string, certs []tls.Certificate,
) (*tls.Conn, error) {
	protos := hello.SupportedProtos
	if len(protos) == 0 {
		protos = []string{"http/1.1"}
	}
	upstream := tls.Client(conn, &tls.Config{
		ServerName:           host,
		NextProtos:           protos,
		RootCAs:              p.cfg.RootCAs,
		InsecureSkipVerify:   p.cfg.IgnoreHTTPSErrors, //nolint:gosec // the user opted out with ignoreHTTPSErrors
		GetClientCertificate: selectClientCertificate(certs),
	})
	return upstream, upstream.HandshakeContext(hello.Context())
}

// selectClientCertificate picks the certificate to send. For TLS 1.3 it returns the first
// certificate usable with the earliest scheme in the server's order; for TLS 1.2 it considers
// only the last certificate. The server's CA list is ignored. With no usable certificate it
// returns an empty certificate, so that none is sent.
func selectClientCertificate(certs []tls.Certificate) func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
	return func(cri *tls.CertificateRequestInfo) (*tls.Certificate, error) {
		req := *cri
		req.AcceptableCAs = nil
		if req.Version >= tls.VersionTLS13 {
			for _, scheme := range cri.SignatureSchemes {
				req.SignatureSchemes = []tls.SignatureScheme{scheme}
				for i := range certs {
					if req.SupportsCertificate(&certs[i]) == nil {
						return &certs[i], nil
					}
				}
			}
			return &tls.Certificate{}, nil
		}
		if len(certs) > 0 {
			if last := &certs[len(certs)-1]; req.SupportsCertificate(last) == nil {
				return last, nil
			}
		}
		return &tls.Certificate{}, nil
	}
}

// peekConn lets the first bytes of a connection be inspected without consuming them.
type peekConn struct {
	net.Conn
	r *bufio.Reader
}

func newPeekConn(conn net.Conn) *peekConn {
	return &peekConn{Conn: conn, r: bufio.NewReader(conn)}
}

// peekFirstByte returns the connection's first byte, waiting at most timeout.
func (c *peekConn) peekFirstByte(timeout time.Duration) (byte, error) {
	if err := c.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return 0, err
	}
	b, err := c.r.Peek(1)
	if err != nil {
		// Peek keeps the error for the next Read; nothing was buffered, so start afresh.
		c.r.Reset(c.Conn)
	}
	if derr := c.SetReadDeadline(time.Time{}); err == nil {
		err = derr
	}
	if err != nil {
		return 0, err
	}
	return b[0], nil
}

func (c *peekConn) Read(b []byte) (int, error) {
	return c.r.Read(b)
}

// CloseWrite half-closes the underlying connection when it supports it.
func (c *peekConn) CloseWrite() error {
	if cw, ok := c.Conn.(interface{ CloseWrite() error }); ok {
		return cw.CloseWrite()
	}
	return errors.ErrUnsupported
}
