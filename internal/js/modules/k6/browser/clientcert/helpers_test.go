package clientcert

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"io"
	stdlog "log"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/log"
)

// testCA issues certificates for tests.
type testCA struct {
	cert *x509.Certificate
	key  crypto.Signer
	pool *x509.CertPool
}

func newTestCA(t *testing.T, cn string) *testCA {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber:          testSerial(t),
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, key.Public(), key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	pool := x509.NewCertPool()
	pool.AddCert(cert)
	return &testCA{cert: cert, key: key, pool: pool}
}

// issue returns a certificate for cn with a key of the given kind, valid for loopback hosts.
func (ca *testCA) issue(t *testing.T, cn string, kind keyKind, usage x509.ExtKeyUsage) tls.Certificate {
	t.Helper()
	priv := testSigner(t, kind)
	tmpl := &x509.Certificate{
		SerialNumber: testSerial(t),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{usage},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.cert, priv.Public(), ca.key)
	require.NoError(t, err)
	leaf, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: priv, Leaf: leaf}
}

func testSigner(t *testing.T, kind keyKind) crypto.Signer {
	t.Helper()
	switch kind {
	case rsaPKCS1, rsaPKCS8:
		k, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		return k
	case ed25519PKCS8:
		_, k, err := ed25519.GenerateKey(rand.Reader)
		require.NoError(t, err)
		return k
	default:
		k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)
		return k
	}
}

func testSerial(t *testing.T) *big.Int {
	t.Helper()
	n, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 64))
	require.NoError(t, err)
	return n
}

type serverOptions struct {
	h2         bool
	clientAuth tls.ClientAuthType
	clientCAs  *x509.CertPool
	minVersion uint16
	maxVersion uint16
}

// startTLSServer starts an HTTPS server with a certificate issued by ca. The handler reports
// the client certificate CN in X-Client-Cn and the request's HTTP major version in
// X-Proto-Major, and echoes the request body.
func startTLSServer(t *testing.T, ca *testCA, opts serverOptions) *httptest.Server {
	t.Helper()
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
			w.Header().Set("X-Client-Cn", r.TLS.PeerCertificates[0].Subject.CommonName)
		}
		w.Header().Set("X-Proto-Major", strconv.Itoa(r.ProtoMajor))
		_, _ = io.Copy(w, r.Body)
	}))
	srv.Config.ErrorLog = stdlog.New(io.Discard, "", 0)
	srv.EnableHTTP2 = opts.h2
	srv.TLS = &tls.Config{
		Certificates: []tls.Certificate{ca.issue(t, "server", ecdsaSEC1, x509.ExtKeyUsageServerAuth)},
		ClientAuth:   opts.clientAuth,
		ClientCAs:    opts.clientCAs,
		MinVersion:   opts.minVersion,
		MaxVersion:   opts.maxVersion,
	}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return srv
}

func startProxy(t *testing.T, cfg ProxyConfig) *Proxy {
	t.Helper()
	var d net.Dialer
	p, err := NewProxy(cfg, d.DialContext, log.NewNullLogger())
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Close() })
	return p
}

// newProxyClient returns a client that connects through p the way Chromium does, ignoring
// the proxy's certificate.
func newProxyClient(t *testing.T, p *Proxy, h2 bool) *http.Client {
	t.Helper()
	tr := &http.Transport{
		Proxy:             http.ProxyURL(&url.URL{Scheme: "socks5", Host: p.Addr()}),
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
		ForceAttemptHTTP2: h2,
	}
	if !h2 {
		tr.TLSNextProto = map[string]func(string, *tls.Conn) http.RoundTripper{}
	}
	t.Cleanup(tr.CloseIdleConnections)
	return &http.Client{Transport: tr, Timeout: 10 * time.Second}
}

// testResponse is the part of an HTTP response the tests inspect.
type testResponse struct {
	StatusCode int
	ProtoMajor int
	Header     http.Header
	TLS        *tls.ConnectionState
	Body       string
}

// doRequest sends a request and returns its response.
func doRequest(t *testing.T, client *http.Client, method, target string, body io.Reader) testResponse {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, target, body)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return testResponse{
		StatusCode: resp.StatusCode,
		ProtoMajor: resp.ProtoMajor,
		Header:     resp.Header,
		TLS:        resp.TLS,
		Body:       string(b),
	}
}

// startHTTPServer starts a plain HTTP server that echoes the request body and returns its URL.
func startHTTPServer(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(w, r.Body)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func originOf(t *testing.T, rawURL string) string {
	t.Helper()
	o, err := NormalizeOrigin(rawURL)
	require.NoError(t, err)
	return o
}

// startEchoAfterEOFServer listens on 127.0.0.1 and answers each connection with "response:"
// followed by everything it read, once the client has half-closed.
func startEchoAfterEOFServer(t *testing.T) string {
	t.Helper()
	var lc net.ListenConfig
	l, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	var wg sync.WaitGroup
	t.Cleanup(func() {
		_ = l.Close()
		wg.Wait()
	})
	wg.Go(func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			wg.Go(func() {
				defer func() { _ = c.Close() }()
				req, err := io.ReadAll(c)
				if err != nil {
					return
				}
				_, _ = c.Write(append([]byte("response:"), req...))
			})
		}
	})
	return l.Addr().String()
}

// tlsPipe returns the client and server ends of an in-memory TLS connection that negotiated
// protocol, and the server's underlying connection. The server uses the proxy's certificate.
func tlsPipe(t *testing.T, protocol string) (*tls.Conn, *tls.Conn, net.Conn) {
	t.Helper()
	c, s := net.Pipe()
	t.Cleanup(func() {
		_ = c.Close()
		_ = s.Close()
	})
	cert, err := dummyCertificate()
	require.NoError(t, err)
	server := tls.Server(s, &tls.Config{Certificates: []tls.Certificate{cert}, NextProtos: []string{protocol}})
	client := tls.Client(c, &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{protocol},
	})

	serverErr := make(chan error, 1)
	go func() { serverErr <- server.HandshakeContext(t.Context()) }()
	require.NoError(t, client.HandshakeContext(t.Context()))
	require.NoError(t, <-serverErr)
	require.Equal(t, protocol, client.ConnectionState().NegotiatedProtocol)
	return client, server, s
}

// fakeConn is a net.Conn whose Read and Write are set per test. Without a read function,
// Read blocks until Close; without a write function, Write succeeds until Close.
type fakeConn struct {
	read  func(b []byte) (int, error)
	write func(b []byte) (int, error)

	closeOnce      sync.Once
	closed         chan struct{}
	closeWriteOnce sync.Once
	closedWrite    chan struct{}
}

func newFakeConn() *fakeConn {
	return &fakeConn{closed: make(chan struct{}), closedWrite: make(chan struct{})}
}

func (c *fakeConn) Read(b []byte) (int, error) {
	if c.read != nil {
		return c.read(b)
	}
	<-c.closed
	return 0, net.ErrClosed
}

func (c *fakeConn) Write(b []byte) (int, error) {
	if c.write != nil {
		return c.write(b)
	}
	select {
	case <-c.closed:
		return 0, net.ErrClosed
	default:
		return len(b), nil
	}
}

func (c *fakeConn) Close() error {
	c.closeOnce.Do(func() { close(c.closed) })
	return nil
}

func (c *fakeConn) CloseWrite() error {
	c.closeWriteOnce.Do(func() { close(c.closedWrite) })
	return nil
}

func (c *fakeConn) isClosed() bool {
	select {
	case <-c.closed:
		return true
	default:
		return false
	}
}

func (c *fakeConn) LocalAddr() net.Addr              { return fakeAddr{} }
func (c *fakeConn) RemoteAddr() net.Addr             { return fakeAddr{} }
func (c *fakeConn) SetDeadline(time.Time) error      { return nil }
func (c *fakeConn) SetReadDeadline(time.Time) error  { return nil }
func (c *fakeConn) SetWriteDeadline(time.Time) error { return nil }

type fakeAddr struct{}

func (fakeAddr) Network() string { return "fake" }
func (fakeAddr) String() string  { return "fake" }

// countingConn counts the bytes written to the embedded connection.
type countingConn struct {
	net.Conn
	written atomic.Int64
}

func (c *countingConn) Write(b []byte) (int, error) {
	n, err := c.Conn.Write(b)
	c.written.Add(int64(n))
	return n, err
}

// startRelay runs relay in a goroutine and returns a channel closed when it returns.
func startRelay(
	ctx context.Context, browser, browserRaw, upstream, upstreamRaw net.Conn, protocol string, replayLimit int,
) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		relay(ctx, browser, browserRaw, upstream, upstreamRaw, protocol, replayLimit)
	}()
	return done
}

func requireReturns(t *testing.T, done <-chan struct{}, timeout time.Duration, what string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(timeout):
		require.FailNow(t, what+" did not return within "+timeout.String())
	}
}
