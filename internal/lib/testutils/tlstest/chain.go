// Package tlstest provides shared helpers for building root CA → intermediate → leaf
// chains used by AIA fetching tests. Test-only, do not import in production code.
package tlstest

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Chain holds root CA → intermediate → leaf material. The leaf DER is generated on demand
// (LeafTLSCertificate) so callers can bake in an AIA URL first via SetAIAURL.
type Chain struct {
	RootCert         *x509.Certificate
	RootDER          []byte
	RootKey          *ecdsa.PrivateKey
	IntermediateCert *x509.Certificate
	IntermediateDER  []byte
	IntermediateKey  *ecdsa.PrivateKey
	LeafKey          *ecdsa.PrivateKey
	LeafTmpl         *x509.Certificate
	RootPool         *x509.CertPool
}

// NewChain builds a fresh root, intermediate and leaf template. Certificates use short
// (24h) lifetimes and P-256 keys.
func NewChain(t testing.TB) *Chain {
	t.Helper()

	genKey := func() *ecdsa.PrivateKey {
		k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)
		return k
	}
	serial := func() *big.Int {
		n, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
		require.NoError(t, err)
		return n
	}

	rootKey := genKey()
	rootTmpl := &x509.Certificate{
		SerialNumber:          serial(),
		Subject:               pkix.Name{CommonName: "tlstest Root CA"},
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

	interKey := genKey()
	interTmpl := &x509.Certificate{
		SerialNumber:          serial(),
		Subject:               pkix.Name{CommonName: "tlstest Intermediate CA"},
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

	leafKey := genKey()
	leafTmpl := &x509.Certificate{
		SerialNumber: serial(),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}

	rootPool := x509.NewCertPool()
	rootPool.AddCert(rootCert)

	return &Chain{
		RootCert:         rootCert,
		RootDER:          rootDER,
		RootKey:          rootKey,
		IntermediateCert: interCert,
		IntermediateDER:  interDER,
		IntermediateKey:  interKey,
		LeafKey:          leafKey,
		LeafTmpl:         leafTmpl,
		RootPool:         rootPool,
	}
}

// SetAIAURL sets the leaf's IssuingCertificateURL. Call before LeafTLSCertificate.
func (c *Chain) SetAIAURL(aiaURL string) {
	c.LeafTmpl.IssuingCertificateURL = []string{aiaURL}
}

// LeafTLSCertificate returns a tls.Certificate containing only the leaf (no intermediate).
func (c *Chain) LeafTLSCertificate(t testing.TB) tls.Certificate {
	t.Helper()
	leafDER, err := x509.CreateCertificate(
		rand.Reader, c.LeafTmpl, c.IntermediateCert, &c.LeafKey.PublicKey, c.IntermediateKey,
	)
	require.NoError(t, err)
	leafKeyDER, err := x509.MarshalECPrivateKey(c.LeafKey)
	require.NoError(t, err)
	cert, err := tls.X509KeyPair(
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: leafKeyDER}),
	)
	require.NoError(t, err)
	return cert
}

// FullChainTLSCertificate returns leaf + intermediate DER.
func (c *Chain) FullChainTLSCertificate(t testing.TB) tls.Certificate {
	t.Helper()
	cert := c.LeafTLSCertificate(t)
	cert.Certificate = append(cert.Certificate, c.IntermediateCert.Raw)
	return cert
}

// AIAHandler serves the DER bytes set via SetCert with an application/pkix-cert Content-Type.
type AIAHandler struct {
	mu      sync.RWMutex
	certDER []byte
}

// SetCert atomically updates the DER bytes served by subsequent requests.
func (h *AIAHandler) SetCert(der []byte) {
	h.mu.Lock()
	h.certDER = der
	h.mu.Unlock()
}

// ServeHTTP writes the currently-set DER bytes as an AIA response.
func (h *AIAHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	w.Header().Set("Content-Type", "application/pkix-cert")
	_, _ = w.Write(h.certDER)
}
