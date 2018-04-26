package cloud

import (
	"encoding/json"
	"sort"
	"strconv"
	"time"

	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/lib/netext"
	"github.com/loadimpact/k6/stats"
)

// Timestamp is used for sending times encoded as microsecond UNIX timestamps to the cloud servers
type Timestamp time.Time

// MarshalJSON encodes the microsecond UNIX timestamps as strings because JavaScripts doesn't have actual integers and tends to round big numbers
func (ct Timestamp) MarshalJSON() ([]byte, error) {
	return []byte(`"` + strconv.FormatInt(time.Time(ct).UnixNano()/1000, 10) + `"`), nil
}

// UnmarshalJSON decodes the string-enclosed microsecond timestamp back into the proper time.Time alias
func (ct *Timestamp) UnmarshalJSON(p []byte) error {
	var s string
	if err := json.Unmarshal(p, &s); err != nil {
		return err
	}
	microSecs, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	*ct = Timestamp(time.Unix(microSecs/1000000, (microSecs%1000000)*1000))
	return nil
}

// Sample is the generic struct that contains all types of data that we send to the cloud.
type Sample struct {
	Type   string      `json:"type"`
	Metric string      `json:"metric"`
	Data   interface{} `json:"data"`
}

// SampleDataSingle is used in all simple un-aggregated single-value samples.
type SampleDataSingle struct {
	Time  Timestamp         `json:"time"`
	Type  stats.MetricType  `json:"type"`
	Tags  *stats.SampleTags `json:"tags,omitempty"`
	Value float64           `json:"value"`
}

// SampleDataMap is used by samples that contain multiple values, currently
// that's only iteration metrics (`iter_li_all`) and unaggregated HTTP
// requests (`http_req_li_all`).
type SampleDataMap struct {
	Time   Timestamp          `json:"time"`
	Type   stats.MetricType   `json:"type"`
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
			Time: Timestamp(trail.GetTime()),
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

// SampleDataAggregatedHTTPReqs is used in aggregated samples for HTTP requests.
type SampleDataAggregatedHTTPReqs struct {
	Time   Timestamp         `json:"time"`
	Type   string            `json:"type"`
	Count  uint64            `json:"count"`
	Tags   *stats.SampleTags `json:"tags,omitempty"`
	Values struct {
		Duration       AggregatedMetric `json:"http_req_duration"`
		Blocked        AggregatedMetric `json:"http_req_blocked"`
		Connecting     AggregatedMetric `json:"http_req_connecting"`
		TLSHandshaking AggregatedMetric `json:"http_req_tls_handshaking"`
		Sending        AggregatedMetric `json:"http_req_sending"`
		Waiting        AggregatedMetric `json:"http_req_waiting"`
		Receiving      AggregatedMetric `json:"http_req_receiving"`
	} `json:"values"`
}

// Add updates all agregated values with the supplied trail data
func (sdagg *SampleDataAggregatedHTTPReqs) Add(trail *netext.Trail) {
	sdagg.Count++
	sdagg.Values.Duration.Add(trail.Duration)
	sdagg.Values.Blocked.Add(trail.Blocked)
	sdagg.Values.Connecting.Add(trail.Connecting)
	sdagg.Values.TLSHandshaking.Add(trail.TLSHandshaking)
	sdagg.Values.Sending.Add(trail.Sending)
	sdagg.Values.Waiting.Add(trail.Waiting)
	sdagg.Values.Receiving.Add(trail.Receiving)
}

// CalcAverages calculates and sets all `Avg` properties in the `Values` struct
func (sdagg *SampleDataAggregatedHTTPReqs) CalcAverages() {
	count := float64(sdagg.Count)
	sdagg.Values.Duration.Calc(count)
	sdagg.Values.Blocked.Calc(count)
	sdagg.Values.Connecting.Calc(count)
	sdagg.Values.TLSHandshaking.Calc(count)
	sdagg.Values.Sending.Calc(count)
	sdagg.Values.Waiting.Calc(count)
	sdagg.Values.Receiving.Calc(count)
}

// AggregatedMetric is used to store aggregated information for a
// particular metric in an SampleDataAggregatedMap.
type AggregatedMetric struct {
	// Used by Add() to keep working state
	minD time.Duration
	maxD time.Duration
	sumD time.Duration
	// Updated by Calc() and used in the JSON output
	Min float64 `json:"min"`
	Max float64 `json:"max"`
	Avg float64 `json:"avg"`
}

// Add the new duration to the internal sum and update Min and Max if necessary
func (am *AggregatedMetric) Add(t time.Duration) {
	if am.sumD == 0 || am.minD > t {
		am.minD = t
	}
	if am.maxD < t {
		am.maxD = t
	}
	am.sumD += t
}

// Calc populates the float fields for min and max and calulates the average value
func (am *AggregatedMetric) Calc(count float64) {
	am.Min = stats.D(am.minD)
	am.Max = stats.D(am.maxD)
	am.Avg = stats.D(am.sumD) / count
}

type aggregationBucket map[*stats.SampleTags][]*netext.Trail

type durations []time.Duration

func (d durations) Len() int           { return len(d) }
func (d durations) Swap(i, j int)      { d[i], d[j] = d[j], d[i] }
func (d durations) Less(i, j int) bool { return d[i] < d[j] }
func (d durations) GetNormalBounds(iqrCoef float64) (min, max time.Duration) {
	l := len(d)
	if l == 0 {
		return
	}

	sort.Sort(d)
	var q1, q3 time.Duration
	if l%4 == 0 {
		q1 = d[l/4]
		q3 = d[(l/4)*3]
	} else {
		q1 = (d[l/4] + d[(l/4)+1]) / 2
		q3 = (d[(l/4)*3] + d[(l/4)*3+1]) / 2
	}

	iqr := float64(q3 - q1)
	min = q1 - time.Duration(iqrCoef*iqr)
	max = q3 + time.Duration(iqrCoef*iqr)
	return
}
