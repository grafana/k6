package trace

import (
	"context"

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

// StartTestRun starts the span that represents a k6 test run.
func StartTestRun(
	ctx context.Context, provider tracerProvider,
) (context.Context, oteltrace.Span) {
	options := make([]oteltrace.SpanStartOption, 0, 1)
	if !oteltrace.SpanContextFromContext(ctx).IsRemote() {
		options = append(options, oteltrace.WithNewRoot())
	}
	return provider.Tracer(instrumentationName).Start(ctx, "k6.run", options...)
}

// StartIteration starts an independent root trace for an iteration and links it
// to the test run span carried by ctx.
func StartIteration(
	ctx context.Context, provider tracerProvider, info IterationInfo,
) (context.Context, oteltrace.Span) {
	if provider == nil {
		return noop.NewTracerProvider().Tracer(instrumentationName).Start(
			ctx, "iteration", oteltrace.WithNewRoot(),
		)
	}

	runSpanContext := oteltrace.SpanContextFromContext(ctx)
	attrs := []attribute.KeyValue{
		attribute.Int64("test.iteration.number", info.Number),
		uint64Attribute("test.vu", info.VUID),
		attribute.String("test.scenario", info.Scenario),
		uint64Attribute("k6.vu.id_in_instance", info.VUID),
		uint64Attribute("k6.vu.id_in_test", info.VUIDGlobal),
		uint64Attribute("k6.vu.iteration_in_scenario", info.VUIterationInScenario),
	}
	if runSpanContext.IsValid() {
		attrs = append(attrs, attribute.String("k6.run.id", runSpanContext.TraceID().String()))
	}
	if info.HasScenarioIterationNumbers {
		attrs = append(attrs,
			uint64Attribute("k6.scenario.iteration_in_instance", info.ScenarioIterationInInstance),
			uint64Attribute("k6.scenario.iteration_in_test", info.ScenarioIterationInTest),
		)
	}

	options := []oteltrace.SpanStartOption{
		oteltrace.WithNewRoot(),
		oteltrace.WithAttributes(attrs...),
	}
	if runSpanContext.IsValid() {
		options = append(options, oteltrace.WithLinks(oteltrace.Link{SpanContext: runSpanContext}))
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
