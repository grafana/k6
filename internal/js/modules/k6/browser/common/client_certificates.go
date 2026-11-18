package common

import (
	"context"
	"errors"
	"fmt"
	"net"
	"runtime"
	"strings"
	"sync/atomic"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/clientcert"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/env"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/k6ext"
	k6netext "go.k6.io/k6/v2/lib/netext"
)

// proxyCredentials holds a browser context's httpCredentials for the client certificate
// proxy, which reads them from its own goroutines.
type proxyCredentials struct {
	credentials atomic.Pointer[Credentials]
}

func newProxyCredentials(c Credentials) *proxyCredentials {
	p := &proxyCredentials{}
	p.set(c)
	return p
}

func (p *proxyCredentials) set(c Credentials) {
	p.credentials.Store(&c)
}

func (p *proxyCredentials) get() (username, password string) {
	c := p.credentials.Load()
	return c.Username, c.Password
}

// halfCloseConn exposes the CloseWrite method that [k6netext.Conn] hides.
type halfCloseConn struct {
	*k6netext.Conn
}

// CloseWrite half-closes the wrapped connection when it supports it.
func (c halfCloseConn) CloseWrite() error {
	if cw, ok := c.Conn.Conn.(interface{ CloseWrite() error }); ok {
		return cw.CloseWrite()
	}
	return errors.ErrUnsupported
}

// clientCertificateDirectDial returns a DialFunc that dials with d and keeps the
// connections' CloseWrite reachable.
func clientCertificateDirectDial(d *k6netext.Dialer) clientcert.DialFunc {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		conn, err := d.DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}
		if nc, ok := conn.(*k6netext.Conn); ok {
			return halfCloseConn{nc}, nil
		}
		return conn, nil
	}
}

// startClientCertificateProxy starts the proxy presenting opts.ClientCertificates. It
// reaches destinations through opts.Proxy or the environment proxy, or directly with the
// VU's hosts and DNS settings.
func (b *Browser) startClientCertificateProxy(
	opts *BrowserContextOptions,
) (*clientcert.Proxy, *proxyCredentials, error) {
	state := k6ext.GetVU(b.vuCtx).State()
	resolver, err := newResolver(state.Options.DNS)
	if err != nil {
		return nil, nil, fmt.Errorf("newResolver(%+v): %w", state.Options.DNS, err)
	}
	direct := k6netext.NewDialer(net.Dialer{}, resolver)
	direct.Hosts = state.Options.Hosts.Trie

	creds := newProxyCredentials(opts.HTTPCredentials)
	cfg := clientcert.UpstreamConfig{
		Credentials: creds.get,
		LookupEnv:   env.Lookup,
		GOOS:        runtime.GOOS,
		Logger:      b.logger,
	}
	if opts.Proxy != nil {
		cfg.ProxyServer = strings.TrimSpace(opts.Proxy.Server)
	}
	dial, err := clientcert.NewUpstreamDialer(cfg, clientCertificateDirectDial(direct))
	if err != nil {
		return nil, nil, err
	}

	proxy, err := clientcert.NewProxy(clientcert.ProxyConfig{
		Certificates:      opts.ClientCertificates,
		IgnoreHTTPSErrors: opts.IgnoreHTTPSErrors,
	}, dial, b.logger)
	if err != nil {
		return nil, nil, fmt.Errorf("starting client certificate proxy: %w", err)
	}
	b.logger.Debugf("Browser:startClientCertificateProxy", "addr:%v", proxy.Addr())

	return proxy, creds, nil
}

// clientCertificateProxyBypassList returns the Chromium bypass list for a context routed
// through the client certificate proxy: loopback hosts are proxied too.
func clientCertificateProxyBypassList(proxy *ProxyOptions) string {
	const noLoopbackBypass = "<-loopback>"
	if proxy == nil {
		return noLoopbackBypass
	}
	if bypass := strings.TrimSpace(proxy.Bypass); bypass != "" {
		return noLoopbackBypass + "," + bypass
	}
	return noLoopbackBypass
}
