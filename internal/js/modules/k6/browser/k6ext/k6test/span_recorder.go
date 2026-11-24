package k6test

import (
	"context"
	"sync"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// RecordedSpan is a snapshot of a span captured by a SpanRecorder, exposing just
// enough for tests to assert on span names, parentage, attributes and end state.
type RecordedSpan struct {
	Name     string
	TraceID  oteltrace.TraceID
	SpanID   oteltrace.SpanID
	ParentID oteltrace.SpanID
	Attrs    []attribute.KeyValue
}

// AttrInt64 returns the int64 value of the named attribute on the span, failing
// the test if it is absent.
func (s RecordedSpan) AttrInt64(tb testing.TB, key string) int64 {
	tb.Helper()
	for _, kv := range s.Attrs {
		if string(kv.Key) == key {
			return kv.Value.AsInt64()
		}
	}
	tb.Fatalf("attribute %q not found on span %q", key, s.Name)
	return 0
}

// AttrString returns the string value of the named attribute on the span,
// failing the test if it is absent.
func (s RecordedSpan) AttrString(tb testing.TB, key string) string {
	tb.Helper()
	for _, kv := range s.Attrs {
		if string(kv.Key) == key {
			return kv.Value.AsString()
		}
	}
	tb.Fatalf("attribute %q not found on span %q", key, s.Name)
	return ""
}

// SpanRecorder is a minimal sdktrace.SpanProcessor that records started spans
// (for name/parent/attribute inspection) and which span ids have ended.
type SpanRecorder struct {
	mu      sync.Mutex
	started []RecordedSpan
	ended   map[oteltrace.SpanID]bool
}

// NewSpanRecorder returns a ready-to-use SpanRecorder.
func NewSpanRecorder() *SpanRecorder {
	return &SpanRecorder{ended: make(map[oteltrace.SpanID]bool)}
}

// OnStart implements sdktrace.SpanProcessor.
func (r *SpanRecorder) OnStart(_ context.Context, s sdktrace.ReadWriteSpan) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.started = append(r.started, RecordedSpan{
		Name:     s.Name(),
		TraceID:  s.SpanContext().TraceID(),
		SpanID:   s.SpanContext().SpanID(),
		ParentID: s.Parent().SpanID(),
		Attrs:    s.Attributes(),
	})
}

// OnEnd implements sdktrace.SpanProcessor.
func (r *SpanRecorder) OnEnd(s sdktrace.ReadOnlySpan) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ended[s.SpanContext().SpanID()] = true
}

// Shutdown implements sdktrace.SpanProcessor.
func (r *SpanRecorder) Shutdown(context.Context) error { return nil }

// ForceFlush implements sdktrace.SpanProcessor.
func (r *SpanRecorder) ForceFlush(context.Context) error { return nil }

// Find returns the first recorded span with the given name.
func (r *SpanRecorder) Find(name string) (RecordedSpan, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.started {
		if s.Name == name {
			return s, true
		}
	}
	return RecordedSpan{}, false
}

// IsEnded reports whether the span with the given id has ended.
func (r *SpanRecorder) IsEnded(id oteltrace.SpanID) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ended[id]
}
