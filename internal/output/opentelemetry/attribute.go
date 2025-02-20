package opentelemetry

import (
	"github.com/mstoykov/atlas"
	"go.k6.io/k6/metrics"
	"go.opentelemetry.io/otel/attribute"
)

// newAttributeSet converts a k6 tag set into
// the equivalent set of opentelemetry attributes
func newAttributeSet(t *metrics.TagSet) attribute.Set {
	n := (*atlas.Node)(t)
	if n.Len() < 1 {
		return *attribute.EmptySet()
	}
	labels := make([]attribute.KeyValue, 0, n.Len())
	for !n.IsRoot() {
		prev, key, value := n.Data()
		n = prev
		if key == "" || value == "" {
			continue
		}
		labels = append(labels, attribute.String(key, value))
	}

	return attribute.NewSet(labels...)
}
