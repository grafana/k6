package grpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/lib/testutils/tlstest"
	"go.k6.io/k6/v2/lib/netext"
)

// Verifies AIA verification honours gRPC-supplied cacerts, not the VU config's RootCAs.
func TestBuildTLSConfig_AIAWithCustomCACerts(t *testing.T) {
	t.Parallel()

	chain := tlstest.NewChain(t)

	aiaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/pkix-cert")
		_, _ = w.Write(chain.IntermediateDER)
	}))
	t.Cleanup(aiaSrv.Close)
	chain.SetAIAURL(aiaSrv.URL)

	tlsListener := newLeafOnlyTLSListener(t, chain)
	t.Cleanup(func() { _ = tlsListener.Close() })

	// VU config has no RootCAs; the user relies on per-connect cacerts.
	vuCfg := &tls.Config{MinVersion: tls.VersionTLS12}
	wrappedVU := netext.WrapTLSConfigForAIAFetching(vuCfg, nullLogger(), nil)

	rootPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: chain.RootDER})
	tlsCfg, err := buildTLSConfig(wrappedVU, nil, nil, [][]byte{rootPEM}, true, false, nullLogger())
	require.NoError(t, err)

	tlsCfg.ServerName = "localhost"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	dialer := &tls.Dialer{Config: tlsCfg}
	conn, err := dialer.DialContext(ctx, "tcp", tlsListener.Addr().String())
	require.NoError(t, err)
	_ = conn.Close()
}

// Verifies gRPC is not silently insecure when AIA is enabled: if the presented cert cannot
// be validated against the caller's cacerts, the dial must fail even though the wrapper
// disables Go's built-in InsecureSkipVerify handling internally.
func TestBuildTLSConfig_AIARejectsMismatchedCACert(t *testing.T) {
	t.Parallel()

	// Server chain (root A → inter A → leaf).
	chain := tlstest.NewChain(t)

	aiaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/pkix-cert")
		_, _ = w.Write(chain.IntermediateDER)
	}))
	t.Cleanup(aiaSrv.Close)
	chain.SetAIAURL(aiaSrv.URL)

	tlsListener := newLeafOnlyTLSListener(t, chain)
	t.Cleanup(func() { _ = tlsListener.Close() })

	// Build an unrelated root CA (root B). The caller supplies root B as `cacerts` and enables
	// AIA. The server presents a chain rooted in A, so verification must fail.
	otherChain := tlstest.NewChain(t)
	otherRootPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: otherChain.RootDER})

	vuCfg := &tls.Config{MinVersion: tls.VersionTLS12}
	wrappedVU := netext.WrapTLSConfigForAIAFetching(vuCfg, nullLogger(), nil)

	tlsCfg, err := buildTLSConfig(wrappedVU, nil, nil, [][]byte{otherRootPEM}, true, false, nullLogger())
	require.NoError(t, err)
	tlsCfg.ServerName = "localhost"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	dialer := &tls.Dialer{Config: tlsCfg}
	conn, err := dialer.DialContext(ctx, "tcp", tlsListener.Addr().String())
	if conn != nil {
		_ = conn.Close()
	}
	require.Error(t, err, "AIA-enabled gRPC dial must fail when cacerts do not cover the server chain")
	require.True(t,
		strings.Contains(err.Error(), "unknown authority") ||
			strings.Contains(err.Error(), "signed by"),
		"unexpected error: %v", err,
	)
}

// newLeafOnlyTLSListener starts a raw tls.Listener that serves the leaf certificate only.
// Used by gRPC path tests (which want a low-level TLS handshake, not an HTTP server).
func newLeafOnlyTLSListener(t testing.TB, c *tlstest.Chain) net.Listener {
	t.Helper()

	leafCert, err := tls.X509KeyPair(
		pemEncodeCert(c.LeafTLSCertificate(t).Certificate[0]),
		pemEncodeKey(t, c),
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

func pemEncodeCert(der []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func pemEncodeKey(t testing.TB, c *tlstest.Chain) []byte {
	t.Helper()
	keyDER, err := x509.MarshalECPrivateKey(c.LeafKey)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
}

func nullLogger() logrus.FieldLogger {
	l := logrus.New()
	l.SetLevel(logrus.PanicLevel)
	return l
}
