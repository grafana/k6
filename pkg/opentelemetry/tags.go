package opentelemetry

import (
	"github.com/mstoykov/atlas"
	"go.k6.io/k6/metrics"
	"go.opentelemetry.io/otel/attribute"
)

// MapTagSet converts a k6 tag set into
// the equivalent set of opentelemetry attributes
func MapTagSet(t *metrics.TagSet) []attribute.KeyValue {
	n := (*atlas.Node)(t)
	if n.Len() < 1 {
		return nil
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
	return labels
}
