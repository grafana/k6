package remotewrite

import (
	"sort"

	prompb "buf.build/gen/go/prometheus/prometheus/protocolbuffers/go"
	"github.com/mstoykov/atlas"
	"go.k6.io/k6/metrics"
)

const namelbl = "__name__"

// MapTagSet converts a k6 tag set into
// the equivalent set of Labels as expected from the
// Prometheus' data model.
func MapTagSet(t *metrics.TagSet) []*prompb.Label {
	n := (*atlas.Node)(t)
	if n.Len() < 1 {
		return nil
	}
	labels := make([]*prompb.Label, 0, n.Len())
	for !n.IsRoot() {
		prev, key, value := n.Data()
		n = prev
		if key == "" || value == "" {
			continue
		}
		labels = append(labels, &prompb.Label{Name: key, Value: value})
	}
	return labels
}

// MapSeries converts a k6 time series into
// the equivalent set of Labels (name+tags) as expected from the
// Prometheus' data model.
//
// The labels are lexicographic sorted as required
// from the Remote write's specification.
func MapSeries(series metrics.TimeSeries, suffix string) []*prompb.Label {
	v := defaultMetricPrefix + series.Metric.Name
	if suffix != "" {
		v += "_" + suffix
	}
	lbls := append(MapTagSet(series.Tags), &prompb.Label{
		Name:  namelbl,
		Value: v,
	})
	sort.Slice(lbls, func(i int, j int) bool {
		return lbls[i].Name < lbls[j].Name
	})
	return lbls
}
