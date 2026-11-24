package clientcert

import (
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/lib/testutils/tlstest"
)

const sentinelPassphrase = "k6-sentinel-passphrase"

// legacyEncrypt re-encodes a PEM key with RFC 1423 (Proc-Type: 4,ENCRYPTED) encryption.
func legacyEncrypt(t *testing.T, keyPEM []byte, passphrase string) []byte {
	t.Helper()
	b, _ := pem.Decode(keyPEM)
	require.NotNil(t, b)
	//nolint:staticcheck // legacy PEM encryption is the format under test
	enc, err := x509.EncryptPEMBlock(rand.Reader, b.Type, b.Bytes, []byte(passphrase), x509.PEMCipherAES256)
	require.NoError(t, err)
	return pem.EncodeToMemory(enc)
}

func TestLoadCertificate(t *testing.T) {
	t.Parallel()

	chain := tlstest.NewChain(t)
	tests := []struct {
		name       string
		keyType    tlstest.KeyType
		algorithm  x509.PublicKeyAlgorithm
		encrypt    bool
		passphrase string
	}{
		{name: "rsa", keyType: tlstest.KeyRSA, algorithm: x509.RSA},
		{name: "ecdsa", keyType: tlstest.KeyECDSA, algorithm: x509.ECDSA},
		{name: "ed25519", keyType: tlstest.KeyEd25519, algorithm: x509.Ed25519},
		{
			name: "legacy_encrypted", keyType: tlstest.KeyRSA, algorithm: x509.RSA,
			encrypt: true, passphrase: "k6-test",
		},
		{
			name: "unencrypted_with_passphrase", keyType: tlstest.KeyECDSA, algorithm: x509.ECDSA,
			passphrase: "k6-test",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			certPEM, keyPEM := chain.ClientPEM(t, "k6-"+tt.name, tt.keyType)
			if tt.encrypt {
				keyPEM = legacyEncrypt(t, keyPEM, tt.passphrase)
			}
			cert, err := LoadCertificate(certPEM, keyPEM, tt.passphrase)
			require.NoError(t, err)
			require.NotNil(t, cert.Leaf)
			assert.Equal(t, "k6-"+tt.name, cert.Leaf.Subject.CommonName)
			assert.Equal(t, tt.algorithm, cert.Leaf.PublicKeyAlgorithm)
			require.Len(t, cert.Certificate, 2)
			assert.Equal(t, cert.Leaf.Raw, cert.Certificate[0])
		})
	}
}

func TestLoadCertificate_Errors(t *testing.T) {
	t.Parallel()

	chain := tlstest.NewChain(t)
	certPEM, keyPEM := chain.ClientPEM(t, "k6-client", tlstest.KeyRSA)
	_, otherKeyPEM := chain.ClientPEM(t, "k6-other", tlstest.KeyRSA)
	encryptedKeyPEM := legacyEncrypt(t, keyPEM, "k6-test")
	pkcs8KeyPEM := pem.EncodeToMemory(&pem.Block{Type: "ENCRYPTED PRIVATE KEY", Bytes: []byte{0x30, 0x00}})

	tests := []struct {
		name       string
		key        []byte
		passphrase string
		wantErr    string
	}{
		{
			name: "encrypted_pkcs8", key: pkcs8KeyPEM, passphrase: sentinelPassphrase,
			wantErr: "unsupported encrypted PKCS#8 key; convert to an unencrypted or legacy-encrypted PEM key",
		},
		{
			name: "encrypted_without_passphrase", key: encryptedKeyPEM,
			wantErr: "private key is encrypted; passphrase is required",
		},
		{
			name: "wrong_passphrase", key: encryptedKeyPEM, passphrase: sentinelPassphrase,
			wantErr: "decrypting private key",
		},
		{
			name: "cert_key_mismatch", key: otherKeyPEM,
			wantErr: "private key does not match public key",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := LoadCertificate(certPEM, tt.key, tt.passphrase)
			assert.ErrorContains(t, err, tt.wantErr)
			if tt.passphrase != "" {
				assert.NotContains(t, err.Error(), sentinelPassphrase)
			}
		})
	}
}
