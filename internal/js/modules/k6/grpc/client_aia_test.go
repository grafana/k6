package grpc

import (
	"context"
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
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/lib/netext"
)

// Verifies AIA verification honours gRPC-supplied cacerts, not the VU config's RootCAs.
func TestBuildTLSConfig_AIAWithCustomCACerts(t *testing.T) {
	t.Parallel()

	chain := newGRPCTestChain(t)

	aiaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/pkix-cert")
		_, _ = w.Write(chain.intermediateDER)
	}))
	t.Cleanup(aiaSrv.Close)
	chain.setAIAURL(t, aiaSrv.URL)

	tlsListener := chain.newLeafOnlyTLSServer(t)
	t.Cleanup(func() { _ = tlsListener.Close() })

	// VU config has no RootCAs; the user relies on per-connect cacerts.
	vuCfg := &tls.Config{MinVersion: tls.VersionTLS12}
	wrappedVU := netext.WrapTLSConfigForAIAFetching(vuCfg, nullLogger(), nil)

	rootPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: chain.rootDER})
	tlsCfg, err := buildTLSConfig(wrappedVU, nil, nil, [][]byte{rootPEM}, true, nullLogger())
	require.NoError(t, err)

	tlsCfg.ServerName = "localhost"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	dialer := &tls.Dialer{Config: tlsCfg}
	conn, err := dialer.DialContext(ctx, "tcp", tlsListener.Addr().String())
	require.NoError(t, err)
	_ = conn.Close()
}

type grpcTestChain struct {
	rootDER         []byte
	rootCert        *x509.Certificate
	rootKey         *ecdsa.PrivateKey
	intermediateDER []byte
	intermediateKey *ecdsa.PrivateKey
	interCert       *x509.Certificate
	leafKey         *ecdsa.PrivateKey
	leafTmpl        *x509.Certificate
}

func newGRPCTestChain(t testing.TB) *grpcTestChain {
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
		Subject:               pkix.Name{CommonName: "gRPC Test Root"},
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
		Subject:               pkix.Name{CommonName: "gRPC Test Intermediate"},
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

	return &grpcTestChain{
		rootDER:         rootDER,
		rootCert:        rootCert,
		rootKey:         rootKey,
		intermediateDER: interDER,
		intermediateKey: interKey,
		interCert:       interCert,
		leafKey:         leafKey,
		leafTmpl:        leafTmpl,
	}
}

func (c *grpcTestChain) setAIAURL(t testing.TB, aiaURL string) {
	t.Helper()
	c.leafTmpl.IssuingCertificateURL = []string{aiaURL}
}

func (c *grpcTestChain) newLeafOnlyTLSServer(t testing.TB) net.Listener {
	t.Helper()
	leafDER, err := x509.CreateCertificate(rand.Reader, c.leafTmpl, c.interCert, &c.leafKey.PublicKey, c.intermediateKey)
	require.NoError(t, err)
	leafKeyDER, err := x509.MarshalECPrivateKey(c.leafKey)
	require.NoError(t, err)
	leafCert, err := tls.X509KeyPair(
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: leafKeyDER}),
	)
	require.NoError(t, err)

	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{leafCert},
		MinVersion:   tls.VersionTLS12,
	})
	require.NoError(t, err)

	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				if strings.Contains(acceptErr.Error(), "use of closed") {
					return
				}
				return
			}
			// Force the handshake to complete, then close.
			if tlsConn, ok := conn.(*tls.Conn); ok {
				hctx, hcancel := context.WithTimeout(context.Background(), 5*time.Second)
				_ = tlsConn.HandshakeContext(hctx)
				hcancel()
			}
			_ = conn.Close()
		}
	}()
	return listener
}

func nullLogger() logrus.FieldLogger {
	l := logrus.New()
	l.SetLevel(logrus.PanicLevel)
	return l
}
