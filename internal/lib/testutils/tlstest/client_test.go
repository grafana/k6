package tlstest

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// getWithClientCert requests a server that requires a client certificate verifying against
// chain's root, using cert, and returns the client certificate CN the server saw.
func getWithClientCert(t *testing.T, chain *Chain, cert tls.Certificate) (string, error) {
	t.Helper()

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.TLS.PeerCertificates[0].Subject.CommonName)
	}))
	srv.TLS = &tls.Config{
		Certificates: []tls.Certificate{chain.FullChainTLSCertificate(t)},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    chain.RootPool,
		MinVersion:   tls.VersionTLS12,
	}
	srv.StartTLS()
	defer srv.Close()

	client := srv.Client()
	client.Transport.(*http.Transport).TLSClientConfig.Certificates = []tls.Certificate{cert}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL, nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	return string(body), err
}

func TestChainClientCertificate(t *testing.T) {
	t.Parallel()

	chain := NewChain(t)
	tests := []struct {
		name    string
		keyType KeyType
		wantAlg x509.PublicKeyAlgorithm
	}{
		{name: "rsa", keyType: KeyRSA, wantAlg: x509.RSA},
		{name: "ecdsa", keyType: KeyECDSA, wantAlg: x509.ECDSA},
		{name: "ed25519", keyType: KeyEd25519, wantAlg: x509.Ed25519},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fromPEM, err := tls.X509KeyPair(chain.ClientPEM(t, "pem", tt.keyType))
			require.NoError(t, err)
			certs := map[string]tls.Certificate{"pem": fromPEM, "cert": chain.ClientCertificate(t, "cert", tt.keyType)}
			for cn, cert := range certs {
				assert.Equal(t, tt.wantAlg, cert.Leaf.PublicKeyAlgorithm, cn)
				got, err := getWithClientCert(t, chain, cert)
				require.NoError(t, err, cn)
				assert.Equal(t, cn, got)
			}
		})
	}
}

func TestChainClientCertificate_OtherChain(t *testing.T) {
	t.Parallel()

	_, err := getWithClientCert(t, NewChain(t), NewChain(t).ClientCertificate(t, "other", KeyECDSA))
	assert.ErrorContains(t, err, "unknown certificate authority")
}
