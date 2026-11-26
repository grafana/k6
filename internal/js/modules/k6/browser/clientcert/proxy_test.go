package clientcert

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
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
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/log"
	"go.k6.io/k6/v2/internal/lib/testutils/tlstest"
)

func TestProxy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		h2         bool
		unlistedCA bool
		protoMajor int
		protocol   string
	}{
		{name: "h2", h2: true, protoMajor: 2, protocol: "h2"},
		{name: "http1", h2: false, protoMajor: 1, protocol: "http/1.1"},
		{name: "ca_not_listed_by_server", h2: true, unlistedCA: true, protoMajor: 2, protocol: "h2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			chain := tlstest.NewChain(t)
			serverCfg := &tls.Config{
				ClientAuth: tls.RequireAndVerifyClientCert,
				ClientCAs:  chain.RootPool,
			}
			if tt.unlistedCA {
				// The server lists a CA that issued none of the client's chain but accepts any
				// certificate.
				dummy, err := dummyCertificate()
				require.NoError(t, err)
				serverCfg.ClientAuth = tls.RequireAnyClientCert
				serverCfg.ClientCAs = x509.NewCertPool()
				serverCfg.ClientCAs.AddCert(dummy.Leaf)
			}
			srv := startTLSServer(t, chain, serverCfg, tt.h2)
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
		h2                bool
		rejectCert        bool
		maxVersion        uint16
		ignoreHTTPSErrors bool
		wantStatus        int
		wantProtoMajor    int
	}{
		{name: "untrusted_server_h1", untrusted: true, wantStatus: http.StatusServiceUnavailable, wantProtoMajor: 1},
		{
			name: "untrusted_server_h2", untrusted: true, h2: true,
			wantStatus: http.StatusServiceUnavailable, wantProtoMajor: 2,
		},
		{
			name: "server_rejects_cert_tls12", rejectCert: true,
			maxVersion: tls.VersionTLS12, wantStatus: http.StatusServiceUnavailable, wantProtoMajor: 1,
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
			srv := startTLSServer(t, chain, serverCfg, tt.h2)

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
			assert.Equal(t, tt.wantProtoMajor, resp.ProtoMajor)
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

func TestSelectClientCertificate(t *testing.T) {
	t.Parallel()

	var (
		ecdsaScheme   = tls.ECDSAWithP256AndSHA256
		pssScheme     = tls.PSSWithSHA256
		pkcs1Scheme   = tls.PKCS1WithSHA256
		ed25519Scheme = tls.Ed25519
	)
	certs := map[string]tls.Certificate{
		"rsa":     tlstest.NewChain(t).ClientCertificate(t, "rsa", tlstest.KeyRSA),
		"ecdsa":   tlstest.NewChain(t).ClientCertificate(t, "ecdsa", tlstest.KeyECDSA),
		"ed25519": tlstest.NewChain(t).ClientCertificate(t, "ed25519", tlstest.KeyEd25519),
	}
	// No certificate in the loaded chains is issued by the dummy certificate's subject.
	dummy, err := dummyCertificate()
	require.NoError(t, err)
	otherCA := [][]byte{dummy.Leaf.RawSubject}

	tests := []struct {
		name          string
		version       uint16
		schemes       []tls.SignatureScheme
		acceptableCAs [][]byte
		certs         []string
		want          string
	}{
		{
			name: "tls13_ecdsa_first", version: tls.VersionTLS13,
			schemes: []tls.SignatureScheme{ecdsaScheme, pssScheme, pkcs1Scheme},
			certs:   []string{"rsa", "ecdsa"}, want: "ecdsa",
		},
		{
			name: "tls13_ecdsa_first_reversed", version: tls.VersionTLS13,
			schemes: []tls.SignatureScheme{ecdsaScheme, pssScheme, pkcs1Scheme},
			certs:   []string{"ecdsa", "rsa"}, want: "ecdsa",
		},
		{
			name: "tls13_rsa_first", version: tls.VersionTLS13,
			schemes: []tls.SignatureScheme{pssScheme, pkcs1Scheme, ecdsaScheme},
			certs:   []string{"ecdsa", "rsa"}, want: "rsa",
		},
		{
			name: "tls13_ed25519_first", version: tls.VersionTLS13,
			schemes: []tls.SignatureScheme{ed25519Scheme, pssScheme},
			certs:   []string{"rsa", "ed25519"}, want: "ed25519",
		},
		{
			name: "tls13_ca_list_ignored", version: tls.VersionTLS13,
			schemes: []tls.SignatureScheme{pssScheme}, acceptableCAs: otherCA,
			certs: []string{"rsa"}, want: "rsa",
		},
		{
			name: "tls13_no_acceptable_scheme", version: tls.VersionTLS13,
			schemes: []tls.SignatureScheme{ecdsaScheme},
			certs:   []string{"rsa"}, want: "",
		},
		{
			name: "tls12_last_loaded", version: tls.VersionTLS12,
			schemes: []tls.SignatureScheme{ecdsaScheme, pssScheme, pkcs1Scheme},
			certs:   []string{"ecdsa", "rsa"}, want: "rsa",
		},
		{
			name: "tls12_last_loaded_reversed", version: tls.VersionTLS12,
			schemes: []tls.SignatureScheme{pssScheme, pkcs1Scheme, ecdsaScheme},
			certs:   []string{"rsa", "ecdsa"}, want: "ecdsa",
		},
		{
			name: "tls12_last_not_acceptable", version: tls.VersionTLS12,
			schemes: []tls.SignatureScheme{ecdsaScheme},
			certs:   []string{"ecdsa", "rsa"}, want: "",
		},
		{
			name: "tls12_ca_list_ignored", version: tls.VersionTLS12,
			schemes: []tls.SignatureScheme{pssScheme, pkcs1Scheme}, acceptableCAs: otherCA,
			certs: []string{"rsa"}, want: "rsa",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			loaded := make([]tls.Certificate, 0, len(tt.certs))
			for _, name := range tt.certs {
				loaded = append(loaded, certs[name])
			}
			got, err := selectClientCertificate(loaded)(&tls.CertificateRequestInfo{
				Version:          tt.version,
				SignatureSchemes: tt.schemes,
				AcceptableCAs:    tt.acceptableCAs,
			})
			require.NoError(t, err)
			require.NotNil(t, got)
			if tt.want == "" {
				assert.Empty(t, got.Certificate)
				return
			}
			require.NotNil(t, got.Leaf)
			assert.Equal(t, tt.want, got.Leaf.Subject.CommonName)
		})
	}
}

func TestServe503H2_Terminates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		request bool
	}{
		{name: "client_keeps_connection_open", request: true},
		{name: "browser_closes_before_request"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, browser, browserRaw := tlsPipe(t, "h2")
			done := make(chan struct{})
			go func() {
				defer close(done)
				serve503H2(browser, browserRaw, errors.New("upstream failed"))
			}()

			if !tt.request {
				require.NoError(t, client.Close())
				requireReturns(t, done, 3*time.Second, "serve503H2")
				return
			}

			var resp h2Response
			select {
			case resp = <-sendH2Request(t, client):
			case <-time.After(5 * time.Second):
				require.FailNow(t, "no response")
			}

			requireReturns(t, done, 3*time.Second, "serve503H2")
			require.NoError(t, resp.err)
			assert.Equal(t, "503", resp.status)
			assert.Equal(t, "text/html; charset=utf-8", resp.contentType)
			assert.Equal(t, "k6 client-certificate error: upstream failed", resp.body)
		})
	}
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

// tlsPipe returns the client and server ends of an in-memory TLS connection that negotiated
// protocol, and the server's underlying connection.
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

// requireReturns fails t unless done is closed within timeout.
func requireReturns(t *testing.T, done <-chan struct{}, timeout time.Duration, what string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(timeout):
		require.FailNow(t, what+" did not return within "+timeout.String())
	}
}

type h2Response struct {
	status      string
	contentType string
	body        string
	err         error
}

// sendH2Request sends a GET request on stream 1 of conn and delivers the response on the
// returned channel. Unlike Go's HTTP/2 client, it keeps reading conn afterwards and never
// closes it, even after a GOAWAY.
func sendH2Request(t *testing.T, conn net.Conn) <-chan h2Response {
	t.Helper()
	fr := http2.NewFramer(conn, conn)
	fr.ReadMetaHeaders = hpack.NewDecoder(4096, nil)

	result := make(chan h2Response, 1)
	go func() {
		var resp h2Response
		for {
			f, err := fr.ReadFrame()
			if err != nil {
				resp.err = err
				result <- resp
				return
			}
			switch f := f.(type) {
			case *http2.MetaHeadersFrame:
				resp.status = f.PseudoValue("status")
				for _, hf := range f.RegularFields() {
					if hf.Name == "content-type" {
						resp.contentType = hf.Value
					}
				}
			case *http2.DataFrame:
				resp.body += string(f.Data())
				if f.StreamEnded() {
					result <- resp
					for {
						if _, err := fr.ReadFrame(); err != nil {
							return
						}
					}
				}
			}
		}
	}()

	var block bytes.Buffer
	enc := hpack.NewEncoder(&block)
	for _, hf := range []hpack.HeaderField{
		{Name: ":method", Value: http.MethodGet},
		{Name: ":scheme", Value: "https"},
		{Name: ":authority", Value: "example.com"},
		{Name: ":path", Value: "/"},
	} {
		require.NoError(t, enc.WriteField(hf))
	}
	_, err := io.WriteString(conn, http2.ClientPreface)
	require.NoError(t, err)
	require.NoError(t, fr.WriteSettings())
	require.NoError(t, fr.WriteHeaders(http2.HeadersFrameParam{
		StreamID:      1,
		BlockFragment: block.Bytes(),
		EndStream:     true,
		EndHeaders:    true,
	}))
	return result
}
