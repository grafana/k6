package webcrypto

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRSAOaepGenerateKeyUsages(t *testing.T) {
	params := &RSAHashedKeyGenParams{
		Algorithm:      Algorithm{Name: RSAOaep},
		ModulusLength:  2048,
		PublicExponent: []byte{1, 0, 1},
		Hash:           Algorithm{Name: SHA256},
	}

	result, err := params.GenerateKey(true, []CryptoKeyUsage{
		EncryptCryptoKeyUsage,
		DecryptCryptoKeyUsage,
		WrapKeyCryptoKeyUsage,
		UnwrapKeyCryptoKeyUsage,
	})
	require.NoError(t, err)

	pair, err := result.ResolveCryptoKeyPair()
	require.NoError(t, err)
	require.Equal(t, []CryptoKeyUsage{DecryptCryptoKeyUsage, UnwrapKeyCryptoKeyUsage}, pair.PrivateKey.Usages)
	require.Equal(t, []CryptoKeyUsage{EncryptCryptoKeyUsage, WrapKeyCryptoKeyUsage}, pair.PublicKey.Usages)
}
