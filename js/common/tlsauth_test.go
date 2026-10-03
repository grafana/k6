package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTLSAuthValidation(t *testing.T) {
	t.Parallel()

	t.Run("rejects_non_object", func(t *testing.T) {
		t.Parallel()
		_, err := ParseTLSAuth("not-an-object")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be an object")
	})

	t.Run("rejects_missing_cert", func(t *testing.T) {
		t.Parallel()
		_, err := ParseTLSAuth(map[string]any{"key": "-----BEGIN PRIVATE KEY-----\n-----END PRIVATE KEY-----"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "tlsAuth.cert")
	})

	t.Run("rejects_missing_key", func(t *testing.T) {
		t.Parallel()
		_, err := ParseTLSAuth(map[string]any{"cert": "-----BEGIN CERTIFICATE-----\n-----END CERTIFICATE-----"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "tlsAuth.key")
	})

	t.Run("rejects_non_string_password", func(t *testing.T) {
		t.Parallel()
		_, err := ParseTLSAuth(map[string]any{
			"cert":     "-----BEGIN CERTIFICATE-----\n-----END CERTIFICATE-----",
			"key":      "-----BEGIN PRIVATE KEY-----\n-----END PRIVATE KEY-----",
			"password": 123,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "tlsAuth.password")
	})
}
