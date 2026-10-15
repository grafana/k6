package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/features"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/k6ext"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/k6ext/k6test"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/log"
	k6metrics "go.k6.io/k6/v2/metrics"
)

// reportWebVitalGroup builds a FrameSession backed by a test VU, captures a
// navigation's metric context under navGroup, then changes the live group tag
// to liveGroup before reporting an LCP web vital. It returns the group tag on
// the resulting buffered sample. asyncEnabled toggles the async-metric-context
// feature flag.
func reportWebVitalGroup(t *testing.T, asyncEnabled bool, navGroup, liveGroup string) string {
	t.Helper()

	registry := k6metrics.NewRegistry()
	k6m := k6ext.RegisterCustomMetrics(registry)
	vu := k6test.NewVU(t)
	vu.ActivateVU()

	state := vu.State()
	state.FeatureFlags = &features.Flags{AsyncMetricContext: asyncEnabled}

	page := &Page{}
	// Capture the navigation's metric context, then settle the operation so it
	// becomes the page's most-recently-completed network operation, mirroring a
	// page load finishing before the browser reports its web vitals.
	state.Tags.Modify(func(tagsAndMeta *k6metrics.TagsAndMeta) {
		tagsAndMeta.SetTag(k6metrics.TagGroup.String(), navGroup)
	})
	finishAndRetain, _ := page.BeginNetworkOperation(state.Tags.GetCurrentValues())
	finishAndRetain()

	// The VU moves on: the live group tag no longer reflects the navigation that
	// produced the web vital.
	state.Tags.Modify(func(tagsAndMeta *k6metrics.TagsAndMeta) {
		tagsAndMeta.SetTag(k6metrics.TagGroup.String(), liveGroup)
	})

	fs := &FrameSession{
		ctx:               vu.Context(),
		vu:                vu,
		page:              page,
		k6Metrics:         k6m,
		bufferedWebVitals: make(map[webVitalKey]webVitalSample),
		logger:            log.NewNullLogger(),
	}

	const lcp = `{"Name":"LCP","Value":123.4,"Rating":"good","URL":"https://example.test/"}`
	require.NoError(t, fs.parseAndEmitWebVitalMetric(lcp))
	require.Len(t, fs.bufferedWebVitals, 1)

	var group string
	for _, wv := range fs.bufferedWebVitals {
		g, ok := wv.sample.Tags.Get(k6metrics.TagGroup.String())
		require.True(t, ok)
		group = g
	}
	return group
}

// TestParseAndEmitWebVitalMetricUsesNavigationContext verifies that, with the
// async-metric-context feature enabled, a web vital reported asynchronously is
// attributed to the group active during the navigation that produced it, rather
// than whatever group happens to be active when the CDP binding event fires.
func TestParseAndEmitWebVitalMetricUsesNavigationContext(t *testing.T) {
	t.Parallel()

	t.Run("enabled uses captured navigation group", func(t *testing.T) {
		t.Parallel()
		group := reportWebVitalGroup(t, true, "::nav", "::after")
		assert.Equal(t, "::nav", group)
	})

	t.Run("disabled uses live group", func(t *testing.T) {
		t.Parallel()
		group := reportWebVitalGroup(t, false, "::nav", "::after")
		assert.Equal(t, "::after", group)
	})
}
