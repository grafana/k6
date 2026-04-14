package opentelemetry

import (
	"go.k6.io/k6/metrics"
	"go.opentelemetry.io/otel/attribute"
)

// newAttributeSet converts a k6 tag set into
// the equivalent set of opentelemetry attributes
func newAttributeSet(t *metrics.TagSet) attribute.Set {
	if t.Len() < 1 {
		return *attribute.EmptySet()
	}
	labels := make([]attribute.KeyValue, 0, t.Len())
	t.ForEach(func(key, value string) {
		if key == "" || value == "" {
			return
		}
		labels = append(labels, attribute.String(key, value))
	})

	return attribute.NewSet(labels...)
}
