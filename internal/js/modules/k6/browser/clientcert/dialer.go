package clientcert

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/log"
)

// UpstreamConfig configures how destination connections are opened.
type UpstreamConfig struct {
	// ProxyServer is the browser context's proxy.server; empty means none.
	ProxyServer string
	// Credentials returns the browser context's httpCredentials, used for proxy 407 challenges.
	Credentials func() (username, password string)
	// Logger receives debug logs of each dial; nil disables them.
	Logger *log.Logger

	// rootCAs verifies https proxies; nil means the system roots.
	rootCAs *x509.CertPool
}

// upstreamProxy is a parsed proxy server.
type upstreamProxy struct {
	scheme string // http or https
	host   string // hostname, used as the TLS server name of an https proxy
	addr   string // host:port
}

type upstreamDialer struct {
	cfg    UpstreamConfig
	proxy  *upstreamProxy
	direct DialFunc
}

// NewUpstreamDialer returns a DialFunc that reaches destinations through the configured http or
// https proxy, or directly through direct when there is none. Destination hostnames are passed to
// the proxy unresolved.
func NewUpstreamDialer(cfg UpstreamConfig, direct DialFunc) (DialFunc, error) {
	if cfg.ProxyServer == "" {
		return direct, nil
	}
	server := cfg.ProxyServer
	if !strings.Contains(server, "://") {
		server = "http://" + server
	}
	u, err := url.Parse(server)
	if err != nil {
		return nil, errors.New("unsupported proxy server: cannot parse it as [<scheme>://]<host>[:<port>]")
	}
	p, err := parseUpstreamProxy(u)
	if err != nil {
		return nil, err
	}
	d := &upstreamDialer{cfg: cfg, proxy: p, direct: direct}
	return d.dial, nil
}

// parseUpstreamProxy validates the proxy scheme and applies its default port.
func parseUpstreamProxy(u *url.URL) (*upstreamProxy, error) {
	scheme := strings.ToLower(u.Scheme)
	var port string
	switch scheme {
	case "http":
		port = "80"
	case "https":
		port = "443"
	default:
		return nil, fmt.Errorf("unsupported proxy server scheme %q: use http or https", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return nil, errors.New("unsupported proxy server: missing host")
	}
	if p := u.Port(); p != "" {
		port = p
	}
	return &upstreamProxy{scheme: scheme, host: host, addr: net.JoinHostPort(host, port)}, nil
}

// dial opens a tunnel to address through the proxy, retrying once with the context's
// credentials when the proxy answers 407.
func (d *upstreamDialer) dial(ctx context.Context, _, address string) (net.Conn, error) {
	d.cfg.Logger.Debugf("Upstream:dial", "address:%s via:%s", address, d.proxy.scheme)
	conn, code, status, err := d.connect(ctx, address, "")
	if err != nil {
		return nil, err
	}
	if code == http.StatusProxyAuthRequired {
		var username, password string
		if d.cfg.Credentials != nil {
			username, password = d.cfg.Credentials()
		}
		if strings.TrimSpace(username) == "" && strings.TrimSpace(password) == "" {
			return nil, fmt.Errorf("proxy CONNECT to %s requires authentication (407)", address)
		}
		auth := "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
		if conn, code, status, err = d.connect(ctx, address, auth); err != nil {
			return nil, err
		}
	}
	if code != http.StatusOK {
		return nil, fmt.Errorf("proxy CONNECT to %s failed: %s", address, status)
	}
	return conn, nil
}

// connect makes one CONNECT attempt on its own connection and returns the response status.
// The connection is returned only for a 200; for any other status it is closed.
func (d *upstreamDialer) connect(
	ctx context.Context, address, auth string,
) (_ net.Conn, code int, status string, _ error) {
	p := d.proxy
	raw, err := d.direct(ctx, "tcp", p.addr)
	if err != nil {
		return nil, 0, "", ctxErrOr(ctx, fmt.Errorf("dialing %s proxy: %w", p.scheme, err))
	}
	// Closing raw, never the TLS conn, aborts without waiting to send close_notify.
	stop := context.AfterFunc(ctx, func() { _ = raw.Close() })
	defer stop()
	handedOver := false
	defer func() {
		if !handedOver {
			_ = raw.Close()
		}
	}()

	transport := raw
	if p.scheme == "https" {
		tc := tls.Client(raw, &tls.Config{
			ServerName: p.host,
			RootCAs:    d.cfg.rootCAs,
			MinVersion: tls.VersionTLS12,
		})
		if err := tc.HandshakeContext(ctx); err != nil {
			return nil, 0, "", ctxErrOr(ctx, fmt.Errorf("TLS handshake with https proxy: %w", err))
		}
		transport = tc
	}

	req := &http.Request{
		Method: http.MethodConnect,
		URL:    &url.URL{Opaque: address},
		Host:   address,
		Header: make(http.Header),
	}
	if auth != "" {
		req.Header.Set("Proxy-Authorization", auth)
	}
	if err := req.Write(transport); err != nil {
		return nil, 0, "", ctxErrOr(ctx, fmt.Errorf("writing CONNECT to %s proxy: %w", p.scheme, err))
	}
	br := bufio.NewReader(transport)
	// Closing the CONNECT response body would drain its framing from the tunnel or stall on a
	// slow error body; the raw conn is closed instead.
	resp, err := http.ReadResponse(br, req) //nolint:bodyclose // the raw conn is closed instead
	if err != nil {
		return nil, 0, "", ctxErrOr(ctx, fmt.Errorf("reading CONNECT response from %s proxy: %w", p.scheme, err))
	}
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, resp.Status, nil
	}
	if !stop() || ctx.Err() != nil {
		return nil, 0, "", ctx.Err()
	}
	handedOver = true
	return &tunnelConn{Conn: raw, br: br, transport: transport}, resp.StatusCode, resp.Status, nil
}

func ctxErrOr(ctx context.Context, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	return err
}

// tunnelConn is an established CONNECT tunnel. Reads drain the bytes buffered while reading
// the CONNECT response before reading the transport; Close and deadlines apply to the
// embedded raw connection.
type tunnelConn struct {
	net.Conn

	br        *bufio.Reader
	transport net.Conn
}

func (c *tunnelConn) Read(b []byte) (int, error) {
	if c.br.Buffered() > 0 {
		return c.br.Read(b)
	}
	return c.transport.Read(b)
}

func (c *tunnelConn) Write(b []byte) (int, error) {
	return c.transport.Write(b)
}

// CloseWrite half-closes the transport, or closes the connection when the transport cannot
// be half-closed.
func (c *tunnelConn) CloseWrite() error {
	if cw, ok := c.transport.(interface{ CloseWrite() error }); ok {
		return cw.CloseWrite()
	}
	return c.Close()
}
