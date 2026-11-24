package common

import (
	"fmt"
	"net"
	"strings"
	"sync/atomic"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/clientcert"
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

// startClientCertificateProxy starts the proxy presenting opts.ClientCertificates. It
// reaches destinations through opts.Proxy, or directly with the VU's hosts and DNS settings.
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
		Logger:      b.logger,
	}
	if opts.Proxy != nil {
		cfg.ProxyServer = strings.TrimSpace(opts.Proxy.Server)
	}
	dial, err := clientcert.NewUpstreamDialer(cfg, direct.DialContext)
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

// clientCertificateProxyRules returns the Chromium proxy server and bypass list for a context
// routed through the client certificate proxy at addr: https, ws and wss go to addr, plain http
// goes to proxy.server when proxy is set, and loopback hosts are proxied too.
func clientCertificateProxyRules(addr string, proxy *ProxyOptions) (server, bypassList string) {
	const noLoopbackBypass = "<-loopback>"
	server, bypassList = "https="+addr, noLoopbackBypass
	if proxy == nil {
		return server, bypassList
	}
	server = "http=" + strings.TrimSpace(proxy.Server) + ";" + server
	if bypass := strings.TrimSpace(proxy.Bypass); bypass != "" {
		bypassList += "," + bypass
	}
	return server, bypassList
}
