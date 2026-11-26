package trace

import (
	"context"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/propagation"
	oteltrace "go.opentelemetry.io/otel/trace"
)

const (
	// TraceParentEnvVar is the environment variable holding the W3C trace context
	// "traceparent" of the trace that k6 continues.
	TraceParentEnvVar = "TRACEPARENT"
	// TraceStateEnvVar is the environment variable holding the optional W3C "tracestate".
	TraceStateEnvVar = "TRACESTATE"

	traceStateHeader = "Tracestate"
)

// ExtractParentFromEnv returns ctx carrying the remote span context described by the
// TRACEPARENT (and optional TRACESTATE) variables of env, so a test run started with it
// continues that trace.
//
// ctx is returned unchanged when TRACEPARENT is not set. When it is invalid a warning
// is logged and ctx is also returned unchanged, so k6 starts a new trace instead.
//
// A malformed TRACESTATE is ignored with a warning: the trace is still continued, without
// the tracestate.
//
// The sampled flag of the traceparent is only honored by a parent-based sampler, which is
// the OpenTelemetry SDK default: the spans of an unsampled trace are then not recorded.
// A sampler explicitly selected by the user, for example OTEL_TRACES_SAMPLER=always_on,
// takes precedence and can still record them.
func ExtractParentFromEnv(
	ctx context.Context, env map[string]string, logger logrus.FieldLogger,
) context.Context {
	traceParent := env[TraceParentEnvVar]
	if traceParent == "" {
		return ctx
	}

	// HeaderCarrier canonicalizes the keys, as the header constants of this package are.
	carrier := propagation.HeaderCarrier{}
	carrier.Set(traceparentHeader, traceParent)
	traceState := env[TraceStateEnvVar]
	if traceState != "" {
		carrier.Set(traceStateHeader, traceState)
	}

	extracted := propagation.TraceContext{}.Extract(ctx, carrier)
	sc := oteltrace.SpanContextFromContext(extracted)
	if !sc.IsValid() {
		logger.Warnf("Ignoring invalid %s environment variable, a new trace is started", TraceParentEnvVar)
		return ctx
	}

	// The propagator silently drops a malformed tracestate, report it.
	if traceState != "" && sc.TraceState().Len() == 0 {
		logger.Warnf("Ignoring invalid %s environment variable", TraceStateEnvVar)
	}
	if !sc.IsSampled() {
		logger.Debugf("The incoming trace is not sampled (%s flags), "+
			"its spans are not recorded unless the sampler is overridden", TraceParentEnvVar)
	}

	return extracted
}
