package state

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetFlags_IgnoresLegacyBinaryProvisioning(t *testing.T) {
	t.Parallel()

	defaults := GetDefaultFlags(".config", ".cache")

	t.Run("K6_BINARY_PROVISIONING has no effect", func(t *testing.T) {
		t.Parallel()

		env := map[string]string{"K6_BINARY_PROVISIONING": "false"}
		flags := getFlags(defaults, env, nil)
		assert.True(t, flags.AutoExtensionResolution,
			"K6_BINARY_PROVISIONING should be ignored; AutoExtensionResolution should remain default (true)")
	})

	t.Run("K6_AUTO_EXTENSION_RESOLUTION still works", func(t *testing.T) {
		t.Parallel()

		env := map[string]string{"K6_AUTO_EXTENSION_RESOLUTION": "false"}
		flags := getFlags(defaults, env, nil)
		assert.False(t, flags.AutoExtensionResolution,
			"K6_AUTO_EXTENSION_RESOLUTION=false should disable AutoExtensionResolution")
	})
}
