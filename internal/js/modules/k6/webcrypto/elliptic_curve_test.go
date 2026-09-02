package webcrypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestECDHDeriveBitsNonByteAlignedLength(t *testing.T) {
	t.Parallel()

	generator := &ECKeyGenParams{
		Algorithm:  Algorithm{Name: ECDH},
		NamedCurve: EllipticCurveKindP256,
	}

	aliceResult, err := generator.GenerateKey(true, []CryptoKeyUsage{DeriveBitsCryptoKeyUsage})
	require.NoError(t, err)
	aliceKeyPair, err := aliceResult.ResolveCryptoKeyPair()
	require.NoError(t, err)

	bobResult, err := generator.GenerateKey(true, []CryptoKeyUsage{DeriveBitsCryptoKeyUsage})
	require.NoError(t, err)
	bobKeyPair, err := bobResult.ResolveCryptoKeyPair()
	require.NoError(t, err)

	deriver := ECDHKeyDeriveParams{
		Algorithm: Algorithm{Name: ECDH},
		Public:    bobKeyPair.PublicKey,
	}

	fullBits, err := deriver.DeriveBits(aliceKeyPair.PrivateKey, 256)
	require.NoError(t, err)
	require.Len(t, fullBits, 32)

	partialBits, err := deriver.DeriveBits(aliceKeyPair.PrivateKey, 245)
	require.NoError(t, err)
	require.Len(t, partialBits, 31)

	assert.Equal(t, fullBits[:30], partialBits[:30])
	assert.Equal(t, fullBits[30]&0xf8, partialBits[30])
	assert.Zero(t, partialBits[30]&0x07)
}
