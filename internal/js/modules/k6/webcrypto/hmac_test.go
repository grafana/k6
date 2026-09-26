package webcrypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"
)

func TestHMACKeyGenParamsGenerateKeyRejectsInvalidLength(t *testing.T) {
	t.Parallel()

	validUsages := []CryptoKeyUsage{SignCryptoKeyUsage, VerifyCryptoKeyUsage}

	tests := []struct {
		name    string
		length  int64
		wantErr string
	}{
		{
			name:    "zero",
			length:  0,
			wantErr: "algorithm's length must be a positive number",
		},
		{
			name:    "negative_multiple_of_8",
			length:  -8,
			wantErr: "algorithm's length must be a positive number",
		},
		{
			name:    "negative_non_multiple",
			length:  -1,
			wantErr: "algorithm's length must be a positive number",
		},
		{
			name:    "positive_not_multiple_of_8",
			length:  7,
			wantErr: "algorithm's length must be a multiple of 8",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			hkgp := &HMACKeyGenParams{
				Algorithm: Algorithm{Name: HMAC},
				Hash:      Algorithm{Name: SHA256},
				Length:    null.IntFrom(tc.length),
			}

			// Previously length:-8 panicked in make([]byte, asByteLength()).
			result, err := hkgp.GenerateKey(true, validUsages)
			require.Error(t, err)
			assert.Nil(t, result)
			var webErr *Error
			require.ErrorAs(t, err, &webErr)
			assert.Equal(t, OperationError, webErr.Name)
			assert.Contains(t, webErr.Message, tc.wantErr)
		})
	}
}

func TestHMACKeyGenParamsGenerateKeyAcceptsPositiveMultipleOf8(t *testing.T) {
	t.Parallel()

	hkgp := &HMACKeyGenParams{
		Algorithm: Algorithm{Name: HMAC},
		Hash:      Algorithm{Name: SHA256},
		Length:    null.IntFrom(256),
	}

	result, err := hkgp.GenerateKey(true, []CryptoKeyUsage{SignCryptoKeyUsage})
	require.NoError(t, err)
	require.NotNil(t, result)

	key, ok := result.(*CryptoKey)
	require.True(t, ok)
	handle, ok := key.handle.([]byte)
	require.True(t, ok)
	assert.Len(t, handle, 32)
}
