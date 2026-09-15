package webcrypto

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAESCBCDecryptRejectsInvalidCiphertextLength(t *testing.T) {
	t.Parallel()

	key := CryptoKey{handle: make([]byte, 16)}
	params := &AESCBCParams{Iv: make([]byte, 16)}

	for _, tt := range []struct {
		name string
		ct   []byte
	}{
		{name: "empty", ct: []byte{}},
		{name: "len15", ct: make([]byte, 15)},
		{name: "len17", ct: make([]byte, 17)},
		{name: "len31", ct: make([]byte, 31)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.NotPanics(t, func() {
				_, err := params.Decrypt(tt.ct, key)
				assert.Error(t, err)

				var webErr *Error
				require.ErrorAs(t, err, &webErr)
				assert.Equal(t, OperationError, webErr.Name)
			})
		})
	}
}

func TestAESCBCDecryptRoundTrip(t *testing.T) {
	t.Parallel()

	keyBytes := make([]byte, 16)
	for i := range keyBytes {
		keyBytes[i] = byte(i)
	}
	key := CryptoKey{handle: keyBytes}
	params := &AESCBCParams{Iv: make([]byte, 16)}

	ciphertext, err := params.Encrypt([]byte("hello webcrypto"), key)
	require.NoError(t, err)
	require.NotEmpty(t, ciphertext)
	require.Equal(t, 0, len(ciphertext)%16)

	plaintext, err := params.Decrypt(ciphertext, key)
	require.NoError(t, err)
	assert.Equal(t, []byte("hello webcrypto"), plaintext)
}

func TestAESGCMDecryptRejectsNon12ByteIV(t *testing.T) {
	t.Parallel()

	key := CryptoKey{handle: make([]byte, 16)}
	ct := make([]byte, 32)

	for _, ivLen := range []int{0, 1, 8, 11, 13, 16} {
		t.Run("ivLen="+strconv.Itoa(ivLen), func(t *testing.T) {
			t.Parallel()

			params := &AESGCMParams{Iv: make([]byte, ivLen)}
			require.NotPanics(t, func() {
				_, err := params.Decrypt(ct, key)
				assert.Error(t, err)

				var webErr *Error
				require.ErrorAs(t, err, &webErr)
				assert.Equal(t, NotSupportedError, webErr.Name)
			})
		})
	}
}

func TestAESGCMDecryptRoundTrip(t *testing.T) {
	t.Parallel()

	keyBytes := make([]byte, 16)
	for i := range keyBytes {
		keyBytes[i] = byte(i + 1)
	}
	key := CryptoKey{handle: keyBytes}
	params := &AESGCMParams{Iv: make([]byte, 12)}

	ciphertext, err := params.Encrypt([]byte("gcm plaintext"), key)
	require.NoError(t, err)

	plaintext, err := params.Decrypt(ciphertext, key)
	require.NoError(t, err)
	assert.Equal(t, []byte("gcm plaintext"), plaintext)
}
