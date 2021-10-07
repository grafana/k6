package remotewrite

import (
	"github.com/prometheus/prometheus/prompb"
	"go.k6.io/k6/stats"
)

func tagsToPrometheusLabels(tags *stats.SampleTags) ([]prompb.Label, error) {
	tagsMap := tags.CloneTags()
	labelPairs := make([]prompb.Label, 0, len(tagsMap))

	for name, value := range tagsMap {
		if len(name) < 1 || len(value) < 1 {
			continue
		}
		// TODO add checks:
		// - reserved underscore
		// - sorting
		// - duplicates?

		labelPairs = append(labelPairs, prompb.Label{
			Name:  name,
			Value: value,
		})
	}

	// names of the metrics might be remote agent dependent so let Mapping set those

	return labelPairs[:len(labelPairs):len(labelPairs)], nil
}

// func (l labels) Len() int           { return len(l) }
// func (l labels) Less(i, j int) bool { return l[i].Name < l[j].Name }
// func (l labels) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }
