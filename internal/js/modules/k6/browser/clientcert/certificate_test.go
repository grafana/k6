package clientcert

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"embed"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/*.p12
var testdata embed.FS

const sentinelPassphrase = "k6-sentinel-passphrase"

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := testdata.ReadFile("testdata/" + name)
	require.NoError(t, err)
	return b
}

type keyKind int

const (
	rsaPKCS1 keyKind = iota
	rsaPKCS8
	ecdsaSEC1
	ed25519PKCS8
)

// generatePEM returns a self-signed certificate and its private key, both PEM encoded.
func generatePEM(t *testing.T, cn string, kind keyKind) (certPEM []byte, keyPEM []byte) {
	t.Helper()

	var (
		priv     crypto.Signer
		keyBlock *pem.Block
	)
	switch kind {
	case rsaPKCS1, rsaPKCS8:
		k, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		priv = k
		if kind == rsaPKCS1 {
			keyBlock = &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)}
		} else {
			der, err := x509.MarshalPKCS8PrivateKey(k)
			require.NoError(t, err)
			keyBlock = &pem.Block{Type: "PRIVATE KEY", Bytes: der}
		}
	case ecdsaSEC1:
		k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)
		priv = k
		der, err := x509.MarshalECPrivateKey(k)
		require.NoError(t, err)
		keyBlock = &pem.Block{Type: "EC PRIVATE KEY", Bytes: der}
	case ed25519PKCS8:
		_, k, err := ed25519.GenerateKey(rand.Reader)
		require.NoError(t, err)
		priv = k
		der, err := x509.MarshalPKCS8PrivateKey(k)
		require.NoError(t, err)
		keyBlock = &pem.Block{Type: "PRIVATE KEY", Bytes: der}
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, priv.Public(), priv)
	require.NoError(t, err)

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(keyBlock)
}

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

	type input struct {
		cert, key, pfx []byte
		passphrase     string
	}
	pemInput := func(cn string, kind keyKind) func(t *testing.T) input {
		return func(t *testing.T) input {
			c, k := generatePEM(t, cn, kind)
			return input{cert: c, key: k}
		}
	}
	pfxInput := func(name string) func(t *testing.T) input {
		return func(t *testing.T) input {
			return input{pfx: readFixture(t, name), passphrase: "k6-test"}
		}
	}

	tests := []struct {
		name      string
		input     func(t *testing.T) input
		cn        string
		algorithm x509.PublicKeyAlgorithm
		keyType   any
		chainLen  int
	}{
		{
			name: "pem_rsa_pkcs1", input: pemInput("k6-rsa-pkcs1", rsaPKCS1),
			cn: "k6-rsa-pkcs1", algorithm: x509.RSA, keyType: &rsa.PrivateKey{}, chainLen: 1,
		},
		{
			name: "pem_rsa_pkcs8", input: pemInput("k6-rsa-pkcs8", rsaPKCS8),
			cn: "k6-rsa-pkcs8", algorithm: x509.RSA, keyType: &rsa.PrivateKey{}, chainLen: 1,
		},
		{
			name: "pem_ecdsa_sec1", input: pemInput("k6-ecdsa-sec1", ecdsaSEC1),
			cn: "k6-ecdsa-sec1", algorithm: x509.ECDSA, keyType: &ecdsa.PrivateKey{}, chainLen: 1,
		},
		{
			name: "pem_ed25519", input: pemInput("k6-ed25519", ed25519PKCS8),
			cn: "k6-ed25519", algorithm: x509.Ed25519, keyType: ed25519.PrivateKey{}, chainLen: 1,
		},
		{
			name: "pem_legacy_encrypted",
			input: func(t *testing.T) input {
				c, k := generatePEM(t, "k6-legacy-encrypted", rsaPKCS1)
				return input{cert: c, key: legacyEncrypt(t, k, "k6-test"), passphrase: "k6-test"}
			},
			cn: "k6-legacy-encrypted", algorithm: x509.RSA, keyType: &rsa.PrivateKey{}, chainLen: 1,
		},
		{
			name: "pem_unencrypted_with_passphrase",
			input: func(t *testing.T) input {
				c, k := generatePEM(t, "k6-unencrypted", ecdsaSEC1)
				return input{cert: c, key: k, passphrase: "k6-test"}
			},
			cn: "k6-unencrypted", algorithm: x509.ECDSA, keyType: &ecdsa.PrivateKey{}, chainLen: 1,
		},
		{
			name: "pfx_legacy_3des_rsa", input: pfxInput("rsa-3des.p12"),
			cn: "k6-test-pfx-rsa", algorithm: x509.RSA, keyType: &rsa.PrivateKey{}, chainLen: 1,
		},
		{
			name: "pfx_legacy_3des_ecdsa", input: pfxInput("ecdsa-3des.p12"),
			cn: "k6-test-pfx-ecdsa", algorithm: x509.ECDSA, keyType: &ecdsa.PrivateKey{}, chainLen: 1,
		},
		{
			name: "pfx_with_chain", input: pfxInput("chain-3des.p12"),
			cn: "k6-test-pfx-chain", algorithm: x509.RSA, keyType: &rsa.PrivateKey{}, chainLen: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			in := tt.input(t)
			cert, err := LoadCertificate(in.cert, in.key, in.pfx, in.passphrase)
			require.NoError(t, err)
			require.NotNil(t, cert.Leaf)

			assert.Equal(t, tt.cn, cert.Leaf.Subject.CommonName)
			assert.Equal(t, tt.algorithm, cert.Leaf.PublicKeyAlgorithm)
			assert.IsType(t, tt.keyType, cert.PrivateKey)
			require.Len(t, cert.Certificate, tt.chainLen)
			assert.Equal(t, cert.Leaf.Raw, cert.Certificate[0])
		})
	}
}

func TestLoadCertificate_Errors(t *testing.T) {
	t.Parallel()

	type input struct {
		cert, key, pfx []byte
		passphrase     string
	}

	tests := []struct {
		name    string
		input   func(t *testing.T) input
		wantErr string
	}{
		{
			name: "pem_encrypted_pkcs8",
			input: func(t *testing.T) input {
				c, _ := generatePEM(t, "k6-pkcs8", rsaPKCS8)
				k := pem.EncodeToMemory(&pem.Block{Type: "ENCRYPTED PRIVATE KEY", Bytes: []byte{0x30, 0x00}})
				return input{cert: c, key: k, passphrase: sentinelPassphrase}
			},
			wantErr: "unsupported encrypted PKCS#8 key; convert to an unencrypted or legacy-encrypted PEM key",
		},
		{
			name: "pem_encrypted_without_passphrase",
			input: func(t *testing.T) input {
				c, k := generatePEM(t, "k6-encrypted", rsaPKCS1)
				return input{cert: c, key: legacyEncrypt(t, k, "k6-test")}
			},
			wantErr: "private key is encrypted; passphrase is required",
		},
		{
			name: "pem_wrong_passphrase",
			input: func(t *testing.T) input {
				c, k := generatePEM(t, "k6-encrypted", rsaPKCS1)
				return input{cert: c, key: legacyEncrypt(t, k, "k6-test"), passphrase: sentinelPassphrase}
			},
			wantErr: "decrypting private key",
		},
		{
			name: "pem_cert_key_mismatch",
			input: func(t *testing.T) input {
				c, _ := generatePEM(t, "k6-mismatch", rsaPKCS1)
				_, k := generatePEM(t, "k6-other", rsaPKCS1)
				return input{cert: c, key: k}
			},
			wantErr: "private key does not match public key",
		},
		{
			name: "pem_not_pem",
			input: func(t *testing.T) input {
				_, k := generatePEM(t, "k6-not-pem", rsaPKCS1)
				return input{cert: []byte("not a certificate"), key: k}
			},
			wantErr: "failed to find any PEM data in certificate input",
		},
		{
			name: "pfx_aes",
			input: func(t *testing.T) input {
				return input{pfx: readFixture(t, "rsa-aes.p12"), passphrase: sentinelPassphrase}
			},
			wantErr: "unsupported PFX encryption; re-export with 'openssl pkcs12 -legacy' or use PEM",
		},
		{
			name: "pfx_wrong_passphrase",
			input: func(t *testing.T) input {
				return input{pfx: readFixture(t, "rsa-3des.p12"), passphrase: sentinelPassphrase}
			},
			wantErr: "incorrect PFX passphrase",
		},
		{
			name: "pfx_garbage",
			input: func(_ *testing.T) input {
				return input{pfx: []byte("not a pfx")}
			},
			wantErr: "decoding PFX",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			in := tt.input(t)
			_, err := LoadCertificate(in.cert, in.key, in.pfx, in.passphrase)
			require.Error(t, err)
			assert.ErrorContains(t, err, tt.wantErr)
			if in.passphrase != "" {
				assert.NotContains(t, err.Error(), sentinelPassphrase)
			}
		})
	}
}
