package tlstest

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// KeyType selects the client certificate's key algorithm.
type KeyType int

// Client certificate key algorithms.
const (
	KeyRSA     KeyType = iota // RSA 2048
	KeyECDSA                  // ECDSA P-256
	KeyEd25519                // Ed25519
)

// ClientCertificate issues a client-auth leaf (ExtKeyUsageClientAuth) for cn, signed by the
// intermediate. The certificate holds the leaf followed by the intermediate.
func (c *Chain) ClientCertificate(t testing.TB, cn string, kt KeyType) tls.Certificate {
	t.Helper()

	cert, err := tls.X509KeyPair(c.ClientPEM(t, cn, kt))
	require.NoError(t, err)
	return cert
}

// ClientPEM is ClientCertificate as PEM: the leaf followed by the intermediate, and the PKCS#8 key.
func (c *Chain) ClientPEM(t testing.TB, cn string, kt KeyType) (certPEM, keyPEM []byte) {
	t.Helper()

	var key crypto.Signer
	var err error
	switch kt {
	case KeyRSA:
		key, err = rsa.GenerateKey(rand.Reader, 2048)
	case KeyECDSA:
		key, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	case KeyEd25519:
		_, key, err = ed25519.GenerateKey(rand.Reader)
	default:
		require.FailNow(t, "unknown key type", "%d", kt)
	}
	require.NoError(t, err)

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, tmpl, c.IntermediateCert, key.Public(), c.IntermediateKey)
	require.NoError(t, err)
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER})
	certPEM = append(certPEM, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.IntermediateDER})...)
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	return certPEM, keyPEM
}
