package netext

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/lib/testutils/tlstest"
)

// ────────────────────────────────────────────────────────────────────────────
// Test helpers
// ────────────────────────────────────────────────────────────────────────────

// buildChainWithAIA is a small local shim: creates a tlstest.Chain and bakes aiaURL into
// the leaf template so both the AIA URL and the leaf DER are ready in one call.
func buildChainWithAIA(t testing.TB, aiaURL string) *tlstest.Chain {
	t.Helper()
	c := tlstest.NewChain(t)
	c.SetAIAURL(aiaURL)
	return c
}

// leafOnlyTLSServer starts an HTTPS server that sends only the leaf certificate.
func leafOnlyTLSServer(t testing.TB, c *tlstest.Chain) *httptest.Server {
	t.Helper()
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintln(w, "ok")
	}))
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{c.LeafTLSCertificate(t)}}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return srv
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

	h := &tlstest.AIAHandler{}
	aiaSrv := startAIAServer(t, h)
	chain := buildChainWithAIA(t, aiaSrv.URL+"/ca.der")
	h.SetCert(chain.IntermediateDER) // serve real intermediate

	tlsSrv := leafOnlyTLSServer(t, chain)

	wrappedCfg := WrapTLSConfigForAIAFetching(
		&tls.Config{RootCAs: chain.RootPool},
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

	h := &tlstest.AIAHandler{}
	aiaSrv := startAIAServer(t, h)
	chain := buildChainWithAIA(t, aiaSrv.URL+"/ca.der")
	h.SetCert(chain.IntermediateDER)

	// Build a TLS server that sends the FULL chain: leaf + intermediate.
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintln(w, "ok")
	}))
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{chain.FullChainTLSCertificate(t)}}
	srv.StartTLS()
	t.Cleanup(srv.Close)

	wrappedCfg := WrapTLSConfigForAIAFetching(
		&tls.Config{RootCAs: chain.RootPool},
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

	h := &tlstest.AIAHandler{}
	aiaSrv := startAIAServer(t, h)
	chain := buildChainWithAIA(t, aiaSrv.URL+"/ca.der")
	h.SetCert(chain.IntermediateDER)

	tlsSrv := leafOnlyTLSServer(t, chain)

	// No wrapping – behaves exactly like plain Go TLS.
	plainCfg := &tls.Config{RootCAs: chain.RootPool}
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
	chain := buildChainWithAIA(t, aiaURL) // leaf cert points to closed port
	tlsSrv := leafOnlyTLSServer(t, chain)

	// Fast client so the connection-refused error surfaces quickly.
	fastClient := &http.Client{Timeout: 500 * time.Millisecond}
	wrappedCfg := WrapTLSConfigForAIAFetching(
		&tls.Config{RootCAs: chain.RootPool},
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

	chain := buildChainWithAIA(t, hangSrv.URL+"/ca.der")
	tlsSrv := leafOnlyTLSServer(t, chain)

	const shortTimeout = 100 * time.Millisecond
	fastClient := &http.Client{Timeout: shortTimeout}
	wrappedCfg := WrapTLSConfigForAIAFetching(
		&tls.Config{RootCAs: chain.RootPool},
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

	chain := buildChainWithAIA(t, errSrv.URL+"/ca.der")
	tlsSrv := leafOnlyTLSServer(t, chain)

	wrappedCfg := WrapTLSConfigForAIAFetching(
		&tls.Config{RootCAs: chain.RootPool},
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

	chain := buildChainWithAIA(t, garbageSrv.URL+"/ca.der")
	tlsSrv := leafOnlyTLSServer(t, chain)

	wrappedCfg := WrapTLSConfigForAIAFetching(
		&tls.Config{RootCAs: chain.RootPool},
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
// (d.1) PKCS#7 AIA responses are not supported; a Warn log must fire so users
// can diagnose "AIA didn't work" for chains served in that format.
// ────────────────────────────────────────────────────────────────────────────

func TestFetchCertFromAIAURL_PKCS7ResponseLogsWarn(t *testing.T) {
	t.Parallel()

	// Body starts with the ASN.1 signedData OID so the heuristic fires even if
	// the server's Content-Type is missing or misleading.
	pkcs7Body := append([]byte{0x30, 0x82, 0x00, 0x00}, []byte{0x2A, 0x86, 0x48, 0x86, 0xF7, 0x0D, 0x01, 0x07, 0x02}...)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/pkcs7-mime")
		_, _ = w.Write(pkcs7Body)
	}))
	t.Cleanup(srv.Close)

	logger, hook := logtest.NewNullLogger()
	logger.SetLevel(logrus.WarnLevel)

	cert, err := fetchCertFromAIAURL(srv.URL, aiaHTTPClient, logger)
	require.Error(t, err)
	assert.Nil(t, cert)

	var warned bool
	for _, entry := range hook.AllEntries() {
		if entry.Level == logrus.WarnLevel && strings.Contains(entry.Message, "PKCS#7") {
			warned = true
			break
		}
	}
	assert.True(t, warned, "expected a Warn log about PKCS#7 not being supported")
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
	h := &tlstest.AIAHandler{}
	circularSrv := startAIAServer(t, h)
	circularURL := circularSrv.URL + "/ca.der"

	// Build the chain using circularURL as the leaf's AIA URL.
	chain := buildChainWithAIA(t, circularURL)

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
	h.SetCert(decoyDER) // AIA server serves the decoy (not the real intermediate)

	tlsSrv := leafOnlyTLSServer(t, chain)

	// Use a fast client to bound the test duration in case the seen-map protection
	// were somehow bypassed (belt-and-suspenders).
	fastClient := &http.Client{Timeout: 2 * time.Second}
	wrappedCfg := WrapTLSConfigForAIAFetching(
		&tls.Config{RootCAs: chain.RootPool},
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

	h := &tlstest.AIAHandler{}
	aiaSrv := startAIAServer(t, h)
	chain := buildChainWithAIA(t, aiaSrv.URL+"/ca.der")
	h.SetCert(chain.IntermediateDER)

	tlsSrv := leafOnlyTLSServer(t, chain)

	wrappedCfg := WrapTLSConfigForAIAFetching(
		&tls.Config{
			RootCAs:    chain.RootPool,
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
// Four data points:
//   - Baseline      : full chain, no AIA wrapper (cost floor)
//   - AIANotNeeded  : AIA wrapper enabled, server sends full chain — no fetch triggered
//   - WarmCache     : AIA wrapper, incomplete chain, intermediate already cached (steady state)
//   - ColdCache     : AIA wrapper, incomplete chain, cache evicted per iteration (first connection cost)
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
	h := &tlstest.AIAHandler{}
	aiaSrv := startAIAServer(b, h)
	chain := buildChainWithAIA(b, aiaSrv.URL+"/ca.der")
	h.SetCert(chain.IntermediateDER)

	// Full chain server: leaf + intermediate, no AIA fetch ever needed.
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintln(w, "ok")
	}))
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{chain.FullChainTLSCertificate(b)}}
	srv.StartTLS()
	b.Cleanup(srv.Close)

	client := newBenchClient(&tls.Config{RootCAs: chain.RootPool})

	b.ResetTimer()
	for b.Loop() {
		resp, err := client.Get(srv.URL) //nolint:noctx
		require.NoError(b, err)
		_ = resp.Body.Close()
	}
}

// BenchmarkTLSHandshake_AIANotNeeded measures a TLS handshake where AIA fetching is enabled
// but the server sends a complete certificate chain, so verification succeeds on the first
// attempt and no HTTP fetch is ever triggered.
func BenchmarkTLSHandshake_AIANotNeeded(b *testing.B) {
	h := &tlstest.AIAHandler{}
	aiaSrv := startAIAServer(b, h)
	chain := buildChainWithAIA(b, aiaSrv.URL+"/ca.der")
	h.SetCert(chain.IntermediateDER)

	// Full chain server: leaf + intermediate — same as Baseline, but with AIA wrapper active.
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintln(w, "ok")
	}))
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{chain.FullChainTLSCertificate(b)}}
	srv.StartTLS()
	b.Cleanup(srv.Close)

	wrappedCfg := WrapTLSConfigForAIAFetching(
		&tls.Config{RootCAs: chain.RootPool},
		nullLogger(),
		nil,
	)
	client := newBenchClient(wrappedCfg)

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
	h := &tlstest.AIAHandler{}
	aiaSrv := startAIAServer(b, h)
	aiaURL := aiaSrv.URL + "/ca.der"
	chain := buildChainWithAIA(b, aiaURL)
	h.SetCert(chain.IntermediateDER)

	tlsSrv := leafOnlyTLSServer(b, chain)

	wrappedCfg := WrapTLSConfigForAIAFetching(
		&tls.Config{RootCAs: chain.RootPool},
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
	h := &tlstest.AIAHandler{}
	aiaSrv := startAIAServer(b, h)
	aiaURL := aiaSrv.URL + "/ca.der"
	chain := buildChainWithAIA(b, aiaURL)
	h.SetCert(chain.IntermediateDER)

	tlsSrv := leafOnlyTLSServer(b, chain)

	wrappedCfg := WrapTLSConfigForAIAFetching(
		&tls.Config{RootCAs: chain.RootPool},
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
