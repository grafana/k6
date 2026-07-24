package js

// Integration tests for AIA (Authority Information Access) certificate chain fetching.
// These tests exercise the full k6 stack: runner → VU → HTTP transport → TLS handshake.

import (
	"crypto/rand"
	"crypto/rsa"
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

	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/metrics"
)

// rsaIntermediateCA generates an intermediate CA certificate signed by parent.
// Unlike generateTLSCertificateWithCA, this sets IsCA=true so the resulting cert
// can sign other certificates (a plain leaf cert cannot act as an issuer).
func rsaIntermediateCA(t *testing.T, parent *x509.Certificate, parentKey *rsa.PrivateKey) ([]byte, *x509.Certificate, *rsa.PrivateKey) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	sn, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber:          sn,
		Subject:               pkix.Name{CommonName: "Test Intermediate CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		SignatureAlgorithm:    x509.SHA256WithRSA,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, parent, &key.PublicKey, parentKey)
	require.NoError(t, err)

	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	return certPEM, cert, key
}

// rsaLeafWithAIA generates a leaf certificate signed by parent that advertises aiaURL
// in its Authority Information Access extension as the location of the issuer cert.
func rsaLeafWithAIA(t *testing.T, host string, parent *x509.Certificate, parentKey *rsa.PrivateKey, aiaURL string) ([]byte, []byte) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	sn, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber:          sn,
		Subject:               pkix.Name{CommonName: host},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		SignatureAlgorithm:    x509.SHA256WithRSA,
		IssuingCertificateURL: []string{aiaURL},
	}
	if ip := net.ParseIP(host); ip != nil {
		tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
	} else {
		tmpl.DNSNames = append(tmpl.DNSNames, host)
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, parent, &key.PublicKey, parentKey)
	require.NoError(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyBytes, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes})
	return certPEM, keyPEM
}

// TestVUIntegrationAIAFetching is a full end-to-end integration test:
// it runs k6 VUs against a TLS server that sends only a leaf certificate and
// requires AIA fetching to build the full chain to a trusted root.
//
// Two sub-tests verify the toggle behaviour:
//   - AIAEnabled:  tlsAIAFetch: true  → connection succeeds
//   - AIADisabled: default (off)      → connection fails with x509 error
func TestVUIntegrationAIAFetching(t *testing.T) {
	t.Parallel()

	// ── 1. Build a three-level certificate chain ────────────────────────────

	// Root CA (self-signed, trusted by the test client)
	rootCertPEM, rootKeyPEM := generateTLSCertificate(t, "127.0.0.1", time.Now(), time.Hour)

	rootBlock, _ := pem.Decode(rootCertPEM)
	rootCert, err := x509.ParseCertificate(rootBlock.Bytes)
	require.NoError(t, err)

	rootKeyBlock, _ := pem.Decode(rootKeyPEM)
	rootKeyAny, err := x509.ParsePKCS8PrivateKey(rootKeyBlock.Bytes)
	require.NoError(t, err)
	rootKey, ok := rootKeyAny.(*rsa.PrivateKey)
	require.True(t, ok)

	// ── 2. Start AIA server before building certs (we need its URL for the leaf) ──

	var (
		aiaMu          sync.RWMutex
		intermediarDER []byte
	)
	aiaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		aiaMu.RLock()
		defer aiaMu.RUnlock()
		w.Header().Set("Content-Type", "application/pkix-cert")
		_, _ = w.Write(intermediarDER)
	}))
	t.Cleanup(aiaSrv.Close)
	aiaURL := aiaSrv.URL + "/intermediate.der"

	// ── 3. Build intermediate CA and leaf with AIA URL ────────────────────────

	_, interCert, interKey := rsaIntermediateCA(t, rootCert, rootKey)

	aiaMu.Lock()
	intermediarDER = interCert.Raw // set before any request arrives
	aiaMu.Unlock()

	leafCertPEM, leafKeyPEM := rsaLeafWithAIA(t, "127.0.0.1", interCert, interKey, aiaURL)

	// ── 4. Start TLS server that sends only the leaf (no intermediate chain) ──

	tlsSrv := getTestServerWithCertificate(t, leafCertPEM, leafKeyPEM)
	go func() { _ = tlsSrv.Config.Serve(tlsSrv.Listener) }()
	t.Cleanup(func() { require.NoError(t, tlsSrv.Config.Close()) })

	_, port, err := net.SplitHostPort(tlsSrv.Listener.Addr().String())
	require.NoError(t, err)
	serverAddr := "127.0.0.1:" + port

	rootPool := x509.NewCertPool()
	rootPool.AddCert(rootCert)

	// ── 5. Table-driven sub-tests ─────────────────────────────────────────────

	testdata := map[string]struct {
		aiaFetch null.Bool
		errMsg   string
	}{
		// AIA fetching enabled: should build chain via AIA and succeed.
		"AIAEnabled": {null.BoolFrom(true), ""},
		// AIA fetching disabled (default): incomplete chain, must fail.
		"AIADisabled": {null.Bool{}, "certificate signed by unknown authority"},
	}

	for name, td := range testdata {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			script := fmt.Sprintf(`
				var http = require("k6/http");
				exports.default = function() { http.get("https://%s/"); }
			`, serverAddr)

			r1, err := getSimpleRunner(t, "/script.js", script)
			require.NoError(t, err)

			opts := lib.Options{
				Throw:       null.BoolFrom(true),
				TLSAIAFetch: td.aiaFetch,
			}
			require.NoError(t, r1.SetOptions(opts))

			r2, err := getSimpleArchiveRunner(t, r1.MakeArchive())
			require.NoError(t, err)

			runners := map[string]*Runner{"Source": r1, "Archive": r2}
			for rname, r := range runners {
				t.Run(rname, func(t *testing.T) {
					t.Parallel()
					r.preInitState.Logger, _ = logtest.NewNullLogger()

					ctx := t.Context()
					initVU, err := r.NewVU(ctx, 1, 1, make(chan metrics.SampleContainer, 100))
					require.NoError(t, err)

					// Give the VU only the root CA – no intermediate.
					// With AIA enabled, it should fetch the intermediate automatically.
					initVU.(*VU).TLSConfig.RootCAs = rootPool

					vu := initVU.Activate(&lib.VUActivationParams{RunContext: ctx})
					runErr := vu.RunOnce()

					if td.errMsg != "" {
						require.Error(t, runErr)
						assert.Contains(t, runErr.Error(), td.errMsg)
					} else {
						require.NoError(t, runErr)
					}
				})
			}
		})
	}
}
