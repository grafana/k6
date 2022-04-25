package common

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFrameWaitForNavigationOptionsParse(t *testing.T) {
	t.Parallel()

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		mockVU := newMockVU(t)
		opts := mockVU.RuntimeField.ToValue(map[string]interface{}{
			"url":       "https://example.com/",
			"timeout":   "1000",
			"waitUntil": "networkidle",
		})
		navOpts := NewFrameWaitForNavigationOptions(0)
		err := navOpts.Parse(mockVU.CtxField, opts)
		require.NoError(t, err)

		assert.Equal(t, "https://example.com/", navOpts.URL)
		assert.Equal(t, time.Second, navOpts.Timeout)
		assert.Equal(t, LifecycleEventNetworkIdle, navOpts.WaitUntil)
	})

	t.Run("err/invalid_waitUntil", func(t *testing.T) {
		t.Parallel()

		mockVU := newMockVU(t)
		opts := mockVU.RuntimeField.ToValue(map[string]interface{}{
			"waitUntil": "none",
		})
		navOpts := NewFrameWaitForNavigationOptions(0)
		err := navOpts.Parse(mockVU.CtxField, opts)

		assert.EqualError(t, err,
			`error parsing waitForNavigation options: `+
				`invalid lifecycle event: "none"; must be one of: `+
				`load, domcontentloaded, networkidle`)
	})
}
