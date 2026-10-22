// Package tracetest provides helpers to assert on the spans created by k6 in tests.
package tracetest

import (
	"context"
	"sync"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	k6trace "go.k6.io/k6/v2/internal/lib/trace"
)

// Recorder is a span processor that keeps the spans that have ended.
type Recorder struct {
	mu    sync.Mutex
	spans []sdktrace.ReadOnlySpan
}

// NewProvider returns a tracer provider recording every ended span in the returned Recorder.
// The options are passed to the SDK tracer provider, e.g. to set a sampler.
func NewProvider(opts ...sdktrace.TracerProviderOption) (*k6trace.TracerProvider, *Recorder) {
	rec := &Recorder{}
	opts = append(opts, sdktrace.WithSpanProcessor(rec))
	return k6trace.WrapTracerProvider(sdktrace.NewTracerProvider(opts...)), rec
}

// OnStart implements sdktrace.SpanProcessor.
func (*Recorder) OnStart(context.Context, sdktrace.ReadWriteSpan) {}

// OnEnd implements sdktrace.SpanProcessor.
func (r *Recorder) OnEnd(s sdktrace.ReadOnlySpan) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.spans = append(r.spans, s)
}

// Shutdown implements sdktrace.SpanProcessor.
func (*Recorder) Shutdown(context.Context) error { return nil }

// ForceFlush implements sdktrace.SpanProcessor.
func (*Recorder) ForceFlush(context.Context) error { return nil }

// Ended returns the spans that have ended, in the order they ended.
func (r *Recorder) Ended() []sdktrace.ReadOnlySpan {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]sdktrace.ReadOnlySpan(nil), r.spans...)
}

// ByName returns the ended spans with the given name.
func (r *Recorder) ByName(name string) []sdktrace.ReadOnlySpan {
	var out []sdktrace.ReadOnlySpan
	for _, s := range r.Ended() {
		if s.Name() == name {
			out = append(out, s)
		}
	}
	return out
}
