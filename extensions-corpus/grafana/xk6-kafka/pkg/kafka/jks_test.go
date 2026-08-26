package kafka

import (
	"bytes"
	"strings"
	"testing"
	"time"

	keystore "github.com/pavlo-v-chernykh/keystore-go/v4"
	"github.com/stretchr/testify/require"
)

func TestJKSToPEM(t *testing.T) {
	t.Parallel()

	keyDER, certDER := genKeyCert(t)
	cert := keystore.Certificate{Type: "X509", Content: certDER}

	ks := keystore.New()
	require.NoError(t, ks.SetPrivateKeyEntry("client", keystore.PrivateKeyEntry{
		CreationTime:     time.Now(),
		PrivateKey:       keyDER,
		CertificateChain: []keystore.Certificate{cert},
	}, []byte("keypass")))
	require.NoError(t, ks.SetTrustedCertificateEntry("ca", keystore.TrustedCertificateEntry{
		CreationTime: time.Now(),
		Certificate:  cert,
	}))

	var buf bytes.Buffer
	require.NoError(t, ks.Store(&buf, []byte("storepass")))

	jks, err := jksToPEM(buf.Bytes(), JKSConfig{
		Password:          "storepass",
		ClientKeyAlias:    "client",
		ClientKeyPassword: "keypass",
		ServerCaAlias:     "ca",
	})
	require.NoError(t, err)
	require.Len(t, jks.ClientCertsPem, 1)
	require.Contains(t, jks.ClientCertsPem[0], "BEGIN CERTIFICATE")
	require.Contains(t, jks.ClientKeyPem, "BEGIN PRIVATE KEY")
	require.Contains(t, jks.ServerCaPem, "BEGIN CERTIFICATE")
}

func TestJKSToPEMTruststoreOnly(t *testing.T) {
	t.Parallel()

	_, certDER := genKeyCert(t)
	ks := keystore.New()
	require.NoError(t, ks.SetTrustedCertificateEntry("ca", keystore.TrustedCertificateEntry{
		CreationTime: time.Now(),
		Certificate:  keystore.Certificate{Type: "X509", Content: certDER},
	}))

	var buf bytes.Buffer
	require.NoError(t, ks.Store(&buf, []byte("storepass")))

	// No client key alias: only the server CA is extracted.
	jks, err := jksToPEM(buf.Bytes(), JKSConfig{Password: "storepass", ServerCaAlias: "ca"})
	require.NoError(t, err)
	require.Empty(t, jks.ClientCertsPem)
	require.Empty(t, jks.ClientKeyPem)
	require.Contains(t, jks.ServerCaPem, "BEGIN CERTIFICATE")
}

func TestJKSToPEMRejectsNonJKS(t *testing.T) {
	t.Parallel()

	_, err := jksToPEM([]byte(strings.Repeat("not a keystore", 8)), JKSConfig{Password: "x"})
	require.Error(t, err)
}
