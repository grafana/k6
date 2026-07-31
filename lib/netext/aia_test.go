package netext

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/lib/testutils/tlstest"
)

func buildChainWithAIA(t testing.TB, aiaURL string) *tlstest.Chain {
	t.Helper()
	c := tlstest.NewChain(t)
	c.SetAIAURL(aiaURL)
	return c
}

// leafOnlyTLSServer serves only the leaf certificate — clients that need the intermediate
// see an incomplete chain.
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

func startAIAServer(t testing.TB, h http.Handler) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func testHTTPClient(t testing.TB, tlsCfg *tls.Config) *http.Client {
	t.Helper()
	return &http.Client{Transport: &http.Transport{TLSClientConfig: tlsCfg}}
}

func nullLogger() logrus.FieldLogger {
	l := logrus.New()
	l.SetOutput(nil)
	l.SetLevel(logrus.PanicLevel)
	return l
}

func closeBody(resp *http.Response) {
	if resp != nil {
		_ = resp.Body.Close()
	}
}

func TestWrapTLSConfigForAIAFetching_HappyPath(t *testing.T) {
	t.Parallel()

	h := &tlstest.AIAHandler{}
	aiaSrv := startAIAServer(t, h)
	chain := buildChainWithAIA(t, aiaSrv.URL+"/ca.der")
	h.SetCert(chain.IntermediateDER)

	tlsSrv := leafOnlyTLSServer(t, chain)

	wrappedCfg := NewAIAFetcher(nil).Wrap(&tls.Config{RootCAs: chain.RootPool}, nullLogger())
	resp, err := testHTTPClient(t, wrappedCfg).Get(tlsSrv.URL) //nolint:noctx
	require.NoError(t, err, "AIA fetching should resolve the incomplete chain")
	_ = resp.Body.Close()
}

func TestWrapTLSConfigForAIAFetching_CompleteChainPassesThrough(t *testing.T) {
	t.Parallel()

	h := &tlstest.AIAHandler{}
	aiaSrv := startAIAServer(t, h)
	chain := buildChainWithAIA(t, aiaSrv.URL+"/ca.der")
	h.SetCert(chain.IntermediateDER)

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintln(w, "ok")
	}))
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{chain.FullChainTLSCertificate(t)}}
	srv.StartTLS()
	t.Cleanup(srv.Close)

	wrappedCfg := NewAIAFetcher(nil).Wrap(&tls.Config{RootCAs: chain.RootPool}, nullLogger())
	resp, err := testHTTPClient(t, wrappedCfg).Get(srv.URL) //nolint:noctx
	require.NoError(t, err, "complete chain should succeed without any AIA fetch")
	_ = resp.Body.Close()
}

func TestWrapTLSConfigForAIAFetching_Disabled(t *testing.T) {
	t.Parallel()

	h := &tlstest.AIAHandler{}
	aiaSrv := startAIAServer(t, h)
	chain := buildChainWithAIA(t, aiaSrv.URL+"/ca.der")
	h.SetCert(chain.IntermediateDER)

	tlsSrv := leafOnlyTLSServer(t, chain)

	plainCfg := &tls.Config{RootCAs: chain.RootPool}
	resp, err := testHTTPClient(t, plainCfg).Get(tlsSrv.URL) //nolint:noctx
	closeBody(resp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "certificate signed by unknown authority",
		"without AIA fetching the incomplete chain must be rejected")
}

func TestWrapTLSConfigForAIAFetching_InsecureSkipVerifyUnchanged(t *testing.T) {
	t.Parallel()

	cfg := &tls.Config{InsecureSkipVerify: true}
	result := NewAIAFetcher(nil).Wrap(cfg, nullLogger())
	assert.Same(t, cfg, result, "wrapper must return the original config when InsecureSkipVerify is set")
}

func TestWrapTLSConfigForAIAFetching_UnreachableAIAURL(t *testing.T) {
	t.Parallel()

	// Grab a free port then close it — connections will be refused.
	ln, err := net.Listen("tcp", "127.0.0.1:0") //nolint:noctx
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	require.NoError(t, ln.Close())

	aiaURL := fmt.Sprintf("http://127.0.0.1:%d/ca.der", port)
	chain := buildChainWithAIA(t, aiaURL)
	tlsSrv := leafOnlyTLSServer(t, chain)

	fastClient := &http.Client{Timeout: 500 * time.Millisecond}
	wrappedCfg := (&AIAFetcher{httpClient: fastClient}).Wrap(
		&tls.Config{RootCAs: chain.RootPool},
		nullLogger(),
	)

	resp, err := testHTTPClient(t, wrappedCfg).Get(tlsSrv.URL) //nolint:noctx
	closeBody(resp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "certificate signed by unknown authority",
		"should fall back to the original x509 error when AIA URL is unreachable")
}

func TestWrapTLSConfigForAIAFetching_FetchTimeout(t *testing.T) {
	t.Parallel()

	// Accept the request but never respond — simulates a stalled AIA endpoint.
	hangSrv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(hangSrv.Close)

	chain := buildChainWithAIA(t, hangSrv.URL+"/ca.der")
	tlsSrv := leafOnlyTLSServer(t, chain)

	const shortTimeout = 100 * time.Millisecond
	fastClient := &http.Client{Timeout: shortTimeout}
	wrappedCfg := (&AIAFetcher{httpClient: fastClient}).Wrap(
		&tls.Config{RootCAs: chain.RootPool},
		nullLogger(),
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

func TestWrapTLSConfigForAIAFetching_AIAReturnsHTTPError(t *testing.T) {
	t.Parallel()

	errSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not here", http.StatusServiceUnavailable)
	}))
	t.Cleanup(errSrv.Close)

	chain := buildChainWithAIA(t, errSrv.URL+"/ca.der")
	tlsSrv := leafOnlyTLSServer(t, chain)

	wrappedCfg := NewAIAFetcher(nil).Wrap(&tls.Config{RootCAs: chain.RootPool}, nullLogger())

	resp, err := testHTTPClient(t, wrappedCfg).Get(tlsSrv.URL) //nolint:noctx
	closeBody(resp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "certificate signed by unknown authority")
}

func TestWrapTLSConfigForAIAFetching_MalformedAIACert(t *testing.T) {
	t.Parallel()

	garbageSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/pkix-cert")
		_, _ = fmt.Fprint(w, "this is not a certificate")
	}))
	t.Cleanup(garbageSrv.Close)

	chain := buildChainWithAIA(t, garbageSrv.URL+"/ca.der")
	tlsSrv := leafOnlyTLSServer(t, chain)

	wrappedCfg := NewAIAFetcher(nil).Wrap(&tls.Config{RootCAs: chain.RootPool}, nullLogger())

	resp, err := testHTTPClient(t, wrappedCfg).Get(tlsSrv.URL) //nolint:noctx
	closeBody(resp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "certificate signed by unknown authority",
		"malformed AIA certificate should be silently ignored; original error is returned")
}

func TestFetchCertFromAIAURL_PKCS7ResponseLogsWarn(t *testing.T) {
	t.Parallel()

	// Body embeds the ASN.1 signedData OID so the heuristic fires without relying on Content-Type.
	pkcs7Body := append([]byte{0x30, 0x82, 0x00, 0x00}, []byte{0x2A, 0x86, 0x48, 0x86, 0xF7, 0x0D, 0x01, 0x07, 0x02}...)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/pkcs7-mime")
		_, _ = w.Write(pkcs7Body)
	}))
	t.Cleanup(srv.Close)

	logger, hook := logtest.NewNullLogger()
	logger.SetLevel(logrus.WarnLevel)

	cert, err := NewAIAFetcher(nil).fetchCertFromAIAURL(srv.URL, logger)
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

func TestWrapTLSConfigForAIAFetching_CircularAIAReferences(t *testing.T) {
	t.Parallel()

	// Cycle: leaf → fetch decoy from server → decoy AIA points to same server → seen-map stops it.
	h := &tlstest.AIAHandler{}
	circularSrv := startAIAServer(t, h)
	circularURL := circularSrv.URL + "/ca.der"
	chain := buildChainWithAIA(t, circularURL)

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

	fastClient := &http.Client{Timeout: 2 * time.Second}
	wrappedCfg := (&AIAFetcher{httpClient: fastClient}).Wrap(
		&tls.Config{RootCAs: chain.RootPool},
		nullLogger(),
	)

	start := time.Now()
	resp, err := testHTTPClient(t, wrappedCfg).Get(tlsSrv.URL) //nolint:noctx
	elapsed := time.Since(start)
	closeBody(resp)

	require.Error(t, err, "circular AIA chain must not succeed")
	assert.Less(t, elapsed, 10*time.Second, "circular reference must terminate, not loop")
	assert.Contains(t, err.Error(), "certificate signed by unknown authority")
}

// AIA fetches must go through the dialer passed to NewAIAFetcher — a dialer that always
// errors makes the AIA fetch fail, leaving the chain incomplete.
func TestAIAFetcher_UsesProvidedDialer(t *testing.T) {
	t.Parallel()

	h := &tlstest.AIAHandler{}
	aiaSrv := startAIAServer(t, h)
	chain := buildChainWithAIA(t, aiaSrv.URL+"/ca.der")
	h.SetCert(chain.IntermediateDER)

	tlsSrv := leafOnlyTLSServer(t, chain)

	var dialCount int
	failingDial := func(_ context.Context, _, _ string) (net.Conn, error) {
		dialCount++
		return nil, errors.New("dial refused by test")
	}

	wrappedCfg := NewAIAFetcher(failingDial).Wrap(
		&tls.Config{RootCAs: chain.RootPool},
		nullLogger(),
	)
	resp, err := testHTTPClient(t, wrappedCfg).Get(tlsSrv.URL) //nolint:noctx
	closeBody(resp)
	require.Error(t, err, "AIA fetch must use the provided dialer; when it fails, chain verification fails")
	assert.Contains(t, err.Error(), "certificate signed by unknown authority")
	assert.Positive(t, dialCount, "expected the provided dialer to be called for the AIA fetch")
}

// Two fetchers must not share cache or singleflight state — one Runner's AIA state
// must not leak into another Runner running in the same process.
func TestAIAFetcher_CacheIsolatedBetweenFetchers(t *testing.T) {
	t.Parallel()

	var aiaHits int32
	handler := &tlstest.AIAHandler{}
	aiaSrv := startAIAServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&aiaHits, 1)
		handler.ServeHTTP(w, r)
	}))
	chain := buildChainWithAIA(t, aiaSrv.URL+"/ca.der")
	handler.SetCert(chain.IntermediateDER)

	tlsSrv := leafOnlyTLSServer(t, chain)

	fetcherA := NewAIAFetcher(nil)
	fetcherB := NewAIAFetcher(nil)

	// Fetcher A warms its cache.
	respA, errA := testHTTPClient(t, fetcherA.Wrap(&tls.Config{RootCAs: chain.RootPool}, nullLogger())).Get(tlsSrv.URL) //nolint:noctx
	require.NoError(t, errA)
	_ = respA.Body.Close()
	hitsAfterA := atomic.LoadInt32(&aiaHits)
	require.Equal(t, int32(1), hitsAfterA, "fetcher A should have fetched once")

	// Fetcher B, independently, must hit the AIA server again — no shared cache.
	respB, errB := testHTTPClient(t, fetcherB.Wrap(&tls.Config{RootCAs: chain.RootPool}, nullLogger())).Get(tlsSrv.URL) //nolint:noctx
	require.NoError(t, errB)
	_ = respB.Body.Close()
	hitsAfterB := atomic.LoadInt32(&aiaHits)
	assert.Equal(t, int32(2), hitsAfterB, "fetcher B must fetch independently; caches are not shared across fetchers")
}

func TestWrapTLSConfigForAIAFetching_HostnameMismatchRejected(t *testing.T) {
	t.Parallel()

	h := &tlstest.AIAHandler{}
	aiaSrv := startAIAServer(t, h)
	chain := buildChainWithAIA(t, aiaSrv.URL+"/ca.der")
	h.SetCert(chain.IntermediateDER)

	tlsSrv := leafOnlyTLSServer(t, chain)

	wrappedCfg := NewAIAFetcher(nil).Wrap(
		&tls.Config{
			RootCAs:    chain.RootPool,
			ServerName: "wrong.example.com", // deliberately wrong
		},
		nullLogger(),
	)

	resp, err := testHTTPClient(t, wrappedCfg).Get(tlsSrv.URL) //nolint:noctx
	closeBody(resp)
	require.Error(t, err, "wrong hostname must be rejected even after AIA succeeds")
}

// newBenchClient forces a fresh TCP+TLS handshake per request (no keep-alive, no session tickets).
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

// BenchmarkTLSHandshake_Baseline: complete chain, no AIA wrapper — cost floor.
func BenchmarkTLSHandshake_Baseline(b *testing.B) {
	h := &tlstest.AIAHandler{}
	aiaSrv := startAIAServer(b, h)
	chain := buildChainWithAIA(b, aiaSrv.URL+"/ca.der")
	h.SetCert(chain.IntermediateDER)

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

// BenchmarkTLSHandshake_AIANotNeeded: AIA wrapper active, but server sends full chain so
// no HTTP fetch is triggered.
func BenchmarkTLSHandshake_AIANotNeeded(b *testing.B) {
	h := &tlstest.AIAHandler{}
	aiaSrv := startAIAServer(b, h)
	chain := buildChainWithAIA(b, aiaSrv.URL+"/ca.der")
	h.SetCert(chain.IntermediateDER)

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintln(w, "ok")
	}))
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{chain.FullChainTLSCertificate(b)}}
	srv.StartTLS()
	b.Cleanup(srv.Close)

	wrappedCfg := NewAIAFetcher(nil).Wrap(&tls.Config{RootCAs: chain.RootPool}, nullLogger())
	client := newBenchClient(wrappedCfg)

	b.ResetTimer()
	for b.Loop() {
		resp, err := client.Get(srv.URL) //nolint:noctx
		require.NoError(b, err)
		_ = resp.Body.Close()
	}
}

// BenchmarkTLSHandshake_AIAWarmCache: steady-state cost when the intermediate is cached.
func BenchmarkTLSHandshake_AIAWarmCache(b *testing.B) {
	h := &tlstest.AIAHandler{}
	aiaSrv := startAIAServer(b, h)
	aiaURL := aiaSrv.URL + "/ca.der"
	chain := buildChainWithAIA(b, aiaURL)
	h.SetCert(chain.IntermediateDER)

	tlsSrv := leafOnlyTLSServer(b, chain)

	wrappedCfg := NewAIAFetcher(nil).Wrap(&tls.Config{RootCAs: chain.RootPool}, nullLogger())

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

// BenchmarkTLSHandshake_AIAColdCache: worst-case, every handshake triggers an AIA fetch.
func BenchmarkTLSHandshake_AIAColdCache(b *testing.B) {
	h := &tlstest.AIAHandler{}
	aiaSrv := startAIAServer(b, h)
	aiaURL := aiaSrv.URL + "/ca.der"
	chain := buildChainWithAIA(b, aiaURL)
	h.SetCert(chain.IntermediateDER)

	tlsSrv := leafOnlyTLSServer(b, chain)

	fetcher := NewAIAFetcher(nil)
	wrappedCfg := fetcher.Wrap(&tls.Config{RootCAs: chain.RootPool}, nullLogger())

	client := newBenchClient(wrappedCfg)

	b.ResetTimer()
	for b.Loop() {
		fetcher.cache.Delete(aiaURL) // force a cold fetch on every iteration

		resp, err := client.Get(tlsSrv.URL) //nolint:noctx
		require.NoError(b, err)
		_ = resp.Body.Close()
	}
}
