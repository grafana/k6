package common

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/features"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/metrics"
)

func newMetricContextState(enabled bool) *lib.State {
	registry := metrics.NewRegistry()
	return &lib.State{
		FeatureFlags: &features.Flags{AsyncMetricContext: enabled},
		Tags: lib.NewVUStateTags(
			registry.RootTagSet().WithTagsFromMap(map[string]string{"group": lib.RootGroupPath}),
		),
	}
}

func setMetricContext(state *lib.State, group, phase, trace string) {
	state.Tags.Modify(func(tagsAndMeta *metrics.TagsAndMeta) {
		tagsAndMeta.SetTag("group", group)
		tagsAndMeta.SetTag("phase", phase)
		tagsAndMeta.SetMetadata("trace", trace)
	})
}

func requireMetricContext(t *testing.T, state *lib.State, group, phase, trace string) {
	t.Helper()
	current := state.Tags.GetCurrentValues()
	actualGroup, _ := current.Tags.Get("group")
	actualPhase, _ := current.Tags.Get("phase")
	require.Equal(t, group, actualGroup)
	require.Equal(t, phase, actualPhase)
	require.Equal(t, trace, current.Metadata["trace"])
}

func TestCapturedMetricContext(t *testing.T) {
	t.Parallel()

	state := newMetricContextState(true)
	setMetricContext(state, "::registered", "registered", "registered")
	captured := CaptureMetricContext(state)
	setMetricContext(state, "::active", "active", "active")

	restore := captured.Enter()
	requireMetricContext(t, state, "::registered", "registered", "registered")
	setMetricContext(state, "::callback", "callback", "callback")
	restore()
	requireMetricContext(t, state, "::active", "active", "active")

	// A reusable snapshot must not retain mutations from its previous invocation.
	restore = captured.Enter()
	requireMetricContext(t, state, "::registered", "registered", "registered")
	restore()
}

func TestCapturedMetricContextDisabled(t *testing.T) {
	t.Parallel()

	state := newMetricContextState(false)
	setMetricContext(state, "::registered", "registered", "registered")
	captured := CaptureMetricContext(state)
	setMetricContext(state, "::active", "active", "active")

	restore := captured.Enter()
	requireMetricContext(t, state, "::active", "active", "active")
	setMetricContext(state, "::callback", "callback", "callback")
	restore()
	requireMetricContext(t, state, "::callback", "callback", "callback")
}

func TestRunWithMetricContext(t *testing.T) {
	t.Parallel()

	state := newMetricContextState(true)
	setMetricContext(state, "::registered", "registered", "registered")
	captured := CaptureMetricContext(state)
	setMetricContext(state, "::active", "active", "active")

	value, err := RunWithMetricContext(captured, func() (string, error) {
		requireMetricContext(t, state, "::registered", "registered", "registered")
		setMetricContext(state, "::callback", "callback", "callback")
		return "result", nil
	})
	require.NoError(t, err)
	require.Equal(t, "result", value)
	requireMetricContext(t, state, "::active", "active", "active")
}

func TestMetricContextTracker(t *testing.T) {
	t.Parallel()

	state := newMetricContextState(true)
	tracker := NewMetricContextTracker(func() *lib.State { return state })
	setMetricContext(state, "::registered", "registered", "registered")
	captured := tracker.Grab()
	setMetricContext(state, "::active", "active", "active")

	tracker.Resumed(captured)
	requireMetricContext(t, state, "::registered", "registered", "registered")
	setMetricContext(state, "::callback", "callback", "callback")
	tracker.Exited()
	requireMetricContext(t, state, "::active", "active", "active")
}

func BenchmarkCapturedMetricContext(b *testing.B) {
	for _, metadataEntries := range []int{0, 4} {
		b.Run(strconv.Itoa(metadataEntries)+"_metadata", func(b *testing.B) {
			state := newMetricContextState(true)
			state.Tags.Modify(func(tagsAndMeta *metrics.TagsAndMeta) {
				tagsAndMeta.SetTag("phase", "registered")
				for i := range metadataEntries {
					tagsAndMeta.SetMetadata(string(rune('a'+i)), "value")
				}
			})
			captured := CaptureMetricContext(state)

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				restore := captured.Enter()
				restore()
			}
		})
	}
}
