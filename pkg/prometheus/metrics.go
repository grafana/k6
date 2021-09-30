package prometheus

import (
	"github.com/prometheus/prometheus/pkg/timestamp"
	"github.com/prometheus/prometheus/prompb"
	"go.k6.io/k6/stats"
)

func getCounter(sample *stats.Sample, labelPairs []prompb.Label) []prompb.TimeSeries {
	return []prompb.TimeSeries{
		{
			Labels: append(labelPairs, prompb.Label{
				Name: "__name__",
				// we cannot use name tag as it can be absent or equal to URL in HTTP testing
				Value: sample.Metric.Name,
			}),
			Samples: []prompb.Sample{
				{
					Value:     sample.Value,
					Timestamp: timestamp.FromTime(sample.Time),
				},
			},
		},
	}
}

func getGauge(sample *stats.Sample, labelPairs []prompb.Label) []prompb.TimeSeries {
	return getCounter(sample, labelPairs)
}

func getHistogram(sample *stats.Sample, labelPairs []prompb.Label) []prompb.TimeSeries {
	baseSample := prompb.Sample{
		Value:     sample.Value,
		Timestamp: timestamp.FromTime(sample.Time),
	}

	sumTimeSeries := prompb.TimeSeries{
		Labels: append(labelPairs, prompb.Label{
			Name:  "__name__",
			Value: sample.Metric.Name + "_sum",
		}),
		Samples: []prompb.Sample{baseSample},
	}

	additionalLabel := prompb.Label{
		Name: "le",
	}
	if sample.Value > 0 {
		additionalLabel.Value = "+Inf"
	} else {
		additionalLabel.Value = "0"
	}
	bucketTimeSeries := []prompb.TimeSeries{
		{
			Labels: append(labelPairs, prompb.Label{
				Name:  "__name__",
				Value: sample.Metric.Name + "_bucket",
			}, additionalLabel),
			Samples: []prompb.Sample{
				{
					Value:     sample.Value,
					Timestamp: timestamp.FromTime(sample.Time),
				},
			},
		},
	}

	countTimeSeries := prompb.TimeSeries{
		Labels: append(labelPairs, prompb.Label{
			Name:  "__name__",
			Value: sample.Metric.Name + "_count",
		}),
		Samples: []prompb.Sample{
			{
				Value:     1,
				Timestamp: timestamp.FromTime(sample.Time),
			},
		},
	}

	return append(bucketTimeSeries, sumTimeSeries, countTimeSeries)
}
