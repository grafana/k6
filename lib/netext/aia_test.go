package netext

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ────────────────────────────────────────────────────────────────────────────
// Test helpers
// ────────────────────────────────────────────────────────────────────────────

// testChain holds a three-level certificate chain: root CA → intermediate CA → leaf.
// The leaf's AIA extension points to an HTTP URL where the intermediate can be fetched.
type testChain struct {
	rootCert         *x509.Certificate
	intermediateCert *x509.Certificate
	intermediateDER  []byte
	leafCert         *x509.Certificate
	leafKey          *ecdsa.PrivateKey
	rootPool         *x509.CertPool
}

// newTestChain builds a root CA, intermediate CA, and leaf certificate.
// aiaURL is baked into both the intermediate and leaf as their IssuingCertificateURL,
// so that WrapTLSConfigForAIAFetching knows where to fetch the intermediate from.
func newTestChain(t testing.TB, aiaURL string) *testChain {
	t.Helper()

	genKey := func(t testing.TB) *ecdsa.PrivateKey {
		t.Helper()
		k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)
		return k
	}
	serial := func(t testing.TB) *big.Int {
		t.Helper()
		n, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
		require.NoError(t, err)
		return n
	}

	// Root CA – self-signed, trusted
	rootKey := genKey(t)
	rootTmpl := &x509.Certificate{
		SerialNumber:          serial(t),
		Subject:               pkix.Name{CommonName: "Test Root CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	rootDER, err := x509.CreateCertificate(rand.Reader, rootTmpl, rootTmpl, &rootKey.PublicKey, rootKey)
	require.NoError(t, err)
	rootCert, err := x509.ParseCertificate(rootDER)
	require.NoError(t, err)

	// Intermediate CA – signed by root, NOT sent by TLS server, fetched via AIA
	interKey := genKey(t)
	interTmpl := &x509.Certificate{
		SerialNumber:          serial(t),
		Subject:               pkix.Name{CommonName: "Test Intermediate CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	interDER, err := x509.CreateCertificate(rand.Reader, interTmpl, rootCert, &interKey.PublicKey, rootKey)
	require.NoError(t, err)
	interCert, err := x509.ParseCertificate(interDER)
	require.NoError(t, err)

	// Leaf cert – signed by intermediate, presented alone by the TLS server
	leafKey := genKey(t)
	leafTmpl := &x509.Certificate{
		SerialNumber:          serial(t),
		Subject:               pkix.Name{CommonName: "localhost"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		IssuingCertificateURL: []string{aiaURL}, // Where to fetch the intermediate
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, interCert, &leafKey.PublicKey, interKey)
	require.NoError(t, err)
	leafCert, err := x509.ParseCertificate(leafDER)
	require.NoError(t, err)

	rootPool := x509.NewCertPool()
	rootPool.AddCert(rootCert)

	return &testChain{
		rootCert:         rootCert,
		intermediateCert: interCert,
		intermediateDER:  interDER,
		leafCert:         leafCert,
		leafKey:          leafKey,
		rootPool:         rootPool,
	}
}

// leafTLSCert returns a tls.Certificate containing only the leaf (no chain).
func (tc *testChain) leafTLSCert(t testing.TB) tls.Certificate {
	t.Helper()
	leafKeyDER, err := x509.MarshalECPrivateKey(tc.leafKey)
	require.NoError(t, err)
	cert, err := tls.X509KeyPair(
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: tc.leafCert.Raw}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: leafKeyDER}),
	)
	require.NoError(t, err)
	return cert
}

// leafOnlyTLSServer starts an HTTPS server that sends only the leaf certificate.
// Clients that rely on a complete chain or AIA will observe an incomplete chain.
func (tc *testChain) leafOnlyTLSServer(t testing.TB) *httptest.Server {
	t.Helper()
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintln(w, "ok")
	}))
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{tc.leafTLSCert(t)}}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return srv
}

// aiaHandler is a thread-safe HTTP handler that serves raw DER bytes.
// Set certDER before making any requests; it is safe to set it after the server starts
// as long as no requests arrive before it is set.
type aiaHandler struct {
	mu      sync.RWMutex
	certDER []byte
}

func (h *aiaHandler) setCert(der []byte) {
	h.mu.Lock()
	h.certDER = der
	h.mu.Unlock()
}

func (h *aiaHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	w.Header().Set("Content-Type", "application/pkix-cert")
	_, _ = w.Write(h.certDER)
}

// startAIAServer starts an httptest.Server backed by h, returning the server URL.
func startAIAServer(t testing.TB, h http.Handler) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

// testHTTPClient builds a minimal http.Client backed by the given root pool, used to
// make HTTPS calls against the leaf-only TLS server during tests.
func testHTTPClient(t testing.TB, tlsCfg *tls.Config) *http.Client {
	t.Helper()
	return &http.Client{Transport: &http.Transport{TLSClientConfig: tlsCfg}}
}

// nullLogger returns a logrus logger that discards all output.
func nullLogger() logrus.FieldLogger {
	l := logrus.New()
	l.SetOutput(nil)
	l.SetLevel(logrus.PanicLevel)
	return l
}

// closeBody closes resp.Body when resp is non-nil, discarding any close error.
// Used in tests that expect TLS failures where the response body is typically nil.
func closeBody(resp *http.Response) {
	if resp != nil {
		_ = resp.Body.Close()
	}
}

// ────────────────────────────────────────────────────────────────────────────
// (a) Happy path – incomplete chain + valid AIA endpoint
// ────────────────────────────────────────────────────────────────────────────

func TestWrapTLSConfigForAIAFetching_HappyPath(t *testing.T) {
	t.Parallel()

	h := &aiaHandler{}
	aiaSrv := startAIAServer(t, h)
	chain := newTestChain(t, aiaSrv.URL+"/ca.der")
	h.setCert(chain.intermediateDER) // serve real intermediate

	tlsSrv := chain.leafOnlyTLSServer(t)

	wrappedCfg := WrapTLSConfigForAIAFetching(
		&tls.Config{RootCAs: chain.rootPool},
		nullLogger(),
		nil, // default AIA client
	)
	resp, err := testHTTPClient(t, wrappedCfg).Get(tlsSrv.URL) //nolint:noctx
	require.NoError(t, err, "AIA fetching should resolve the incomplete chain")
	_ = resp.Body.Close()
}

// ────────────────────────────────────────────────────────────────────────────
// (a-regression) Complete chain – wrapper must not interfere
// ────────────────────────────────────────────────────────────────────────────

func TestWrapTLSConfigForAIAFetching_CompleteChainPassesThrough(t *testing.T) {
	t.Parallel()

	h := &aiaHandler{}
	aiaSrv := startAIAServer(t, h)
	chain := newTestChain(t, aiaSrv.URL+"/ca.der")
	h.setCert(chain.intermediateDER)

	// Build a TLS server that sends the FULL chain: leaf + intermediate.
	leafKeyDER, err := x509.MarshalECPrivateKey(chain.leafKey)
	require.NoError(t, err)
	fullChainCert, err := tls.X509KeyPair(
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: chain.leafCert.Raw}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: leafKeyDER}),
	)
	require.NoError(t, err)
	fullChainCert.Certificate = append(fullChainCert.Certificate, chain.intermediateCert.Raw)

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintln(w, "ok")
	}))
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{fullChainCert}}
	srv.StartTLS()
	t.Cleanup(srv.Close)

	wrappedCfg := WrapTLSConfigForAIAFetching(
		&tls.Config{RootCAs: chain.rootPool},
		nullLogger(),
		nil,
	)
	resp, err := testHTTPClient(t, wrappedCfg).Get(srv.URL) //nolint:noctx
	require.NoError(t, err, "complete chain should succeed without any AIA fetch")
	_ = resp.Body.Close()
}

// ────────────────────────────────────────────────────────────────────────────
// (d) AIA fetching disabled – default behaviour preserved (regression)
// ────────────────────────────────────────────────────────────────────────────

func TestWrapTLSConfigForAIAFetching_Disabled(t *testing.T) {
	t.Parallel()

	h := &aiaHandler{}
	aiaSrv := startAIAServer(t, h)
	chain := newTestChain(t, aiaSrv.URL+"/ca.der")
	h.setCert(chain.intermediateDER)

	tlsSrv := chain.leafOnlyTLSServer(t)

	// No wrapping – behaves exactly like plain Go TLS.
	plainCfg := &tls.Config{RootCAs: chain.rootPool}
	resp, err := testHTTPClient(t, plainCfg).Get(tlsSrv.URL) //nolint:noctx
	closeBody(resp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "certificate signed by unknown authority",
		"without AIA fetching the incomplete chain must be rejected")
}

// ────────────────────────────────────────────────────────────────────────────
// (d) InsecureSkipVerify=true – wrapper must return config unchanged
// ────────────────────────────────────────────────────────────────────────────

func TestWrapTLSConfigForAIAFetching_InsecureSkipVerifyUnchanged(t *testing.T) {
	t.Parallel()

	cfg := &tls.Config{InsecureSkipVerify: true}
	result := WrapTLSConfigForAIAFetching(cfg, nullLogger(), nil)
	assert.Same(t, cfg, result, "wrapper must return the original config when InsecureSkipVerify is set")
}

// ────────────────────────────────────────────────────────────────────────────
// (b) Unreachable AIA URL (connection refused) – graceful failure, no hang
// ────────────────────────────────────────────────────────────────────────────

func TestWrapTLSConfigForAIAFetching_UnreachableAIAURL(t *testing.T) {
	t.Parallel()

	// Grab a free port, then close the listener so the port is not listening.
	ln, err := net.Listen("tcp", "127.0.0.1:0") //nolint:noctx
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	require.NoError(t, ln.Close())

	aiaURL := fmt.Sprintf("http://127.0.0.1:%d/ca.der", port)
	chain := newTestChain(t, aiaURL) // leaf cert points to closed port
	tlsSrv := chain.leafOnlyTLSServer(t)

	// Fast client so the connection-refused error surfaces quickly.
	fastClient := &http.Client{Timeout: 500 * time.Millisecond}
	wrappedCfg := WrapTLSConfigForAIAFetching(
		&tls.Config{RootCAs: chain.rootPool},
		nullLogger(),
		fastClient,
	)

	resp, err := testHTTPClient(t, wrappedCfg).Get(tlsSrv.URL) //nolint:noctx
	closeBody(resp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "certificate signed by unknown authority",
		"should fall back to the original x509 error when AIA URL is unreachable")
}

// ────────────────────────────────────────────────────────────────────────────
// (b) AIA fetch timeout – must fail quickly, not hang
// ────────────────────────────────────────────────────────────────────────────

func TestWrapTLSConfigForAIAFetching_FetchTimeout(t *testing.T) {
	t.Parallel()

	// HTTP server that accepts the connection and reads the request but never writes
	// a response – simulates a slow/stalled AIA endpoint.
	hangSrv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done() // block until client gives up
	}))
	t.Cleanup(hangSrv.Close)

	chain := newTestChain(t, hangSrv.URL+"/ca.der")
	tlsSrv := chain.leafOnlyTLSServer(t)

	const shortTimeout = 100 * time.Millisecond
	fastClient := &http.Client{Timeout: shortTimeout}
	wrappedCfg := WrapTLSConfigForAIAFetching(
		&tls.Config{RootCAs: chain.rootPool},
		nullLogger(),
		fastClient,
	)

	start := time.Now()
	resp, err := testHTTPClient(t, wrappedCfg).Get(tlsSrv.URL) //nolint:noctx
	elapsed := time.Since(start)
	closeBody(resp)

	require.Error(t, err)
	assert.Less(t, elapsed, 5*shortTimeout,
		"AIA fetch should time out quickly, not block for the full aiaFetchTimeout")
	assert.Contains(t, err.Error(), "certificate signed by unknown authority",
		"should report the original certificate error after the AIA timeout")
}

// ────────────────────────────────────────────────────────────────────────────
// (b) AIA server returns HTTP error – graceful failure
// ────────────────────────────────────────────────────────────────────────────

func TestWrapTLSConfigForAIAFetching_AIAReturnsHTTPError(t *testing.T) {
	t.Parallel()

	errSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not here", http.StatusServiceUnavailable)
	}))
	t.Cleanup(errSrv.Close)

	chain := newTestChain(t, errSrv.URL+"/ca.der")
	tlsSrv := chain.leafOnlyTLSServer(t)

	wrappedCfg := WrapTLSConfigForAIAFetching(
		&tls.Config{RootCAs: chain.rootPool},
		nullLogger(),
		nil,
	)

	resp, err := testHTTPClient(t, wrappedCfg).Get(tlsSrv.URL) //nolint:noctx
	closeBody(resp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "certificate signed by unknown authority")
}

// ────────────────────────────────────────────────────────────────────────────
// (c) AIA returns malformed / invalid certificate bytes
// ────────────────────────────────────────────────────────────────────────────

func TestWrapTLSConfigForAIAFetching_MalformedAIACert(t *testing.T) {
	t.Parallel()

	// Serve garbage bytes instead of a real certificate.
	garbageSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/pkix-cert")
		_, _ = fmt.Fprint(w, "this is not a certificate")
	}))
	t.Cleanup(garbageSrv.Close)

	chain := newTestChain(t, garbageSrv.URL+"/ca.der")
	tlsSrv := chain.leafOnlyTLSServer(t)

	wrappedCfg := WrapTLSConfigForAIAFetching(
		&tls.Config{RootCAs: chain.rootPool},
		nullLogger(),
		nil,
	)

	resp, err := testHTTPClient(t, wrappedCfg).Get(tlsSrv.URL) //nolint:noctx
	closeBody(resp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "certificate signed by unknown authority",
		"malformed AIA certificate should be silently ignored; original error is returned")
}

// ────────────────────────────────────────────────────────────────────────────
// (e) Circular AIA references – must terminate without looping
// ────────────────────────────────────────────────────────────────────────────

func TestWrapTLSConfigForAIAFetching_CircularAIAReferences(t *testing.T) {
	t.Parallel()

	// Build a "decoy" certificate whose AIA URL points back to the same server that
	// served it – the simplest circular reference (A → server → A → server → ...).
	// We need the server URL before creating the cert, so use a pointer trick:
	// start the server with a handler that reads from a shared variable.
	h := &aiaHandler{}
	circularSrv := startAIAServer(t, h)
	circularURL := circularSrv.URL + "/ca.der"

	// Build the chain using circularURL as the leaf's AIA URL.
	chain := newTestChain(t, circularURL)

	// Build a decoy cert (self-signed) whose own AIA also points to circularURL,
	// creating the cycle: leaf → fetch decoy → decoy → fetch decoy (seen → stop).
	decoyKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	decoySN, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	require.NoError(t, err)
	decoyTmpl := &x509.Certificate{
		SerialNumber:          decoySN,
		Subject:               pkix.Name{CommonName: "Circular Decoy"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IssuingCertificateURL: []string{circularURL}, // points back to itself
	}
	decoyDER, err := x509.CreateCertificate(rand.Reader, decoyTmpl, decoyTmpl, &decoyKey.PublicKey, decoyKey)
	require.NoError(t, err)
	h.setCert(decoyDER) // AIA server serves the decoy (not the real intermediate)

	tlsSrv := chain.leafOnlyTLSServer(t)

	// Use a fast client to bound the test duration in case the seen-map protection
	// were somehow bypassed (belt-and-suspenders).
	fastClient := &http.Client{Timeout: 2 * time.Second}
	wrappedCfg := WrapTLSConfigForAIAFetching(
		&tls.Config{RootCAs: chain.rootPool},
		nullLogger(),
		fastClient,
	)

	start := time.Now()
	resp, err := testHTTPClient(t, wrappedCfg).Get(tlsSrv.URL) //nolint:noctx
	elapsed := time.Since(start)
	closeBody(resp)

	require.Error(t, err, "circular AIA chain must not succeed")
	assert.Less(t, elapsed, 10*time.Second, "circular reference must terminate, not loop")
	assert.Contains(t, err.Error(), "certificate signed by unknown authority")
}

// ────────────────────────────────────────────────────────────────────────────
// Hostname verification is still enforced after successful AIA fetching
// ────────────────────────────────────────────────────────────────────────────

func TestWrapTLSConfigForAIAFetching_HostnameMismatchRejected(t *testing.T) {
	t.Parallel()

	h := &aiaHandler{}
	aiaSrv := startAIAServer(t, h)
	chain := newTestChain(t, aiaSrv.URL+"/ca.der")
	h.setCert(chain.intermediateDER)

	tlsSrv := chain.leafOnlyTLSServer(t)

	wrappedCfg := WrapTLSConfigForAIAFetching(
		&tls.Config{
			RootCAs:    chain.rootPool,
			ServerName: "wrong.example.com", // deliberately wrong
		},
		nullLogger(),
		nil,
	)

	resp, err := testHTTPClient(t, wrappedCfg).Get(tlsSrv.URL) //nolint:noctx
	closeBody(resp)
	require.Error(t, err, "wrong hostname must be rejected even after AIA succeeds")
}

// ────────────────────────────────────────────────────────────────────────────
// Benchmarks – TLS handshake overhead with and without AIA fetching
//
// Run with: go test -bench=BenchmarkTLSHandshake -benchtime=5s ./lib/netext/
//
// Three data points:
//   - Baseline   : full chain, no AIA wrapper (cost floor)
//   - WarmCache  : AIA wrapper, intermediate already cached (steady state)
//   - ColdCache  : AIA wrapper, cache evicted per iteration (first connection cost)
// ────────────────────────────────────────────────────────────────────────────

// newBenchClient returns an http.Client that forces a fresh TCP+TLS connection on every
// request: DisableKeepAlives prevents connection reuse, and SessionTicketsDisabled prevents
// abbreviated handshakes, so each iteration measures a full TLS handshake.
func newBenchClient(tlsCfg *tls.Config) *http.Client {
	cfg := tlsCfg.Clone()
	cfg.SessionTicketsDisabled = true
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig:   cfg,
			DisableKeepAlives: true,
		},
	}
}

// BenchmarkTLSHandshake_Baseline measures a plain TLS handshake with a complete chain and
// no AIA wrapping, establishing the cost floor.
func BenchmarkTLSHandshake_Baseline(b *testing.B) {
	h := &aiaHandler{}
	aiaSrv := startAIAServer(b, h)
	chain := newTestChain(b, aiaSrv.URL+"/ca.der")
	h.setCert(chain.intermediateDER)

	// Full chain server: leaf + intermediate, no AIA fetch ever needed.
	leafKeyDER, err := x509.MarshalECPrivateKey(chain.leafKey)
	require.NoError(b, err)
	fullChainCert, err := tls.X509KeyPair(
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: chain.leafCert.Raw}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: leafKeyDER}),
	)
	require.NoError(b, err)
	fullChainCert.Certificate = append(fullChainCert.Certificate, chain.intermediateCert.Raw)

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintln(w, "ok")
	}))
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{fullChainCert}}
	srv.StartTLS()
	b.Cleanup(srv.Close)

	client := newBenchClient(&tls.Config{RootCAs: chain.rootPool})

	b.ResetTimer()
	for b.Loop() {
		resp, err := client.Get(srv.URL) //nolint:noctx
		require.NoError(b, err)
		_ = resp.Body.Close()
	}
}

// BenchmarkTLSHandshake_AIAWarmCache measures a TLS handshake where AIA fetching is
// enabled but the intermediate is already cached — the steady-state cost for many VUs
// hitting the same server.
func BenchmarkTLSHandshake_AIAWarmCache(b *testing.B) {
	h := &aiaHandler{}
	aiaSrv := startAIAServer(b, h)
	aiaURL := aiaSrv.URL + "/ca.der"
	chain := newTestChain(b, aiaURL)
	h.setCert(chain.intermediateDER)

	tlsSrv := chain.leafOnlyTLSServer(b)

	wrappedCfg := WrapTLSConfigForAIAFetching(
		&tls.Config{RootCAs: chain.rootPool},
		nullLogger(),
		nil,
	)

	// Prime the cache with one request before timing starts.
	primer := newBenchClient(wrappedCfg)
	resp, err := primer.Get(tlsSrv.URL) //nolint:noctx
	require.NoError(b, err)
	_ = resp.Body.Close()

	client := newBenchClient(wrappedCfg)

	b.ResetTimer()
	for b.Loop() {
		resp, err := client.Get(tlsSrv.URL) //nolint:noctx
		require.NoError(b, err)
		_ = resp.Body.Close()
	}
}

// BenchmarkTLSHandshake_AIAColdCache measures the worst-case cost: AIA fetching enabled
// and the intermediate is not cached, so each handshake triggers an HTTP round-trip to
// the AIA endpoint (first connection to a new server).
func BenchmarkTLSHandshake_AIAColdCache(b *testing.B) {
	h := &aiaHandler{}
	aiaSrv := startAIAServer(b, h)
	aiaURL := aiaSrv.URL + "/ca.der"
	chain := newTestChain(b, aiaURL)
	h.setCert(chain.intermediateDER)

	tlsSrv := chain.leafOnlyTLSServer(b)

	wrappedCfg := WrapTLSConfigForAIAFetching(
		&tls.Config{RootCAs: chain.rootPool},
		nullLogger(),
		nil,
	)

	client := newBenchClient(wrappedCfg)

	b.ResetTimer()
	for b.Loop() {
		// Evict the cached intermediate so each iteration pays the full HTTP fetch cost.
		aiaIntermediateCache.Delete(aiaURL)

		resp, err := client.Get(tlsSrv.URL) //nolint:noctx
		require.NoError(b, err)
		_ = resp.Body.Close()
	}
}
