package opentelemetry

import (
	"github.com/mstoykov/atlas"
	"go.k6.io/k6/metrics"
	"go.opentelemetry.io/otel/attribute"
)

// newAttributeSet converts a k6 tag set into
// the equivalent set of opentelemetry attributes
func newAttributeSet(t *metrics.TagSet, metadata map[string]string) attribute.Set {
	n := (*atlas.Node)(t)
	size := n.Len()
	if len(metadata) > 0 {
		size += len(metadata)
	}
	if size < 1 {
		return *attribute.EmptySet()
	}
	labels := make([]attribute.KeyValue, 0, size)
	for !n.IsRoot() {
		prev, key, value := n.Data()
		n = prev
		if key == "" || value == "" {
			continue
		}
		labels = append(labels, attribute.String(key, value))
	}
	for key, value := range metadata {
		if key == "" || value == "" {
			continue
		}
		labels = append(labels, attribute.String("meta."+key, value))
	}

	return attribute.NewSet(labels...)
}
