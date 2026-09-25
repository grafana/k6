package trace

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func TestSamplerFromConfig(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name      string
		sampler   string
		arg       string
		wantError error
	}{
		{name: "default"},
		{name: "always on", sampler: "always_on"},
		{name: "always off", sampler: "always_off"},
		{name: "parent based always on", sampler: "parentbased_always_on"},
		{name: "parent based always off", sampler: "parentbased_always_off"},
		{name: "ratio", sampler: "traceidratio", arg: "0.25"},
		{name: "parent based ratio", sampler: "parentbased_traceidratio", arg: "0.25"},
		{name: "invalid sampler", sampler: "sometimes", wantError: ErrInvalidSampler},
		{name: "invalid ratio", sampler: "traceidratio", arg: "2", wantError: ErrInvalidSamplerArg},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			sampler, err := SamplerFromConfig(testCase.sampler, testCase.arg)
			if testCase.wantError != nil {
				require.ErrorIs(t, err, testCase.wantError)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, sampler)
		})
	}
}

func TestPropagatorFromConfig(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"", "tracecontext", "jaeger"} {
		propagator, err := PropagatorFromConfig(name)
		require.NoError(t, err)
		require.NotNil(t, propagator)
	}
	_, err := PropagatorFromConfig("b3")
	require.ErrorIs(t, err, ErrInvalidPropagator)
}

func TestContextWithRemoteParent(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name       string
		propagator propagation.TextMapPropagator
		parent     string
		sampled    bool
	}{
		{
			name:       "trace context",
			propagator: propagation.TraceContext{},
			parent:     "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
			sampled:    true,
		},
		{
			name:       "jaeger",
			propagator: jaegerPropagator{},
			parent:     "4bf92f3577b34da6a3ce929d0e0e4736:00f067aa0ba902b7:0:0",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			ctx, err := ContextWithRemoteParent(t.Context(), testCase.parent, testCase.propagator)
			require.NoError(t, err)
			spanContext := oteltrace.SpanContextFromContext(ctx)
			require.True(t, spanContext.IsValid())
			require.True(t, spanContext.IsRemote())
			require.Equal(t, testCase.sampled, spanContext.IsSampled())
		})
	}

	_, err := ContextWithRemoteParent(t.Context(), "invalid", propagation.TraceContext{})
	require.ErrorIs(t, err, ErrInvalidParent)
}

func TestJaegerPropagatorInject(t *testing.T) {
	t.Parallel()

	traceID, err := oteltrace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	require.NoError(t, err)
	spanID, err := oteltrace.SpanIDFromHex("00f067aa0ba902b7")
	require.NoError(t, err)
	spanContext := oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
		TraceID: traceID, SpanID: spanID, TraceFlags: oteltrace.FlagsSampled,
	})
	ctx := oteltrace.ContextWithSpanContext(context.Background(), spanContext)
	carrier := propagation.MapCarrier{}
	jaegerPropagator{}.Inject(ctx, carrier)
	require.Equal(t,
		"4bf92f3577b34da6a3ce929d0e0e4736:00f067aa0ba902b7:0:1",
		carrier.Get(jaegerTraceHeader),
	)
}

func TestProviderWithoutExporterUsesSampler(t *testing.T) {
	t.Parallel()

	provider := NewTracerProviderWithoutExporter(sdktrace.NeverSample())
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })
	_, span := provider.Tracer("test").Start(t.Context(), "span")
	require.True(t, span.SpanContext().IsValid())
	require.False(t, span.SpanContext().IsSampled())
	require.False(t, span.IsRecording())
}
