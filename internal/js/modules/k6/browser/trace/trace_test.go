package trace

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/embedded"
)

// recordingTracer is a minimal trace.Tracer that records started spans and the
// parent each was created under, so tests can assert parent/child linkage
// without pulling in the OTEL SDK (which is not vendored).
type recordingTracer struct {
	embedded.Tracer

	mu      sync.Mutex
	started []recordedSpan
	next    byte
}

type recordedSpan struct {
	name   string
	parent trace.SpanID
}

func (rt *recordingTracer) Start(
	ctx context.Context, name string, _ ...trace.SpanStartOption,
) (context.Context, trace.Span) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	parent := trace.SpanContextFromContext(ctx).SpanID()
	rt.next++
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{1},
		SpanID:  trace.SpanID{rt.next},
	})
	rt.started = append(rt.started, recordedSpan{name: name, parent: parent})

	return trace.ContextWithSpanContext(ctx, sc), recordingSpan{sc: sc}
}

func (rt *recordingTracer) parentOf(name string) (trace.SpanID, bool) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	for _, s := range rt.started {
		if s.name == name {
			return s.parent, true
		}
	}
	return trace.SpanID{}, false
}

type recordingSpan struct {
	trace.Span
	sc trace.SpanContext
}

func (s recordingSpan) SpanContext() trace.SpanContext { return s.sc }
func (s recordingSpan) IsRecording() bool              { return true }
func (s recordingSpan) End(...trace.SpanEndOption)     {}

type fakeTracerProvider struct{ tr trace.Tracer }

func (f fakeTracerProvider) Tracer(string, ...trace.TracerOption) trace.Tracer { return f.tr }

// TestTracerWebVitalAttachesToLiveNavigationSpan asserts that a web_vital event
// attaches to the target's current live navigation span, even when it arrives
// after the span has rotated (as late web vitals do). On the current code the
// event is dropped because its injected span ID no longer matches the live
// span; after the fix it attaches to the live span.
func TestTracerWebVitalAttachesToLiveNavigationSpan(t *testing.T) {
	t.Parallel()

	rec := &recordingTracer{}
	tracer := NewTracer(fakeTracerProvider{tr: rec}, map[string]string{})
	ctx := context.Background()

	tracer.TraceNavigation(ctx, "target")

	// A second navigation rotates the live span; the first is ended.
	_, nav2 := tracer.TraceNavigation(ctx, "target")
	liveID := nav2.SpanContext().SpanID()

	// A late web vital (delivered after the span rotated) must attach to the
	// target's live navigation span rather than being silently dropped.
	_, ev := tracer.TraceEvent(ctx, "target", "web_vital")
	ev.End()

	require.True(t, ev.SpanContext().IsValid(), "web_vital span was dropped (noop)")
	parent, ok := rec.parentOf("web_vital")
	require.True(t, ok, "web_vital span was never started")
	assert.Equal(t, liveID, parent, "web_vital should be a child of the live navigation span")
}
