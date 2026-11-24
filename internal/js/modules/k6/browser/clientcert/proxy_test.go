package clientcert

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	stdlog "log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/log"
	"go.k6.io/k6/v2/internal/lib/testutils/tlstest"
)

func TestProxy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		h2         bool
		protoMajor int
		protocol   string
	}{
		{name: "h2", h2: true, protoMajor: 2, protocol: "h2"},
		{name: "http1", h2: false, protoMajor: 1, protocol: "http/1.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			chain := tlstest.NewChain(t)
			srv := startTLSServer(t, chain, &tls.Config{
				ClientAuth: tls.RequireAndVerifyClientCert,
				ClientCAs:  chain.RootPool,
			}, tt.h2)
			p := startProxy(t, ProxyConfig{
				Certificates: map[string][]tls.Certificate{
					originOf(t, srv.URL): {chain.ClientCertificate(t, "client", tlstest.KeyECDSA)},
				},
				RootCAs: chain.RootPool,
			})

			resp := doRequest(t, newProxyClient(t, p), srv.URL)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, "client", resp.Header.Get("X-Client-Cn"))
			assert.Equal(t, strconv.Itoa(tt.protoMajor), resp.Header.Get("X-Proto-Major"))
			assert.Equal(t, tt.protoMajor, resp.ProtoMajor)
			require.NotNil(t, resp.TLS)
			assert.Equal(t, tt.protocol, resp.TLS.NegotiatedProtocol)
			require.NotEmpty(t, resp.TLS.PeerCertificates)
			assert.Equal(t, proxyCertificateCommonName, resp.TLS.PeerCertificates[0].Subject.CommonName)
		})
	}
}

func TestProxy_NonMatchingOrigin(t *testing.T) {
	t.Parallel()

	otherOrigin := func(t *testing.T, chain *tlstest.Chain) ProxyConfig {
		t.Helper()
		return ProxyConfig{
			Certificates: map[string][]tls.Certificate{
				"https://example.com": {chain.ClientCertificate(t, "client", tlstest.KeyECDSA)},
			},
			RootCAs: chain.RootPool,
		}
	}

	t.Run("tls", func(t *testing.T) {
		t.Parallel()

		chain := tlstest.NewChain(t)
		srv := startTLSServer(t, chain, &tls.Config{
			ClientAuth: tls.VerifyClientCertIfGiven,
			ClientCAs:  chain.RootPool,
		}, false)
		p := startProxy(t, otherOrigin(t, chain))

		resp := doRequest(t, newProxyClient(t, p), srv.URL)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Empty(t, resp.Header.Get("X-Client-Cn"))
		require.NotNil(t, resp.TLS)
		require.NotEmpty(t, resp.TLS.PeerCertificates)
		assert.Equal(t, srv.Certificate().Raw, resp.TLS.PeerCertificates[0].Raw)
	})

	t.Run("plain_half_close", func(t *testing.T) {
		t.Parallel()

		destination, server := tcpPair(t)
		dial := func(context.Context, string, string) (net.Conn, error) {
			return destination, nil
		}
		p, err := NewProxy(otherOrigin(t, tlstest.NewChain(t)), dial, log.NewNullLogger())
		require.NoError(t, err)
		t.Cleanup(func() { _ = p.Close() })

		// The destination half-closes first, so the browser must keep writing after reading EOF.
		serverErr := make(chan error, 1)
		serverGot := make(chan string, 1)
		go func() {
			defer func() { _ = server.Close() }()
			if _, err := server.Write([]byte("response")); err != nil {
				serverErr <- err
				return
			}
			if err := server.CloseWrite(); err != nil {
				serverErr <- err
				return
			}
			req, err := io.ReadAll(server)
			serverGot <- string(req)
			serverErr <- err
		}()

		conn := dialConnectServer(t, p.Addr())
		client, ok := conn.(*net.TCPConn)
		require.True(t, ok)
		br := connectTunnel(t, client, "127.0.0.1:443", "req")
		got, err := io.ReadAll(br)
		require.NoError(t, err)
		assert.Equal(t, "response", string(got))

		_, err = client.Write([]byte("uest"))
		require.NoError(t, err)
		require.NoError(t, client.CloseWrite())
		require.NoError(t, <-serverErr)
		assert.Equal(t, "request", <-serverGot)
	})
}

func TestProxy_UpstreamTLSFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		untrusted         bool
		rejectCert        bool
		maxVersion        uint16
		ignoreHTTPSErrors bool
		wantStatus        int
	}{
		{name: "untrusted_server_h1", untrusted: true, wantStatus: http.StatusServiceUnavailable},
		{
			name: "server_rejects_cert_tls12", rejectCert: true,
			maxVersion: tls.VersionTLS12, wantStatus: http.StatusServiceUnavailable,
		},
		{name: "ignore_https_errors", untrusted: true, ignoreHTTPSErrors: true, wantStatus: http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			chain := tlstest.NewChain(t)
			other := tlstest.NewChain(t)
			serverCfg := &tls.Config{
				ClientAuth: tls.RequireAndVerifyClientCert,
				ClientCAs:  chain.RootPool,
				MaxVersion: tt.maxVersion,
			}
			if tt.rejectCert {
				serverCfg.ClientCAs = other.RootPool
			}
			srv := startTLSServer(t, chain, serverCfg, false)

			cfg := ProxyConfig{
				Certificates: map[string][]tls.Certificate{
					originOf(t, srv.URL): {chain.ClientCertificate(t, "client", tlstest.KeyECDSA)},
				},
				IgnoreHTTPSErrors: tt.ignoreHTTPSErrors,
				RootCAs:           chain.RootPool,
			}
			if tt.untrusted {
				cfg.RootCAs = other.RootPool
			}
			p := startProxy(t, cfg)

			resp := doRequest(t, newProxyClient(t, p), srv.URL)
			assert.Equal(t, tt.wantStatus, resp.StatusCode)
			if tt.wantStatus != http.StatusServiceUnavailable {
				return
			}
			assert.Equal(t, 1, resp.ProtoMajor)
			assert.True(t, strings.HasPrefix(resp.Body, "k6 client-certificate error: "), resp.Body)
			assert.Equal(t, "text/html; charset=utf-8", resp.Header.Get("Content-Type"))
		})
	}
}

func TestProxy_Close(t *testing.T) {
	t.Parallel()

	var d net.Dialer
	p, err := NewProxy(ProxyConfig{}, d.DialContext, log.NewNullLogger())
	require.NoError(t, err)
	addr := p.Addr()
	require.NoError(t, p.Close())

	conn, err := d.DialContext(t.Context(), "tcp", addr)
	if err == nil {
		_ = conn.Close()
	}
	assert.Error(t, err)
}

func TestClientCertificateErrorBody(t *testing.T) {
	t.Parallel()

	got := clientCertificateErrorBody(errors.New("<script>\"x\"\nsecond line"))
	assert.Equal(t, "k6 client-certificate error: &lt;script&gt;&#34;x&#34; <br>second line", got)
}

// startTLSServer starts an HTTPS server presenting chain's leaf, configured by cfg. The handler
// reports the client certificate CN in X-Client-Cn and the request's HTTP major version in
// X-Proto-Major.
func startTLSServer(t *testing.T, chain *tlstest.Chain, cfg *tls.Config, h2 bool) *httptest.Server {
	t.Helper()
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
			w.Header().Set("X-Client-Cn", r.TLS.PeerCertificates[0].Subject.CommonName)
		}
		w.Header().Set("X-Proto-Major", strconv.Itoa(r.ProtoMajor))
	}))
	srv.Config.ErrorLog = stdlog.New(io.Discard, "", 0)
	srv.EnableHTTP2 = h2
	cfg.Certificates = []tls.Certificate{chain.FullChainTLSCertificate(t)}
	srv.TLS = cfg
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

// newProxyClient returns a client that connects through p the way Chromium does: it offers h2
// and http/1.1 and ignores the proxy's certificate.
func newProxyClient(t *testing.T, p *Proxy) *http.Client {
	t.Helper()
	tr := &http.Transport{
		Proxy:             http.ProxyURL(&url.URL{Scheme: "http", Host: p.Addr()}),
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
		ForceAttemptHTTP2: true,
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

// doRequest sends a GET request for target and returns its response.
func doRequest(t *testing.T, client *http.Client, target string) testResponse {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
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

func originOf(t *testing.T, rawURL string) string {
	t.Helper()
	o, err := NormalizeOrigin(rawURL)
	require.NoError(t, err)
	return o
}
