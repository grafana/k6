package trace

import (
	"context"
	"errors"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"go.k6.io/k6/v2/internal/features"
)

const instrumentationName = "go.k6.io/k6"

// Span names of the k6 execution hierarchy: run -> scenario -> VU -> iteration.
const (
	SpanRun       = "k6.run"
	SpanScenario  = "scenario"
	SpanVU        = "vu"
	SpanIteration = "iteration"
)

// Tracer returns the tracer of the k6 execution hierarchy spans, or nil when they are
// disabled. Callers on hot paths should get the tracer once and keep it, as getting it
// from the SDK provider takes a global lock.
//
// The functions starting the spans of the hierarchy accept a nil tracer, and then return
// their context unchanged with a noop span.
func Tracer(tp *TracerProvider, flags *features.Flags) oteltrace.Tracer {
	if !NativeTracingEnabled(flags) || tp == nil || !tp.enabled {
		return nil
	}
	return tp.Tracer(instrumentationName)
}

// NativeTracingEnabled reports whether the native-tracing feature flag, which gates
// the k6 execution hierarchy spans, is enabled.
func NativeTracingEnabled(flags *features.Flags) bool {
	return flags != nil && flags.NativeTracing
}

// ScenarioInfo contains the identifiers attached to a scenario span.
type ScenarioInfo struct {
	Name     string
	Executor string
}

// VUInfo contains the identifiers attached to a VU span.
type VUInfo struct {
	ID       uint64
	IDGlobal uint64
	Scenario string
}

// IterationInfo contains the identifiers attached to an iteration span.
//
// The browser module creates its spans as children of the iteration span, and adds its
// traces metadata to it.
type IterationInfo struct {
	Number                      int64
	VUID                        uint64
	VUIDGlobal                  uint64
	VUIterationInScenario       uint64
	Scenario                    string
	ScenarioIterationInInstance uint64
	ScenarioIterationInTest     uint64
	// HasScenarioIterationNumbers is false when the executor doesn't provide the
	// scenario iteration counters, which are then left out.
	HasScenarioIterationNumbers bool
}

// StartTestRun starts the span of a test run, the root of a new trace.
func StartTestRun(
	ctx context.Context, tracer oteltrace.Tracer, logger logrus.FieldLogger,
) (context.Context, oteltrace.Span) {
	if tracer == nil {
		return ctx, noop.Span{}
	}
	ctx, span := tracer.Start(ctx, SpanRun, oteltrace.WithNewRoot())
	logger.WithField("trace_id", span.SpanContext().TraceID().String()).Debug("Tracing the test run")
	return ctx, span
}

// StartScenario starts a span for a scenario, child of the span in ctx (the run span).
func StartScenario(
	ctx context.Context, tracer oteltrace.Tracer, info ScenarioInfo,
) (context.Context, oteltrace.Span) {
	if tracer == nil {
		return ctx, noop.Span{}
	}
	return tracer.Start(ctx, SpanScenario, oteltrace.WithAttributes(
		attribute.String("test.scenario", info.Name),
		attribute.String("k6.scenario.executor", info.Executor),
	))
}

// StartVU starts a span for a VU activation, child of the span in ctx (the scenario span).
func StartVU(ctx context.Context, tracer oteltrace.Tracer, info VUInfo) (context.Context, oteltrace.Span) {
	if tracer == nil {
		return ctx, noop.Span{}
	}
	return tracer.Start(ctx, SpanVU, oteltrace.WithAttributes(
		uint64Attribute("test.vu", info.ID),
		uint64Attribute("k6.vu.id_in_instance", info.ID),
		uint64Attribute("k6.vu.id_in_test", info.IDGlobal),
		attribute.String("test.scenario", info.Scenario),
	))
}

// StartIteration starts a span for an iteration, child of the span in ctx (the VU span).
func StartIteration(
	ctx context.Context, tracer oteltrace.Tracer, info IterationInfo,
) (context.Context, oteltrace.Span) {
	if tracer == nil {
		return ctx, noop.Span{}
	}
	attrs := []attribute.KeyValue{
		attribute.Int64("test.iteration.number", info.Number),
		uint64Attribute("test.vu", info.VUID),
		attribute.String("test.scenario", info.Scenario),
		uint64Attribute("k6.vu.id_in_instance", info.VUID),
		uint64Attribute("k6.vu.id_in_test", info.VUIDGlobal),
		uint64Attribute("k6.vu.iteration_in_scenario", info.VUIterationInScenario),
	}
	if info.HasScenarioIterationNumbers {
		attrs = append(attrs,
			uint64Attribute("k6.scenario.iteration_in_instance", info.ScenarioIterationInInstance),
			uint64Attribute("k6.scenario.iteration_in_test", info.ScenarioIterationInTest),
		)
	}
	ctx, span := tracer.Start(ctx, SpanIteration, oteltrace.WithAttributes(attrs...))
	return context.WithValue(ctx, iterationSpanKey{}, span), span
}

// iterationSpanKey is the context key of the iteration span. It is kept apart from the active
// span, as the modules can create child spans, so the iteration span can still be found.
type iterationSpanKey struct{}

// MarkIterationFailed records reason as an error on the iteration span in ctx, e.g. when the
// script calls test.fail(). It does nothing when ctx has no iteration span, e.g. in setup, or
// when the iteration span has already ended.
func MarkIterationFailed(ctx context.Context, reason string) {
	span, ok := ctx.Value(iterationSpanKey{}).(oteltrace.Span)
	if !ok {
		return
	}
	span.RecordError(errors.New(reason))
	span.SetStatus(codes.Error, reason)
}

// EndIteration ends an iteration span. An interrupted iteration, e.g. by the end of its
// scenario, is flagged as such instead of being recorded as an error, as
// it is expected for iterations to be interrupted at the end of a test.
func EndIteration(span oteltrace.Span, err error, interrupted bool) {
	if interrupted {
		span.SetAttributes(attribute.Bool("k6.iteration.interrupted", true))
		span.End()
		return
	}
	EndSpan(span, err)
}

// EndSpan records err on the span, when present, and ends it.
func EndSpan(span oteltrace.Span, err error, opts ...oteltrace.SpanEndOption) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End(opts...)
}

func uint64Attribute(key string, value uint64) attribute.KeyValue {
	return attribute.Int64(key, int64(value)) //nolint:gosec
}
