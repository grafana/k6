package webcrypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRSAOAEPGenerateKeyWrappingUsages(t *testing.T) {
	t.Parallel()

	params := &RSAHashedKeyGenParams{
		Algorithm: Algorithm{
			Name: RSAOaep,
		},
		ModulusLength:  2048,
		PublicExponent: []byte{1, 0, 1},
		Hash: Algorithm{
			Name: "SHA-256",
		},
	}

	testCases := []struct {
		name            string
		usages          []CryptoKeyUsage
		expectedPrivate []CryptoKeyUsage
		expectedPublic  []CryptoKeyUsage
	}{
		{
			name:            "all four usages",
			usages:          []CryptoKeyUsage{EncryptCryptoKeyUsage, DecryptCryptoKeyUsage, WrapKeyCryptoKeyUsage, UnwrapKeyCryptoKeyUsage},
			expectedPrivate: []CryptoKeyUsage{DecryptCryptoKeyUsage, UnwrapKeyCryptoKeyUsage},
			expectedPublic:  []CryptoKeyUsage{EncryptCryptoKeyUsage, WrapKeyCryptoKeyUsage},
		},
		{
			name:            "wrap and unwrap only",
			usages:          []CryptoKeyUsage{WrapKeyCryptoKeyUsage, UnwrapKeyCryptoKeyUsage},
			expectedPrivate: []CryptoKeyUsage{UnwrapKeyCryptoKeyUsage},
			expectedPublic:  []CryptoKeyUsage{WrapKeyCryptoKeyUsage},
		},
		{
			name:            "encrypt and decrypt only",
			usages:          []CryptoKeyUsage{EncryptCryptoKeyUsage, DecryptCryptoKeyUsage},
			expectedPrivate: []CryptoKeyUsage{DecryptCryptoKeyUsage},
			expectedPublic:  []CryptoKeyUsage{EncryptCryptoKeyUsage},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			res, err := params.GenerateKey(true, tc.usages)
			require.NoError(t, err)

			keyPair, ok := res.(*CryptoKeyPair)
			require.True(t, ok)

			assert.ElementsMatch(t, tc.expectedPrivate, keyPair.PrivateKey.Usages)
			assert.ElementsMatch(t, tc.expectedPublic, keyPair.PublicKey.Usages)
		})
	}
}
