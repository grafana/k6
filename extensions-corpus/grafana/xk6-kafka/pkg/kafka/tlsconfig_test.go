package kafka

import (
	"crypto/tls"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildTLSConfig(t *testing.T) {
	t.Parallel()

	// Disabled / absent → no TLS.
	cfg, err := buildTLSConfig(nil)
	require.NoError(t, err)
	require.Nil(t, cfg)

	cfg, err = buildTLSConfig(&TLSConfig{EnableTLS: false})
	require.NoError(t, err)
	require.Nil(t, cfg)

	// Mutual TLS + min version + server CA.
	keyDER, certDER := genKeyCert(t)
	keyPEM, certPEM := pemStr("PRIVATE KEY", keyDER), pemStr("CERTIFICATE", certDER)
	cfg, err = buildTLSConfig(&TLSConfig{
		EnableTLS:     true,
		MinVersion:    tls13,
		ClientCertPem: certPEM,
		ClientKeyPem:  keyPEM,
		ServerCaPem:   certPEM,
	})
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, uint16(tls.VersionTLS13), cfg.MinVersion)
	require.Len(t, cfg.Certificates, 1)
	require.NotNil(t, cfg.RootCAs)

	// Unknown TLS version.
	_, err = buildTLSConfig(&TLSConfig{EnableTLS: true, MinVersion: "tlsv9.9"})
	require.Error(t, err)

	// Invalid server CA PEM.
	_, err = buildTLSConfig(&TLSConfig{EnableTLS: true, ServerCaPem: "not a pem"})
	require.Error(t, err)
}
