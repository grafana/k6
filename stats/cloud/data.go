package cloud

import (
	"time"

	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/lib/netext"

	"github.com/loadimpact/k6/stats"
)

// Sample is the generic struct that contains all types of data that we send to the cloud.
type Sample struct {
	Type   string      `json:"type"`
	Metric string      `json:"metric"`
	Data   interface{} `json:"data"`
}

// SampleDataSingle is used in all simple un-aggregated single-value samples.
type SampleDataSingle struct {
	Time  time.Time         `json:"time"`
	Type  stats.MetricType  `json:"type"`
	Tags  *stats.SampleTags `json:"tags,omitempty"`
	Value float64           `json:"value"`
}

// SampleDataMap is used by samples that contain multiple values, currently
// that's only iteration metrics (`iter_li_all`) and unaggregated HTTP
// requests (`http_req_li_all`).
type SampleDataMap struct {
	Time   time.Time          `json:"time"`
	Tags   *stats.SampleTags  `json:"tags,omitempty"`
	Values map[string]float64 `json:"values,omitempty"`
}

// NewSampleFromTrail just creates a ready-to-send Sample instance
// directly from a netext.Trail.
func NewSampleFromTrail(trail *netext.Trail) *Sample {
	return &Sample{
		Type:   "Points",
		Metric: "http_req_li_all",
		Data: SampleDataMap{
			Time: trail.GetTime(),
			Tags: trail.GetTags(),
			Values: map[string]float64{
				metrics.HTTPReqs.Name:        1,
				metrics.HTTPReqDuration.Name: stats.D(trail.Duration),

				metrics.HTTPReqBlocked.Name:        stats.D(trail.Blocked),
				metrics.HTTPReqConnecting.Name:     stats.D(trail.Connecting),
				metrics.HTTPReqTLSHandshaking.Name: stats.D(trail.TLSHandshaking),
				metrics.HTTPReqSending.Name:        stats.D(trail.Sending),
				metrics.HTTPReqWaiting.Name:        stats.D(trail.Waiting),
				metrics.HTTPReqReceiving.Name:      stats.D(trail.Receiving),
			},
		},
	}
}

// SampleDataAggregatedMap is used in aggregated samples for HTTP requests.
type SampleDataAggregatedMap struct {
	Time   time.Time                   `json:"time"`
	Type   string                      `json:"type"`
	Count  uint64                      `json:"count"`
	Tags   *stats.SampleTags           `json:"tags,omitempty"`
	Values map[string]AggregatedMetric `json:"values,omitempty"`
}

// AggregatedMetric is used to store aggregated information for a
// particular metric in an SampleDataAggregatedMap.
type AggregatedMetric struct {
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	Avg    float64 `json:"avg"`
	StdDev float64 `json:"stddev"`
}

type aggregationBucket map[*stats.SampleTags][]*netext.Trail
