package webcrypto

import (
	"crypto/ecdh"
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestECDHDeriveBitsRejectsNonPositiveLength(t *testing.T) {
	t.Parallel()

	curve := ecdh.P256()
	priv, err := curve.GenerateKey(rand.Reader)
	require.NoError(t, err)
	pub := priv.PublicKey()

	alg := EcKeyAlgorithm{
		KeyAlgorithm: KeyAlgorithm{Algorithm: Algorithm{Name: ECDH}},
		NamedCurve:   EllipticCurveKindP256,
	}
	privateKey := &CryptoKey{
		Type:      PrivateCryptoKeyType,
		Algorithm: alg,
		Usages:    []CryptoKeyUsage{DeriveBitsCryptoKeyUsage},
		handle:    priv,
	}
	publicKey := &CryptoKey{
		Type:      PublicCryptoKeyType,
		Algorithm: alg,
		handle:    pub,
	}
	params := ECDHKeyDeriveParams{
		Algorithm: Algorithm{Name: ECDH},
		Public:    publicKey,
	}

	tests := []struct {
		name   string
		length int
	}{
		{name: "zero", length: 0},
		// Previously panicked: len(shared) < -1 is false, then b[:-1] panics.
		{name: "negative_multiple_of_8", length: -8},
		{name: "negative", length: -1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result, err := params.DeriveBits(privateKey, tc.length)
			require.Error(t, err)
			assert.Nil(t, result)
			var webErr *Error
			require.ErrorAs(t, err, &webErr)
			assert.Equal(t, OperationError, webErr.Name)
			assert.Contains(t, webErr.Message, "length must be a positive number")
		})
	}
}

func TestECDHDeriveBitsAcceptsPositiveMultipleOf8(t *testing.T) {
	t.Parallel()

	curve := ecdh.P256()
	priv, err := curve.GenerateKey(rand.Reader)
	require.NoError(t, err)
	pub := priv.PublicKey()

	alg := EcKeyAlgorithm{
		KeyAlgorithm: KeyAlgorithm{Algorithm: Algorithm{Name: ECDH}},
		NamedCurve:   EllipticCurveKindP256,
	}
	privateKey := &CryptoKey{
		Type:      PrivateCryptoKeyType,
		Algorithm: alg,
		Usages:    []CryptoKeyUsage{DeriveBitsCryptoKeyUsage},
		handle:    priv,
	}
	publicKey := &CryptoKey{
		Type:      PublicCryptoKeyType,
		Algorithm: alg,
		handle:    pub,
	}
	params := ECDHKeyDeriveParams{
		Algorithm: Algorithm{Name: ECDH},
		Public:    publicKey,
	}

	result, err := params.DeriveBits(privateKey, 128)
	require.NoError(t, err)
	assert.Len(t, result, 16)
}
