package http

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/v2/lib"
)

// TestPerRequestTLSAuth verifies that a client certificate can be selected
// per HTTP request via params.tlsAuth, independently of global options.tlsAuth.
func TestPerRequestTLSAuth(t *testing.T) {
	t.Parallel()

	caCertPEM, caKeyPEM := mustGenerateCert(t, "ca", true, nil, nil)
	caCert, caKey := mustParseCertKey(t, caCertPEM, caKeyPEM)

	srvCertPEM, srvKeyPEM := mustGenerateCert(t, "127.0.0.1", false, caCert, caKey)
	clientACertPEM, clientAKeyPEM := mustGenerateCert(t, "client-a", false, caCert, caKey)
	clientBCertPEM, clientBKeyPEM := mustGenerateCert(t, "client-b", false, caCert, caKey)

	clientCAPool := x509.NewCertPool()
	require.True(t, clientCAPool.AppendCertsFromPEM(caCertPEM))

	serverCert, err := tls.X509KeyPair(append(srvCertPEM, caCertPEM...), srvKeyPEM)
	require.NoError(t, err)

	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    clientCAPool,
		MinVersion:   tls.VersionTLS12,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	seenSubjects := make(chan string, 8)
	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
				http.Error(w, "missing client cert", http.StatusUnauthorized)
				return
			}
			seenSubjects <- r.TLS.PeerCertificates[0].Subject.CommonName
			_, _ = fmt.Fprint(w, "ok")
		}),
		ReadHeaderTimeout: time.Second,
	}
	go func() { _ = srv.Serve(listener) }()
	t.Cleanup(func() { _ = srv.Close() })

	url := "https://" + listener.Addr().String() + "/"
	ts := newTestCase(t)
	state := ts.runtime.VU.State()
	state.Options.Throw = null.BoolFrom(true)
	state.Options.InsecureSkipTLSVerify = null.BoolFrom(true)
	// Start with no global client certificates so only per-request tlsAuth can succeed.
	state.TLSConfig = &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec
		MinVersion:         tls.VersionTLS12,
	}
	if tr, ok := state.Transport.(*http.Transport); ok {
		tr.TLSClientConfig = state.TLSConfig
	}

	jsCertA := jsPEM(clientACertPEM)
	jsKeyA := jsPEM(clientAKeyPEM)
	jsCertB := jsPEM(clientBCertPEM)
	jsKeyB := jsPEM(clientBKeyPEM)

	t.Run("missing_tlsAuth_fails", func(t *testing.T) {
		_, err := ts.runtime.VU.Runtime().RunString(fmt.Sprintf(`
			http.get(%q);
		`, url))
		require.Error(t, err)
	})

	t.Run("per_request_tlsAuth_succeeds", func(t *testing.T) {
		_, err := ts.runtime.VU.Runtime().RunString(fmt.Sprintf(`
			var res = http.get(%q, {
				tlsAuth: { cert: "%s", key: "%s" }
			});
			if (res.status !== 200) {
				throw new Error("unexpected status: " + res.status + " error: " + res.error);
			}
			if (res.body !== "ok") {
				throw new Error("unexpected body: " + res.body);
			}
		`, url, jsCertA, jsKeyA))
		require.NoError(t, err)
		require.Equal(t, "client-a", <-seenSubjects)
	})

	t.Run("can_switch_certificates_between_requests", func(t *testing.T) {
		_, err := ts.runtime.VU.Runtime().RunString(fmt.Sprintf(`
			var resA = http.get(%q, { tlsAuth: { cert: "%s", key: "%s" } });
			if (resA.status !== 200) { throw new Error("A failed: " + resA.error); }
			var resB = http.get(%q, { tlsAuth: { cert: "%s", key: "%s" } });
			if (resB.status !== 200) { throw new Error("B failed: " + resB.error); }
		`, url, jsCertA, jsKeyA, url, jsCertB, jsKeyB))
		require.NoError(t, err)
		require.Equal(t, "client-a", <-seenSubjects)
		require.Equal(t, "client-b", <-seenSubjects)
	})

	t.Run("per_request_overrides_global_tlsAuth", func(t *testing.T) {
		auth := &lib.TLSAuth{
			TLSAuthFields: lib.TLSAuthFields{
				Cert: string(clientACertPEM),
				Key:  string(clientAKeyPEM),
			},
		}
		cert, err := auth.Certificate()
		require.NoError(t, err)
		state.TLSConfig = &tls.Config{
			InsecureSkipVerify: true, //nolint:gosec
			Certificates:       []tls.Certificate{*cert},
			MinVersion:         tls.VersionTLS12,
		}
		if tr, ok := state.Transport.(*http.Transport); ok {
			tr.TLSClientConfig = state.TLSConfig
		}

		_, err = ts.runtime.VU.Runtime().RunString(fmt.Sprintf(`
			var res = http.get(%q, {
				tlsAuth: { cert: "%s", key: "%s" }
			});
			if (res.status !== 200) {
				throw new Error("override failed: " + res.error);
			}
		`, url, jsCertB, jsKeyB))
		require.NoError(t, err)
		require.Equal(t, "client-b", <-seenSubjects)
	})
}

func jsPEM(pemBytes []byte) string {
	return strings.ReplaceAll(string(pemBytes), "\n", `\n`)
}

func mustGenerateCert(
	t *testing.T, name string, isCA bool, parent *x509.Certificate, parentKey *rsa.PrivateKey,
) ([]byte, []byte) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: name, Organization: []string{"k6-test"}},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
	}
	if ip := net.ParseIP(name); ip != nil {
		tmpl.IPAddresses = []net.IP{ip}
	} else {
		tmpl.DNSNames = []string{name}
	}
	if isCA {
		tmpl.IsCA = true
		tmpl.BasicConstraintsValid = true
		tmpl.KeyUsage |= x509.KeyUsageCertSign
		parent = tmpl
		parentKey = key
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, parent, &key.PublicKey, parentKey)
	require.NoError(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	return certPEM, keyPEM
}

func mustParseCertKey(t *testing.T, certPEM, keyPEM []byte) (*x509.Certificate, *rsa.PrivateKey) {
	t.Helper()
	block, _ := pem.Decode(certPEM)
	require.NotNil(t, block)
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	keyBlock, _ := pem.Decode(keyPEM)
	require.NotNil(t, keyBlock)
	keyAny, err := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	require.NoError(t, err)
	key, ok := keyAny.(*rsa.PrivateKey)
	require.True(t, ok)
	return cert, key
}
