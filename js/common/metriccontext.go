package common

import (
	"github.com/grafana/sobek"

	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/metrics"
)

// metricContextTracker propagates metric context through promise reactions, including await
// continuations. Promise reactions do not nest and run on the event-loop goroutine.
type metricContextTracker struct {
	state   func() *lib.State
	restore func()
}

// CapturedMetricContext is an immutable registration-time metric context. Its zero value is a
// disabled context, so callers don't need separate feature-flag or nil-state branches.
type CapturedMetricContext struct {
	state       *lib.State
	tagsAndMeta metrics.TagsAndMeta
}

var _ sobek.AsyncContextTracker = (*metricContextTracker)(nil)

// NewMetricContextTracker returns an async-context tracker backed by the current VU state.
func NewMetricContextTracker(state func() *lib.State) sobek.AsyncContextTracker {
	return &metricContextTracker{state: state}
}

// Grab captures the metric context when a promise reaction is created.
func (t *metricContextTracker) Grab() any {
	state := t.state()
	if state == nil || state.Tags == nil {
		return nil
	}
	return state.Tags.GetCurrentValues()
}

// Resumed applies the context captured for a promise reaction.
func (t *metricContextTracker) Resumed(trackingObject any) {
	captured, ok := trackingObject.(metrics.TagsAndMeta)
	state := t.state()
	if !ok || state == nil || state.Tags == nil {
		t.restore = nil
		return
	}
	t.restore = ApplyMetricContext(state, captured)
}

// Exited restores the context that was active before the promise reaction ran.
func (t *metricContextTracker) Exited() {
	if t.restore != nil {
		t.restore()
		t.restore = nil
	}
}

// AsyncMetricContextEnabled reports whether asynchronous metric-context propagation is enabled.
func AsyncMetricContextEnabled(state *lib.State) bool {
	return state != nil && state.FeatureFlags != nil && state.FeatureFlags.AsyncMetricContext
}

// CaptureMetricContext captures the current tags and metadata for work that will run outside
// Sobek's promise machinery.
func CaptureMetricContext(state *lib.State) CapturedMetricContext {
	if !AsyncMetricContextEnabled(state) || state.Tags == nil {
		return CapturedMetricContext{}
	}
	return CapturedMetricContext{state: state, tagsAndMeta: state.Tags.GetCurrentValues()}
}

// Enter installs a clone of the captured context and returns a function that restores the context
// it interrupted. Entering a disabled context is a no-op.
func (c CapturedMetricContext) Enter() (restore func()) {
	if c.state == nil {
		return func() {}
	}
	return ApplyMetricContext(c.state, c.tagsAndMeta)
}

// RunWithMetricContext executes fn under captured and supports callbacks that return a value.
func RunWithMetricContext[T any](captured CapturedMetricContext, fn func() (T, error)) (T, error) {
	if captured.state == nil {
		return fn()
	}
	restore := captured.Enter()
	defer restore()
	return fn()
}

// WrapSobekCallable captures invocation under c while preserving the callable's arguments and
// return value.
func (c CapturedMetricContext) WrapSobekCallable(fn sobek.Callable) sobek.Callable {
	if fn == nil || c.state == nil {
		return fn
	}
	return func(this sobek.Value, args ...sobek.Value) (sobek.Value, error) {
		return RunWithMetricContext(c, func() (sobek.Value, error) {
			return fn(this, args...)
		})
	}
}

// ApplyMetricContext replaces the current tags and metadata until the returned function restores
// them. Both assignments clone metadata so mutations cannot change a reusable captured snapshot.
func ApplyMetricContext(state *lib.State, captured metrics.TagsAndMeta) (restore func()) {
	previous := state.Tags.GetCurrentValues()
	state.Tags.Modify(func(tagsAndMeta *metrics.TagsAndMeta) {
		*tagsAndMeta = captured.Clone()
	})
	return func() {
		state.Tags.Modify(func(tagsAndMeta *metrics.TagsAndMeta) {
			*tagsAndMeta = previous
		})
	}
}
