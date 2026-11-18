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

	"golang.org/x/net/http/httpproxy"
	"golang.org/x/net/proxy"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/log"
)

// UpstreamConfig configures how destination connections are opened.
type UpstreamConfig struct {
	// ProxyServer is the browser context's proxy.server; empty means none.
	ProxyServer string
	// Credentials returns the browser context's httpCredentials, used for proxy 407 challenges.
	Credentials func() (username, password string)
	// LookupEnv reads proxy environment variables.
	LookupEnv func(key string) (string, bool)
	// GOOS selects whether proxy environment variables apply: not on darwin or windows, where
	// Chromium uses system proxy settings instead.
	GOOS string
	// Logger receives debug logs of each dial; nil disables them.
	Logger *log.Logger

	// rootCAs verifies https proxies; nil means the system roots.
	rootCAs *x509.CertPool
}

// upstreamProxy is a parsed proxy server.
type upstreamProxy struct {
	scheme string // http, https, or socks5
	host   string // hostname, used as the TLS server name of an https proxy
	addr   string // host:port
}

type upstreamDialer struct {
	cfg    UpstreamConfig
	proxy  *upstreamProxy
	direct DialFunc
}

// NewUpstreamDialer returns a DialFunc that reaches destinations through the configured or
// environment proxy, or directly through direct.
func NewUpstreamDialer(cfg UpstreamConfig, direct DialFunc) (DialFunc, error) {
	d := &upstreamDialer{cfg: cfg, direct: direct}
	if cfg.ProxyServer != "" {
		server := cfg.ProxyServer
		if !strings.Contains(server, "://") {
			server = "http://" + server
		}
		u, err := url.Parse(server)
		if err != nil {
			return nil, errors.New("unsupported proxy server: cannot parse it as [<scheme>://]<host>[:<port>]")
		}
		if d.proxy, err = parseUpstreamProxy(u); err != nil {
			return nil, err
		}
	}
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
	case "socks5", "socks5h":
		scheme, port = "socks5", "1080"
	default:
		return nil, fmt.Errorf("unsupported proxy server scheme %q: use http, https, or socks5", u.Scheme)
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

func (d *upstreamDialer) dial(ctx context.Context, network, address string) (net.Conn, error) {
	p := d.proxy
	if p == nil {
		var err error
		if p, err = d.environmentProxy(address); err != nil {
			return nil, err
		}
	}
	if p == nil {
		d.cfg.Logger.Debugf("Upstream:dial", "address:%s via:direct", address)
		return d.direct(ctx, network, address)
	}
	d.cfg.Logger.Debugf("Upstream:dial", "address:%s via:%s", address, p.addr)
	if p.scheme == "socks5" {
		return d.dialSOCKS5(ctx, p, network, address)
	}
	return d.dialConnect(ctx, p, address)
}

// environmentProxy returns the HTTPS_PROXY proxy for address unless NO_PROXY or loopback
// exempts it, or nil when the environment does not apply on GOOS.
func (d *upstreamDialer) environmentProxy(address string) (*upstreamProxy, error) {
	if d.cfg.GOOS == "darwin" || d.cfg.GOOS == "windows" || d.cfg.LookupEnv == nil {
		return nil, nil //nolint:nilnil // no proxy is not an error
	}
	cfg := httpproxy.Config{
		HTTPSProxy: d.lookupEnv("HTTPS_PROXY", "https_proxy"),
		NoProxy:    d.lookupEnv("NO_PROXY", "no_proxy"),
	}
	u, err := cfg.ProxyFunc()(&url.URL{Scheme: "https", Host: address})
	if err != nil {
		return nil, errors.New("unsupported proxy server in HTTPS_PROXY: cannot parse it")
	}
	if u == nil {
		return nil, nil //nolint:nilnil // no proxy is not an error
	}
	return parseUpstreamProxy(u)
}

func (d *upstreamDialer) lookupEnv(keys ...string) string {
	for _, k := range keys {
		if v, ok := d.cfg.LookupEnv(k); ok && v != "" {
			return v
		}
	}
	return ""
}

// contextDialer adapts a DialFunc to proxy.ContextDialer.
type contextDialer DialFunc

func (f contextDialer) Dial(network, address string) (net.Conn, error) {
	return f(context.Background(), network, address)
}

func (f contextDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return f(ctx, network, address)
}

func (d *upstreamDialer) dialSOCKS5(ctx context.Context, p *upstreamProxy, network, address string) (net.Conn, error) {
	dialer, err := proxy.SOCKS5("tcp", p.addr, nil, contextDialer(d.direct))
	if err != nil {
		return nil, fmt.Errorf("creating SOCKS5 dialer for %s: %w", p.addr, err)
	}
	cd, ok := dialer.(proxy.ContextDialer)
	if !ok {
		return nil, fmt.Errorf("SOCKS5 dialer for %s does not support contexts", p.addr)
	}
	conn, err := cd.DialContext(ctx, network, address)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("SOCKS5 proxy CONNECT to %s: %w", address, err)
	}
	return conn, nil
}

// dialConnect opens a tunnel to address through an http or https proxy, retrying once with
// the context's credentials when the proxy answers 407.
func (d *upstreamDialer) dialConnect(ctx context.Context, p *upstreamProxy, address string) (net.Conn, error) {
	conn, code, status, err := d.connect(ctx, p, address, "")
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
		if conn, code, status, err = d.connect(ctx, p, address, auth); err != nil {
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
	ctx context.Context, p *upstreamProxy, address, auth string,
) (_ net.Conn, code int, status string, _ error) {
	raw, err := d.direct(ctx, "tcp", p.addr)
	if err != nil {
		return nil, 0, "", ctxErrOr(ctx, fmt.Errorf("dialing proxy %s: %w", p.addr, err))
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
			return nil, 0, "", ctxErrOr(ctx, fmt.Errorf("TLS handshake with proxy %s: %w", p.addr, err))
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
		return nil, 0, "", ctxErrOr(ctx, fmt.Errorf("writing CONNECT to proxy %s: %w", p.addr, err))
	}
	br := bufio.NewReader(transport)
	// Closing the CONNECT response body would drain its framing from the tunnel or stall on a
	// slow error body; the raw conn is closed instead.
	resp, err := http.ReadResponse(br, req) //nolint:bodyclose // the raw conn is closed instead
	if err != nil {
		return nil, 0, "", ctxErrOr(ctx, fmt.Errorf("reading CONNECT response from proxy %s: %w", p.addr, err))
	}
	if resp.StatusCode != http.StatusOK {
		_ = raw.Close()
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
