package trace

import (
	"context"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

const instrumentationName = "go.k6.io/k6"

type tracerProvider interface {
	Tracer(name string, options ...oteltrace.TracerOption) oteltrace.Tracer
}

// IterationInfo contains the execution identifiers attached to an iteration span.
type IterationInfo struct {
	Number                      int64
	VUID                        uint64
	VUIDGlobal                  uint64
	VUIterationInScenario       uint64
	Scenario                    string
	ScenarioIterationInInstance uint64
	ScenarioIterationInTest     uint64
	HasScenarioIterationNumbers bool
}

// ScenarioInfo contains the identifiers attached to a scenario span.
type ScenarioInfo struct {
	Name     string
	Executor string
}

// VUInfo contains the identifiers attached to a VU span.
type VUInfo struct {
	VUID       uint64
	VUIDGlobal uint64
	Scenario   string
}

// StartTestRun starts the span that represents a k6 test run.
func StartTestRun(
	ctx context.Context, provider tracerProvider, logger logrus.FieldLogger,
) (context.Context, oteltrace.Span) {
	options := make([]oteltrace.SpanStartOption, 0, 1)
	if !oteltrace.SpanContextFromContext(ctx).IsRemote() {
		options = append(options, oteltrace.WithNewRoot())
	}

	traceCtx, span := provider.Tracer(instrumentationName).Start(ctx, "k6.run", options...)

	if span.SpanContext().HasTraceID() {
		logger := logger.WithField("trace_id", span.SpanContext().TraceID())
		logger.Info("starting trace")
		context.AfterFunc(traceCtx, func() { logger.Info("stopping trace") })
	}

	return traceCtx, span
}

// StartScenario starts a child span representing a single scenario's run,
// nested under whatever span ctx carries (normally the test run span).
func StartScenario(
	ctx context.Context, provider tracerProvider, info ScenarioInfo,
) (context.Context, oteltrace.Span) {
	if provider == nil {
		return noop.NewTracerProvider().Tracer(instrumentationName).Start(ctx, "k6.scenario")
	}
	return provider.Tracer(instrumentationName).Start(ctx, "k6.scenario", oteltrace.WithAttributes(
		attribute.String("test.scenario", info.Name),
		attribute.String("k6.scenario.executor", info.Executor),
	))
}

// StartVU starts a child span representing a single VU activation, nested
// under whatever span ctx carries (normally the scenario span).
func StartVU(
	ctx context.Context, provider tracerProvider, info VUInfo,
) (context.Context, oteltrace.Span) {
	if provider == nil {
		return noop.NewTracerProvider().Tracer(instrumentationName).Start(ctx, "k6.vu")
	}
	return provider.Tracer(instrumentationName).Start(ctx, "k6.vu", oteltrace.WithAttributes(
		uint64Attribute("test.vu", info.VUID),
		uint64Attribute("k6.vu.id_in_instance", info.VUID),
		uint64Attribute("k6.vu.id_in_test", info.VUIDGlobal),
		attribute.String("test.scenario", info.Scenario),
	))
}

// StartIteration starts the span for a single iteration.
//
// ctx normally carries the VU span (scenario/VU spans are always created,
// regardless of split -- see StartScenario/StartVU). When split is true,
// the iteration gets its own independent root trace, merely linked to
// whatever span ctx carries (in practice, the VU span) instead of nesting
// under it -- this is the pre-existing behavior kept for --traces-split.
// When split is false, the iteration span is a normal child of that span.
func StartIteration(
	ctx context.Context, provider tracerProvider, info IterationInfo, split bool,
) (context.Context, oteltrace.Span) {
	if provider == nil {
		options := make([]oteltrace.SpanStartOption, 0, 1)
		if split {
			options = append(options, oteltrace.WithNewRoot())
		}
		return noop.NewTracerProvider().Tracer(instrumentationName).Start(ctx, "iteration", options...)
	}

	parentSpanContext := oteltrace.SpanContextFromContext(ctx)
	attrs := []attribute.KeyValue{
		attribute.Int64("test.iteration.number", info.Number),
		uint64Attribute("test.vu", info.VUID),
		attribute.String("test.scenario", info.Scenario),
		uint64Attribute("k6.vu.id_in_instance", info.VUID),
		uint64Attribute("k6.vu.id_in_test", info.VUIDGlobal),
		uint64Attribute("k6.vu.iteration_in_scenario", info.VUIterationInScenario),
	}
	if split && parentSpanContext.IsValid() {
		attrs = append(attrs, attribute.String("k6.run.id", parentSpanContext.TraceID().String()))
	}
	if info.HasScenarioIterationNumbers {
		attrs = append(attrs,
			uint64Attribute("k6.scenario.iteration_in_instance", info.ScenarioIterationInInstance),
			uint64Attribute("k6.scenario.iteration_in_test", info.ScenarioIterationInTest),
		)
	}

	options := []oteltrace.SpanStartOption{
		oteltrace.WithAttributes(attrs...),
	}
	if split {
		options = append(options, oteltrace.WithNewRoot())
		if parentSpanContext.IsValid() {
			options = append(options, oteltrace.WithLinks(oteltrace.Link{SpanContext: parentSpanContext}))
		}
	}
	return provider.Tracer(instrumentationName).Start(ctx, "iteration", options...)
}

func uint64Attribute(key string, value uint64) attribute.KeyValue {
	return attribute.Int64(key, int64(value)) //nolint:gosec
}

// EndSpan records err, when present, and ends span.
func EndSpan(span oteltrace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}
