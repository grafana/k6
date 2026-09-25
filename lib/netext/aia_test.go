package netext

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/pem"
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

// Rewrites 127.0.0.1 → localhost so the wrapper sees a non-empty ServerName (Go strips
// IPs from SNI, and the wrapper fails closed on empty). The test chain's leaf has
// "localhost" as a DNS SAN.
func leafOnlyTLSServerURL(srv *httptest.Server) string {
	return strings.Replace(srv.URL, "127.0.0.1", "localhost", 1)
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
	resp, err := testHTTPClient(t, wrappedCfg).Get(leafOnlyTLSServerURL(tlsSrv)) //nolint:noctx
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
	resp, err := testHTTPClient(t, wrappedCfg).Get(strings.Replace(srv.URL, "127.0.0.1", "localhost", 1)) //nolint:noctx
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

// A response that looks like PKCS#7 (Content-Type or embedded signedData OID)
// but does not parse must warn — chain stays incomplete — instead of failing
// silently.
func TestFetchCertFromAIAURL_InvalidPKCS7ResponseLogsWarn(t *testing.T) {
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

	certs, err := NewAIAFetcher(nil).fetchCertFromAIAURL(srv.URL, logger)
	require.Error(t, err)
	assert.Nil(t, certs)
	assert.Contains(t, err.Error(), "PKCS#7", "error should mention what was tried")

	var warned bool
	for _, entry := range hook.AllEntries() {
		if entry.Level == logrus.WarnLevel && strings.Contains(entry.Message, "PKCS#7") {
			warned = true
			break
		}
	}
	assert.True(t, warned, "expected a Warn log about the unparseable PKCS#7 response")
}

// A bare PEM certificate (the plain form most AIA endpoints serve) must keep
// working through the refactored response parsing.
func TestFetchCertFromAIAURL_PEMCertificate(t *testing.T) {
	t.Parallel()

	chain := tlstest.NewChain(t)
	pemBody := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: chain.IntermediateDER})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-pem-file")
		_, _ = w.Write(pemBody)
	}))
	t.Cleanup(srv.Close)

	certs, err := NewAIAFetcher(nil).fetchCertFromAIAURL(srv.URL, nullLogger())
	require.NoError(t, err)
	require.Len(t, certs, 1)
	assert.Equal(t, chain.IntermediateDER, certs[0].Raw)
}

// A PEM block that is neither a certificate nor PKCS#7 must fail with a clear
// error instead of attempting to parse arbitrary payload as a certificate.
func TestFetchCertFromAIAURL_UnexpectedPEMBlockType(t *testing.T) {
	t.Parallel()

	pemBody := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("definitely not a certificate")})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-pem-file")
		_, _ = w.Write(pemBody)
	}))
	t.Cleanup(srv.Close)

	certs, err := NewAIAFetcher(nil).fetchCertFromAIAURL(srv.URL, nullLogger())
	require.Error(t, err)
	assert.Nil(t, certs)
	assert.Contains(t, err.Error(), "unexpected PEM block type")
}

// The full happy path from the issue: a real certs-only .p7c bundle served from
// the AIA URL resolves a leaf-only chain (#6146).
func TestWrapTLSConfigForAIAFetching_PKCS7Bundle(t *testing.T) {
	t.Parallel()

	h := &tlstest.AIAHandler{}
	aiaSrv := startAIAServer(t, h)
	chain := buildChainWithAIA(t, aiaSrv.URL+"/ca.p7c")
	h.SetPKCS7(tlstest.BuildPKCS7Bundle(t, chain.IntermediateCert))

	tlsSrv := leafOnlyTLSServer(t, chain)

	wrappedCfg := NewAIAFetcher(nil).Wrap(&tls.Config{RootCAs: chain.RootPool}, nullLogger())
	resp, err := testHTTPClient(t, wrappedCfg).Get(leafOnlyTLSServerURL(tlsSrv)) //nolint:noctx
	require.NoError(t, err, "PKCS#7 AIA response should resolve the incomplete chain")
	_ = resp.Body.Close()
}

// Real-world bundles often carry more than one certificate; an unrelated extra
// certificate must not stop the useful intermediate from completing the chain.
func TestWrapTLSConfigForAIAFetching_PKCS7BundleWithUnrelatedCerts(t *testing.T) {
	t.Parallel()

	h := &tlstest.AIAHandler{}
	aiaSrv := startAIAServer(t, h)
	chain := buildChainWithAIA(t, aiaSrv.URL+"/ca.p7c")
	h.SetPKCS7(tlstest.BuildPKCS7Bundle(t, tlstest.NewChain(t).RootCert, chain.IntermediateCert))

	tlsSrv := leafOnlyTLSServer(t, chain)

	wrappedCfg := NewAIAFetcher(nil).Wrap(&tls.Config{RootCAs: chain.RootPool}, nullLogger())
	resp, err := testHTTPClient(t, wrappedCfg).Get(leafOnlyTLSServerURL(tlsSrv)) //nolint:noctx
	require.NoError(t, err, "PKCS#7 bundle with extra unrelated certificate should still resolve the chain")
	_ = resp.Body.Close()
}

// Some servers PEM-armor the PKCS#7 bundle instead of serving bare DER.
func TestFetchCertFromAIAURL_PKCS7PEMArmored(t *testing.T) {
	t.Parallel()

	chain := tlstest.NewChain(t)
	pemBody := pem.EncodeToMemory(&pem.Block{
		Type:  "PKCS7",
		Bytes: tlstest.BuildPKCS7Bundle(t, chain.IntermediateCert),
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-pem-file")
		_, _ = w.Write(pemBody)
	}))
	t.Cleanup(srv.Close)

	certs, err := NewAIAFetcher(nil).fetchCertFromAIAURL(srv.URL, nullLogger())
	require.NoError(t, err)
	require.Len(t, certs, 1)
	assert.Equal(t, chain.IntermediateDER, certs[0].Raw)
}

// The PKCS#7 path must feed the URL-keyed cache exactly like the DER path:
// after the first handshake resolves the chain, later handshakes hit the cache.
func TestAIAFetcher_PKCS7BundleCachedAcrossHandshakes(t *testing.T) {
	t.Parallel()

	var aiaHits atomic.Int32
	handler := &tlstest.AIAHandler{}
	aiaSrv := startAIAServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		aiaHits.Add(1)
		handler.ServeHTTP(w, r)
	}))
	chain := buildChainWithAIA(t, aiaSrv.URL+"/ca.p7c")
	handler.SetPKCS7(tlstest.BuildPKCS7Bundle(t, chain.IntermediateCert))

	tlsSrv := leafOnlyTLSServer(t, chain)

	wrappedCfg := NewAIAFetcher(nil).Wrap(&tls.Config{RootCAs: chain.RootPool}, nullLogger())
	client := newBenchClient(wrappedCfg) // fresh handshake per request

	for i := range 2 {
		resp, err := client.Get(leafOnlyTLSServerURL(tlsSrv)) //nolint:noctx
		require.NoError(t, err, "handshake %d should complete via PKCS#7 AIA fetch", i+1)
		_ = resp.Body.Close()
	}
	assert.EqualValues(t, 1, aiaHits.Load(), "the second handshake must be served from the AIA cache")
}

func TestParsePKCS7Certificates(t *testing.T) {
	t.Parallel()

	chain := tlstest.NewChain(t)
	oidData, err := asn1.Marshal(asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 1})
	require.NoError(t, err)
	oidSignedData, err := asn1.Marshal(asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 2})
	require.NoError(t, err)

	t.Run("single certificate", func(t *testing.T) {
		t.Parallel()
		certs, err := parsePKCS7Certificates(tlstest.BuildPKCS7Bundle(t, chain.IntermediateCert))
		require.NoError(t, err)
		require.Len(t, certs, 1)
		assert.Equal(t, chain.IntermediateDER, certs[0].Raw)
	})

	t.Run("multiple certificates preserve bundle order", func(t *testing.T) {
		t.Parallel()
		certs, err := parsePKCS7Certificates(tlstest.BuildPKCS7Bundle(t, chain.IntermediateCert, chain.RootCert))
		require.NoError(t, err)
		require.Len(t, certs, 2)
		assert.Equal(t, chain.IntermediateDER, certs[0].Raw)
		assert.Equal(t, chain.RootDER, certs[1].Raw)
	})

	t.Run("non-signedData content type is rejected", func(t *testing.T) {
		t.Parallel()
		_, err := parsePKCS7Certificates(testDER(0x30, oidData))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported PKCS#7 content type")
	})

	t.Run("bundle without certificates is rejected", func(t *testing.T) {
		t.Parallel()
		signedData := testDER(0x30,
			[]byte{asn1.TagInteger, 0x01, 0x01},
			testDER(0x31),
			testDER(0x30, oidData),
		)
		bundle := testDER(0x30, oidSignedData, testDER(0xA0, signedData))
		_, err := parsePKCS7Certificates(bundle)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no certificates")
	})

	t.Run("trailing bytes after content info are rejected", func(t *testing.T) {
		t.Parallel()
		bundle := append(tlstest.BuildPKCS7Bundle(t, chain.IntermediateCert), 0x00)
		_, err := parsePKCS7Certificates(bundle)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "trailing")
	})

	t.Run("garbage input is rejected", func(t *testing.T) {
		t.Parallel()
		_, err := parsePKCS7Certificates([]byte("not a bundle"))
		require.Error(t, err)
	})
}

// A bundle produced by a real PKCS#7 implementation must parse, and
// tlstest.BuildPKCS7Bundle must stay byte-identical to it, so the test fixtures
// keep exercising the exact wire format production CAs emit.
//
// Fixture: `openssl crl2pkcs7 -nocrl -certfile probe.pem -outform DER` over a
// throwaway self-signed P-256 certificate (CN=probe). The test only parses the
// bundle, so the fixture certificate's expiry is irrelevant.
func TestParsePKCS7Certificates_OpensslBundle(t *testing.T) {
	t.Parallel()

	bundle, err := base64.StdEncoding.DecodeString(
		"MIIBpAYJKoZIhvcNAQcCoIIBlTCCAZECAQExADALBgkqhkiG9w0BBwGgggF5MIIBdTCCARugAwIBAgIUTvyW" +
			"dq5FVFR1VRcDQLKalvVEwEEwCgYIKoZIzj0EAwIwEDEOMAwGA1UEAwwFcHJvYmUwHhcNMjYwOTIyMTkxNzUz" +
			"WhcNMjYwOTIzMTkxNzUzWjAQMQ4wDAYDVQQDDAVwcm9iZTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABMbd" +
			"TxvHzs32qCd5sBn4dBAu9OMdgYQ8d/xegeQcWgGMhssrq0Yi7ej9mRi6kCb3KTGoGsgy7x1rnc6VOd43bqaj" +
			"UzBRMB0GA1UdDgQWBBQUR4wuAnfZbkfqEM09gXBk3zWZpTAfBgNVHSMEGDAWgBQUR4wuAnfZbkfqEM09gXBk" +
			"3zWZpTAPBgNVHRMBAf8EBTADAQH/MAoGCCqGSM49BAMCA0gAMEUCIAgwXfBojr06QZRDflPNizEI5pDe4jpR" +
			"f8FUkqRzFZa2AiEAuSeQdOw0M8cpmkvWUwfSn1JcPhiuUypfVdPtRxe0+RMxAA==")
	require.NoError(t, err)

	certs, err := parsePKCS7Certificates(bundle)
	require.NoError(t, err)
	require.Len(t, certs, 1)
	assert.Equal(t, "probe", certs[0].Subject.CommonName)
	require.NoError(t, certs[0].CheckSignatureFrom(certs[0]), "fixture is a self-signed certificate")

	assert.Equal(t, bundle, tlstest.BuildPKCS7Bundle(t, certs[0]),
		"tlstest.BuildPKCS7Bundle must be byte-identical to the real openssl bundle")
}

// Non-certificate CertificateChoices entries (e.g. v2 attribute certificates,
// context tag [2]) must be skipped without failing the whole bundle.
func TestParsePKCS7CertificateSet_SkipsAttributeCertEntries(t *testing.T) {
	t.Parallel()

	chain := tlstest.NewChain(t)
	attrCert := []byte{0xA2, 0x03, 0x30, 0x01, 0x00} // [2] { SEQUENCE { INTEGER 0 } }
	set := append(append([]byte{}, attrCert...), chain.IntermediateDER...)

	certs, err := parsePKCS7CertificateSet(set)
	require.NoError(t, err)
	require.Len(t, certs, 1)
	assert.Equal(t, chain.IntermediateDER, certs[0].Raw)
}

// testDER is a minimal DER tag-length-value builder for hand-crafting negative
// PKCS#7 structures in tests.
func testDER(tag byte, parts ...[]byte) []byte {
	var body []byte
	for _, part := range parts {
		body = append(body, part...)
	}
	l := len(body)
	var length []byte
	switch {
	case l < 0x80:
		length = []byte{byte(l)}
	case l < 0x100:
		length = []byte{0x81, byte(l)}
	default:
		length = []byte{0x82, byte(l >> 8), byte(l)}
	}
	out := make([]byte, 0, 1+len(length)+l)
	out = append(out, tag)
	out = append(out, length...)
	return append(out, body...)
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

	var aiaHits atomic.Int32
	handler := &tlstest.AIAHandler{}
	aiaSrv := startAIAServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		aiaHits.Add(1)
		handler.ServeHTTP(w, r)
	}))
	chain := buildChainWithAIA(t, aiaSrv.URL+"/ca.der")
	handler.SetCert(chain.IntermediateDER)

	tlsSrv := leafOnlyTLSServer(t, chain)

	fetcherA := NewAIAFetcher(nil)
	fetcherB := NewAIAFetcher(nil)

	// Fetcher A warms its cache.
	respA, errA := testHTTPClient(t, fetcherA.Wrap(&tls.Config{RootCAs: chain.RootPool}, nullLogger())).Get(leafOnlyTLSServerURL(tlsSrv)) //nolint:noctx
	require.NoError(t, errA)
	_ = respA.Body.Close()
	hitsAfterA := aiaHits.Load()
	require.Equal(t, int32(1), hitsAfterA, "fetcher A should have fetched once")

	// Fetcher B, independently, must hit the AIA server again — no shared cache.
	respB, errB := testHTTPClient(t, fetcherB.Wrap(&tls.Config{RootCAs: chain.RootPool}, nullLogger())).Get(leafOnlyTLSServerURL(tlsSrv)) //nolint:noctx
	require.NoError(t, errB)
	_ = respB.Body.Close()
	hitsAfterB := aiaHits.Load()
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

// IP-literal targets have empty ServerName (SNI drops IPs). Since we've disabled stdlib
// hostname verification to interpose AIA, the wrapper must fail closed.
func TestWrapTLSConfigForAIAFetching_IPLiteralTargetRejected(t *testing.T) {
	t.Parallel()

	h := &tlstest.AIAHandler{}
	aiaSrv := startAIAServer(t, h)
	chain := buildChainWithAIA(t, aiaSrv.URL+"/ca.der")
	h.SetCert(chain.IntermediateDER)

	tlsSrv := leafOnlyTLSServer(t, chain)

	// httptest URLs use 127.0.0.1; Go strips IPs from SNI so ServerName is "".
	wrappedCfg := NewAIAFetcher(nil).Wrap(
		&tls.Config{RootCAs: chain.RootPool},
		nullLogger(),
	)

	resp, err := testHTTPClient(t, wrappedCfg).Get(tlsSrv.URL) //nolint:noctx
	closeBody(resp)
	require.Error(t, err, "IP-literal target must be rejected when tlsAIAFetch is enabled")
	assert.Contains(t, err.Error(), "IP-literal target")
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
