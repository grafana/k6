package trace

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
)

const jaegerTraceHeader = "uber-trace-id"

var (
	// ErrInvalidSampler indicates that the configured sampler is not supported.
	ErrInvalidSampler = errors.New("invalid traces sampler")
	// ErrInvalidSamplerArg indicates that the sampler argument is not a ratio between zero and one.
	ErrInvalidSamplerArg = errors.New("invalid traces sampler argument")
	// ErrInvalidPropagator indicates that the configured propagator is not supported.
	ErrInvalidPropagator = errors.New("invalid traces propagator")
	// ErrInvalidParent indicates that the configured remote parent cannot be extracted.
	ErrInvalidParent = errors.New("invalid traces parent")
)

// SamplerFromConfig creates one of the OpenTelemetry standard samplers.
func SamplerFromConfig(name, arg string) (sdktrace.Sampler, error) {
	if name == "" {
		name = "parentbased_always_on"
	}

	switch name {
	case "always_on":
		return sdktrace.AlwaysSample(), nil
	case "always_off":
		return sdktrace.NeverSample(), nil
	case "parentbased_always_on":
		return sdktrace.ParentBased(sdktrace.AlwaysSample()), nil
	case "parentbased_always_off":
		return sdktrace.ParentBased(sdktrace.NeverSample()), nil
	case "traceidratio", "parentbased_traceidratio":
		ratio, err := strconv.ParseFloat(arg, 64)
		if err != nil || ratio < 0 || ratio > 1 {
			return nil, fmt.Errorf("%w %q", ErrInvalidSamplerArg, arg)
		}
		ratioSampler := sdktrace.TraceIDRatioBased(ratio)
		if name == "traceidratio" {
			return ratioSampler, nil
		}
		return sdktrace.ParentBased(ratioSampler), nil
	default:
		return nil, fmt.Errorf("%w %q", ErrInvalidSampler, name)
	}
}

// PropagatorFromConfig creates the configured trace context propagator.
func PropagatorFromConfig(name string) (propagation.TextMapPropagator, error) {
	switch name {
	case "", "tracecontext":
		return propagation.TraceContext{}, nil
	case "jaeger":
		return jaegerPropagator{}, nil
	default:
		return nil, fmt.Errorf("%w %q", ErrInvalidPropagator, name)
	}
}

// PropagatorOrDefault returns propagator, or W3C Trace Context when it is nil.
func PropagatorOrDefault(propagator propagation.TextMapPropagator) propagation.TextMapPropagator {
	if propagator == nil {
		return propagation.TraceContext{}
	}
	return propagator
}

// ContextWithRemoteParent extracts parent into ctx using propagator. The parent
// value is the selected propagator's primary trace header value.
func ContextWithRemoteParent(
	ctx context.Context, parent string, propagator propagation.TextMapPropagator,
) (context.Context, error) {
	if parent == "" {
		return ctx, nil
	}
	fields := propagator.Fields()
	if len(fields) == 0 {
		return ctx, ErrInvalidParent
	}
	ctx = propagator.Extract(ctx, propagation.MapCarrier{fields[0]: parent})
	if !oteltrace.SpanContextFromContext(ctx).IsValid() {
		return ctx, fmt.Errorf("%w %q", ErrInvalidParent, parent)
	}
	return ctx, nil
}

// jaegerPropagator implements the Jaeger uber-trace-id format without pulling
// a second OpenTelemetry module into k6.
type jaegerPropagator struct{}

func (jaegerPropagator) Inject(ctx context.Context, carrier propagation.TextMapCarrier) {
	spanContext := oteltrace.SpanContextFromContext(ctx)
	if !spanContext.IsValid() {
		return
	}
	flags := 0
	if spanContext.IsSampled() {
		flags = 1
	}
	carrier.Set(jaegerTraceHeader, fmt.Sprintf(
		"%s:%s:0:%x", spanContext.TraceID(), spanContext.SpanID(), flags,
	))
}

func (jaegerPropagator) Extract(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	parts := strings.Split(carrier.Get(jaegerTraceHeader), ":")
	if len(parts) != 4 {
		return ctx
	}
	traceIDValue := parts[0]
	if len(traceIDValue) > 32 {
		return ctx
	}
	traceIDValue = strings.Repeat("0", 32-len(traceIDValue)) + traceIDValue
	traceID, err := oteltrace.TraceIDFromHex(traceIDValue)
	if err != nil {
		return ctx
	}
	spanIDValue := parts[1]
	if len(spanIDValue) > 16 {
		return ctx
	}
	spanIDValue = strings.Repeat("0", 16-len(spanIDValue)) + spanIDValue
	spanID, err := oteltrace.SpanIDFromHex(spanIDValue)
	if err != nil {
		return ctx
	}
	flags, err := strconv.ParseUint(parts[3], 16, 64)
	if err != nil {
		return ctx
	}
	traceFlags := oteltrace.TraceFlags(0)
	if flags&1 == 1 {
		traceFlags = oteltrace.FlagsSampled
	}
	spanContext := oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: traceFlags,
		Remote:     true,
	})
	if !spanContext.IsValid() {
		return ctx
	}
	return oteltrace.ContextWithRemoteSpanContext(ctx, spanContext)
}

func (jaegerPropagator) Fields() []string { return []string{jaegerTraceHeader} }
