package common

import (
	"testing"
	"time"

	"github.com/grafana/xk6-browser/k6ext/k6test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFrameGotoOptionsParse(t *testing.T) {
	t.Parallel()

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		vu := k6test.NewVU(t)
		opts := vu.ToGojaValue(map[string]interface{}{
			"timeout":   "1000",
			"waitUntil": "networkidle",
		})
		gotoOpts := NewFrameGotoOptions("https://example.com/", 0)
		err := gotoOpts.Parse(vu.Context(), opts)
		require.NoError(t, err)

		assert.Equal(t, "https://example.com/", gotoOpts.Referer)
		assert.Equal(t, time.Second, gotoOpts.Timeout)
		assert.Equal(t, LifecycleEventNetworkIdle, gotoOpts.WaitUntil)
	})

	t.Run("err/invalid_waitUntil", func(t *testing.T) {
		t.Parallel()

		vu := k6test.NewVU(t)
		opts := vu.ToGojaValue(map[string]interface{}{
			"waitUntil": "none",
		})
		navOpts := NewFrameGotoOptions("", 0)
		err := navOpts.Parse(vu.Context(), opts)

		assert.EqualError(t, err,
			`error parsing goto options: `+
				`invalid lifecycle event: "none"; must be one of: `+
				`load, domcontentloaded, networkidle`)
	})
}

func TestFrameSetContentOptionsParse(t *testing.T) {
	t.Parallel()

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		vu := k6test.NewVU(t)
		opts := vu.ToGojaValue(map[string]interface{}{
			"waitUntil": "networkidle",
		})
		scOpts := NewFrameSetContentOptions(30 * time.Second)
		err := scOpts.Parse(vu.Context(), opts)
		require.NoError(t, err)

		assert.Equal(t, 30*time.Second, scOpts.Timeout)
		assert.Equal(t, LifecycleEventNetworkIdle, scOpts.WaitUntil)
	})

	t.Run("err/invalid_waitUntil", func(t *testing.T) {
		t.Parallel()

		vu := k6test.NewVU(t)
		opts := vu.ToGojaValue(map[string]interface{}{
			"waitUntil": "none",
		})
		navOpts := NewFrameSetContentOptions(0)
		err := navOpts.Parse(vu.Context(), opts)

		assert.EqualError(t, err,
			`error parsing setContent options: `+
				`invalid lifecycle event: "none"; must be one of: `+
				`load, domcontentloaded, networkidle`)
	})
}

func TestFrameWaitForNavigationOptionsParse(t *testing.T) {
	t.Parallel()

	t.Run("ok", func(t *testing.T) {
		t.Parallel()

		vu := k6test.NewVU(t)
		opts := vu.ToGojaValue(map[string]interface{}{
			"url":       "https://example.com/",
			"timeout":   "1000",
			"waitUntil": "networkidle",
		})
		navOpts := NewFrameWaitForNavigationOptions(0)
		err := navOpts.Parse(vu.Context(), opts)
		require.NoError(t, err)

		assert.Equal(t, "https://example.com/", navOpts.URL)
		assert.Equal(t, time.Second, navOpts.Timeout)
		assert.Equal(t, LifecycleEventNetworkIdle, navOpts.WaitUntil)
	})

	t.Run("err/invalid_waitUntil", func(t *testing.T) {
		t.Parallel()

		vu := k6test.NewVU(t)
		opts := vu.ToGojaValue(map[string]interface{}{
			"waitUntil": "none",
		})
		navOpts := NewFrameWaitForNavigationOptions(0)
		err := navOpts.Parse(vu.Context(), opts)

		assert.EqualError(t, err,
			`error parsing waitForNavigation options: `+
				`invalid lifecycle event: "none"; must be one of: `+
				`load, domcontentloaded, networkidle`)
	})
}
