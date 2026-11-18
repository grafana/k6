package clientcert

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/log"
)

const proxyCertificateCN = "k6 client certificate proxy"

func TestProxy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		h2         bool
		protoMajor int
	}{
		{name: "h2", h2: true, protoMajor: 2},
		{name: "http1", h2: false, protoMajor: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ca := newTestCA(t, "test CA")
			srv := startTLSServer(t, ca, serverOptions{
				h2:         tt.h2,
				clientAuth: tls.RequireAndVerifyClientCert,
				clientCAs:  ca.pool,
			})
			p := startProxy(t, ProxyConfig{
				Certificates: map[string][]tls.Certificate{
					originOf(t, srv.URL): {ca.issue(t, "client", ecdsaSEC1, x509.ExtKeyUsageClientAuth)},
				},
				RootCAs: ca.pool,
			})

			resp := doRequest(t, newProxyClient(t, p, tt.h2), http.MethodGet, srv.URL, nil)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, "client", resp.Header.Get("X-Client-Cn"))
			assert.Equal(t, strconv.Itoa(tt.protoMajor), resp.Header.Get("X-Proto-Major"))
			assert.Equal(t, tt.protoMajor, resp.ProtoMajor)
			require.NotNil(t, resp.TLS)
			require.NotEmpty(t, resp.TLS.PeerCertificates)
			assert.Equal(t, proxyCertificateCN, resp.TLS.PeerCertificates[0].Subject.CommonName)
		})
	}
}

func TestProxy_NonMatchingOrigin(t *testing.T) {
	t.Parallel()

	otherOrigin := func(t *testing.T, ca *testCA) ProxyConfig {
		t.Helper()
		return ProxyConfig{
			Certificates: map[string][]tls.Certificate{
				"https://example.com": {ca.issue(t, "client", ecdsaSEC1, x509.ExtKeyUsageClientAuth)},
			},
			RootCAs: ca.pool,
		}
	}

	t.Run("tls", func(t *testing.T) {
		t.Parallel()

		ca := newTestCA(t, "test CA")
		srv := startTLSServer(t, ca, serverOptions{clientAuth: tls.VerifyClientCertIfGiven, clientCAs: ca.pool})
		p := startProxy(t, otherOrigin(t, ca))

		resp := doRequest(t, newProxyClient(t, p, false), http.MethodGet, srv.URL, nil)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Empty(t, resp.Header.Get("X-Client-Cn"))
		require.NotNil(t, resp.TLS)
		require.NotEmpty(t, resp.TLS.PeerCertificates)
		assert.Equal(t, srv.Certificate().Raw, resp.TLS.PeerCertificates[0].Raw)
	})

	t.Run("plain_http", func(t *testing.T) {
		t.Parallel()

		srv := startHTTPServer(t)
		p := startProxy(t, otherOrigin(t, newTestCA(t, "test CA")))

		resp := doRequest(t, newProxyClient(t, p, false), http.MethodPost, srv, strings.NewReader("hello"))
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "hello", resp.Body)
	})

	t.Run("half_close", func(t *testing.T) {
		t.Parallel()

		addr := startEchoAfterEOFServer(t)
		p := startProxy(t, otherOrigin(t, newTestCA(t, "test CA")))

		conn := dialSOCKS5(t, p.Addr())
		require.Equal(t, []byte{0x05, 0x00}, socksNegotiate(t, conn, 0x00))
		host, portStr, err := net.SplitHostPort(addr)
		require.NoError(t, err)
		port, err := strconv.ParseUint(portStr, 10, 16)
		require.NoError(t, err)
		socksRequest(t, conn, 0x01, socksATYPIPv4, net.ParseIP(host).To4(), uint16(port))
		require.Equal(t, byte(0x00), readN(t, conn, 10)[1])

		tcp, ok := conn.(*net.TCPConn)
		require.True(t, ok)
		_, err = tcp.Write([]byte("request"))
		require.NoError(t, err)
		require.NoError(t, tcp.CloseWrite())

		got, err := io.ReadAll(tcp)
		require.NoError(t, err)
		assert.Equal(t, "response:request", string(got))
	})
}

func TestProxy_UpstreamTLSFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		h2                bool
		method            string
		bodySize          int
		untrusted         bool
		rejectCert        bool
		minVersion        uint16
		maxVersion        uint16
		ignoreHTTPSErrors bool
		wantStatus        int
	}{
		{name: "untrusted_server_h1", untrusted: true, wantStatus: http.StatusServiceUnavailable},
		{name: "untrusted_server_h2", h2: true, untrusted: true, wantStatus: http.StatusServiceUnavailable},
		{
			name: "server_rejects_cert_tls13_h1_get", rejectCert: true,
			minVersion: tls.VersionTLS13, wantStatus: http.StatusServiceUnavailable,
		},
		{
			name: "server_rejects_cert_tls13_h1_post_256k", method: http.MethodPost, bodySize: 256 << 10,
			rejectCert: true, minVersion: tls.VersionTLS13, wantStatus: http.StatusServiceUnavailable,
		},
		{
			name: "server_rejects_cert_tls13_h2_get", h2: true, rejectCert: true,
			minVersion: tls.VersionTLS13, wantStatus: http.StatusServiceUnavailable,
		},
		{
			name: "server_rejects_cert_tls13_h2_post_16k", h2: true, method: http.MethodPost, bodySize: 16 << 10,
			rejectCert: true, minVersion: tls.VersionTLS13, wantStatus: http.StatusServiceUnavailable,
		},
		{
			name: "server_rejects_cert_tls13_h2_post_256k", h2: true, method: http.MethodPost, bodySize: 256 << 10,
			rejectCert: true, minVersion: tls.VersionTLS13, wantStatus: http.StatusServiceUnavailable,
		},
		{
			name: "server_rejects_cert_tls12", rejectCert: true,
			maxVersion: tls.VersionTLS12, wantStatus: http.StatusServiceUnavailable,
		},
		{name: "ignore_https_errors", untrusted: true, ignoreHTTPSErrors: true, wantStatus: http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ca := newTestCA(t, "server CA")
			other := newTestCA(t, "other CA")
			opts := serverOptions{
				h2:         tt.h2,
				clientAuth: tls.RequireAndVerifyClientCert,
				clientCAs:  ca.pool,
				minVersion: tt.minVersion,
				maxVersion: tt.maxVersion,
			}
			if tt.rejectCert {
				opts.clientCAs = other.pool
			}
			srv := startTLSServer(t, ca, opts)

			cfg := ProxyConfig{
				Certificates: map[string][]tls.Certificate{
					originOf(t, srv.URL): {ca.issue(t, "client", ecdsaSEC1, x509.ExtKeyUsageClientAuth)},
				},
				IgnoreHTTPSErrors: tt.ignoreHTTPSErrors,
				RootCAs:           ca.pool,
			}
			if tt.untrusted {
				cfg.RootCAs = other.pool
			}
			p := startProxy(t, cfg)

			method := tt.method
			if method == "" {
				method = http.MethodGet
			}
			var body io.Reader
			if tt.bodySize > 0 {
				body = bytes.NewReader(make([]byte, tt.bodySize))
			}
			resp := doRequest(t, newProxyClient(t, p, tt.h2), method, srv.URL, body)
			assert.Equal(t, tt.wantStatus, resp.StatusCode)
			if tt.h2 {
				assert.Equal(t, 2, resp.ProtoMajor)
			}
			if tt.wantStatus != http.StatusServiceUnavailable {
				return
			}
			assert.True(t, strings.HasPrefix(resp.Body, "k6 client-certificate error: "), resp.Body)
			assert.True(t, strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html"), resp.Header.Get("Content-Type"))
		})
	}

	t.Run("replay_overflow_h2", func(t *testing.T) {
		t.Parallel()
		testReplayOverflow(t)
	})
}

// testReplayOverflow relays an h2 request whose recorded bytes exceed the replay limit before
// the server rejects the client certificate after the TLS 1.3 handshake.
func testReplayOverflow(t *testing.T) {
	t.Helper()

	ca := newTestCA(t, "server CA")
	serverCert := ca.issue(t, "server", ecdsaSEC1, x509.ExtKeyUsageServerAuth)
	clientCert := ca.issue(t, "client", ecdsaSEC1, x509.ExtKeyUsageClientAuth)

	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseServer := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(releaseServer)

	var lc net.ListenConfig
	l, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = l.Close() })
	go func() {
		c, err := l.Accept()
		if err != nil {
			return
		}
		defer func() { _ = c.Close() }()
		_ = tls.Server(c, &tls.Config{
			Certificates: []tls.Certificate{serverCert},
			ClientAuth:   tls.RequireAnyClientCert,
			MinVersion:   tls.VersionTLS13,
			NextProtos:   []string{"h2"},
			VerifyPeerCertificate: func([][]byte, [][]*x509.Certificate) error {
				<-release
				return errors.New("client certificate rejected")
			},
		}).HandshakeContext(t.Context())
	}()

	var d net.Dialer
	raw, err := d.DialContext(t.Context(), "tcp", l.Addr().String())
	require.NoError(t, err)
	upstreamRaw := &countingConn{Conn: raw}
	t.Cleanup(func() { _ = upstreamRaw.Close() })
	upstream := tls.Client(upstreamRaw, &tls.Config{
		ServerName:   "127.0.0.1",
		RootCAs:      ca.pool,
		Certificates: []tls.Certificate{clientCert},
		MinVersion:   tls.VersionTLS13,
		NextProtos:   []string{"h2"},
	})
	require.NoError(t, upstream.HandshakeContext(t.Context()))
	handshakeBytes := upstreamRaw.written.Load()

	client, browser, browserRaw := tlsPipe(t, "h2")
	done := startRelay(t.Context(), browser, browserRaw, upstream, upstreamRaw, "h2", 1<<10)

	response := sendH2Request(t, client, make([]byte, 16<<10))
	require.Eventually(t, func() bool {
		return upstreamRaw.written.Load()-handshakeBytes > 2<<10
	}, 5*time.Second, 5*time.Millisecond, "the request was not relayed upstream")
	releaseServer()

	select {
	case resp := <-response:
		assert.Error(t, resp.err)
		assert.Empty(t, resp.status, "the browser received a response")
	case <-time.After(5 * time.Second):
		require.FailNow(t, "the browser connection was not closed")
	}
	requireReturns(t, done, 2*time.Second, "relay")
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

func TestSelectClientCertificate(t *testing.T) {
	t.Parallel()

	var (
		ecdsaScheme   = tls.ECDSAWithP256AndSHA256
		pssScheme     = tls.PSSWithSHA256
		pkcs1Scheme   = tls.PKCS1WithSHA256
		ed25519Scheme = tls.Ed25519
	)
	certs := map[string]tls.Certificate{
		"rsa":     newTestCA(t, "rsa CA").issue(t, "rsa", rsaPKCS1, x509.ExtKeyUsageClientAuth),
		"ecdsa":   newTestCA(t, "ecdsa CA").issue(t, "ecdsa", ecdsaSEC1, x509.ExtKeyUsageClientAuth),
		"ed25519": newTestCA(t, "ed25519 CA").issue(t, "ed25519", ed25519PKCS8, x509.ExtKeyUsageClientAuth),
	}
	otherCA := [][]byte{newTestCA(t, "other CA").cert.RawSubject}

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
