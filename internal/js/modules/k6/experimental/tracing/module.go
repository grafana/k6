// Package tracing provides script access to spans created by k6.
package tracing

import (
	"errors"
	"fmt"

	"github.com/grafana/sobek"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"

	"go.k6.io/k6/v2/js/common"
	"go.k6.io/k6/v2/js/modules"
)

// ErrCurrentSpanInInitContext is returned when currentSpan is called in the init context.
var ErrCurrentSpanInInitContext = common.NewInitContextError(
	"getting the current span in the init context is not supported",
)

// RootModule creates tracing module instances for each VU.
type RootModule struct{}

// ModuleInstance is a per-VU instance of the tracing module.
type ModuleInstance struct {
	vu modules.VU
}

// Span provides script access to a k6-owned span.
type Span struct {
	TraceID   string `js:"traceId"`
	SpanID    string `js:"spanId"`
	Sampled   bool   `js:"sampled"`
	Recording bool   `js:"recording"`

	span oteltrace.Span
}

var (
	_ modules.Module   = &RootModule{}
	_ modules.Instance = &ModuleInstance{}
)

// New returns a new tracing root module.
func New() *RootModule {
	return &RootModule{}
}

// NewModuleInstance implements modules.Module.
func (*RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	return &ModuleInstance{vu: vu}
}

// Exports implements modules.Instance.
func (mi *ModuleInstance) Exports() modules.Exports {
	return modules.Exports{Named: map[string]any{"currentSpan": mi.CurrentSpan}}
}

// CurrentSpan returns the span associated with the current k6 execution context.
func (mi *ModuleInstance) CurrentSpan() (*Span, error) {
	if mi.vu.State() == nil {
		return nil, ErrCurrentSpanInInitContext
	}

	span := oteltrace.SpanFromContext(mi.vu.Context())
	spanContext := span.SpanContext()
	result := &Span{
		Sampled:   spanContext.IsSampled(),
		Recording: span.IsRecording(),
		span:      span,
	}
	if spanContext.IsValid() {
		result.TraceID = spanContext.TraceID().String()
		result.SpanID = spanContext.SpanID().String()
	}
	return result, nil
}

// SetAttribute adds or replaces an attribute on the span.
func (s *Span) SetAttribute(name string, value sobek.Value) error {
	if name == "" {
		return errors.New("span attribute name cannot be empty")
	}
	if common.IsNullish(value) {
		return errors.New("span attribute value cannot be null or undefined")
	}

	attr, err := spanAttribute(name, value.Export())
	if err != nil {
		return err
	}
	s.span.SetAttributes(attr)
	return nil
}

// AddEvent adds a timestamped event to the span.
func (s *Span) AddEvent(name string) error {
	if name == "" {
		return errors.New("span event name cannot be empty")
	}
	s.span.AddEvent(name)
	return nil
}

func spanAttribute(name string, value any) (attribute.KeyValue, error) {
	switch value := value.(type) {
	case bool:
		return attribute.Bool(name, value), nil
	case int64:
		return attribute.Int64(name, value), nil
	case float64:
		return attribute.Float64(name, value), nil
	case string:
		return attribute.String(name, value), nil
	default:
		return attribute.KeyValue{}, fmt.Errorf(
			"unsupported span attribute value type %T; expected a boolean, number, or string", value,
		)
	}
}
