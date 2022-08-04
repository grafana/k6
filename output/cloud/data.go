package cloud

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"go.k6.io/k6/lib/netext/httpext"
	"go.k6.io/k6/metrics"
)

// DataType constants
const (
	DataTypeSingle             = "Point"
	DataTypeMap                = "Points"
	DataTypeAggregatedHTTPReqs = "AggregatedPoints"
)

//go:generate easyjson -pkg -no_std_marshalers -gen_build_flags -mod=mod .

func toMicroSecond(t time.Time) int64 {
	return t.UnixNano() / 1000
}

// Sample is the generic struct that contains all types of data that we send to the cloud.
//easyjson:json
type Sample struct {
	Type   string      `json:"type"`
	Metric string      `json:"metric"`
	Data   interface{} `json:"data"`
}

// UnmarshalJSON decodes the Data into the corresponding struct
func (ct *Sample) UnmarshalJSON(p []byte) error {
	var tmpSample struct {
		Type   string          `json:"type"`
		Metric string          `json:"metric"`
		Data   json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(p, &tmpSample); err != nil {
		return err
	}
	s := Sample{
		Type:   tmpSample.Type,
		Metric: tmpSample.Metric,
	}

	switch tmpSample.Type {
	case DataTypeSingle:
		s.Data = new(SampleDataSingle)
	case DataTypeMap:
		s.Data = new(SampleDataMap)
	case DataTypeAggregatedHTTPReqs:
		s.Data = new(SampleDataAggregatedHTTPReqs)
	default:
		return fmt.Errorf("unknown sample type '%s'", tmpSample.Type)
	}

	if err := json.Unmarshal(tmpSample.Data, &s.Data); err != nil {
		return err
	}

	*ct = s
	return nil
}

// SampleDataSingle is used in all simple un-aggregated single-value samples.
//easyjson:json
type SampleDataSingle struct {
	Time  int64               `json:"time,string"`
	Type  metrics.MetricType  `json:"type"`
	Tags  *metrics.SampleTags `json:"tags,omitempty"`
	Value float64             `json:"value"`
}

// SampleDataMap is used by samples that contain multiple values, currently
// that's only iteration metrics (`iter_li_all`) and unaggregated HTTP
// requests (`http_req_li_all`).
//easyjson:json
type SampleDataMap struct {
	Time   int64               `json:"time,string"`
	Type   metrics.MetricType  `json:"type"`
	Tags   *metrics.SampleTags `json:"tags,omitempty"`
	Values map[string]float64  `json:"values,omitempty"`
}

// NewSampleFromTrail just creates a ready-to-send Sample instance
// directly from a httpext.Trail.
func NewSampleFromTrail(trail *httpext.Trail) *Sample {
	length := 8
	if trail.Failed.Valid {
		length++
	}

	values := make(map[string]float64, length)
	values[metrics.HTTPReqsName] = 1
	values[metrics.HTTPReqDurationName] = metrics.D(trail.Duration)
	values[metrics.HTTPReqBlockedName] = metrics.D(trail.Blocked)
	values[metrics.HTTPReqConnectingName] = metrics.D(trail.Connecting)
	values[metrics.HTTPReqTLSHandshakingName] = metrics.D(trail.TLSHandshaking)
	values[metrics.HTTPReqSendingName] = metrics.D(trail.Sending)
	values[metrics.HTTPReqWaitingName] = metrics.D(trail.Waiting)
	values[metrics.HTTPReqReceivingName] = metrics.D(trail.Receiving)
	if trail.Failed.Valid { // this is done so the adding of 1 map element doesn't reexpand the map as this is a hotpath
		values[metrics.HTTPReqFailedName] = metrics.B(trail.Failed.Bool)
	}
	return &Sample{
		Type:   DataTypeMap,
		Metric: "http_req_li_all",
		Data: &SampleDataMap{
			Time:   toMicroSecond(trail.GetTime()),
			Tags:   trail.GetTags(),
			Values: values,
		},
	}
}

// SampleDataAggregatedHTTPReqs is used in aggregated samples for HTTP requests.
//easyjson:json
type SampleDataAggregatedHTTPReqs struct {
	Time   int64               `json:"time,string"`
	Type   string              `json:"type"`
	Count  uint64              `json:"count"`
	Tags   *metrics.SampleTags `json:"tags,omitempty"`
	Values struct {
		Duration       AggregatedMetric `json:"http_req_duration"`
		Blocked        AggregatedMetric `json:"http_req_blocked"`
		Connecting     AggregatedMetric `json:"http_req_connecting"`
		TLSHandshaking AggregatedMetric `json:"http_req_tls_handshaking"`
		Sending        AggregatedMetric `json:"http_req_sending"`
		Waiting        AggregatedMetric `json:"http_req_waiting"`
		Receiving      AggregatedMetric `json:"http_req_receiving"`
		Failed         AggregatedRate   `json:"http_req_failed,omitempty"`
	} `json:"values"`
}

// Add updates all agregated values with the supplied trail data
func (sdagg *SampleDataAggregatedHTTPReqs) Add(trail *httpext.Trail) {
	sdagg.Count++
	sdagg.Values.Duration.Add(trail.Duration)
	sdagg.Values.Blocked.Add(trail.Blocked)
	sdagg.Values.Connecting.Add(trail.Connecting)
	sdagg.Values.TLSHandshaking.Add(trail.TLSHandshaking)
	sdagg.Values.Sending.Add(trail.Sending)
	sdagg.Values.Waiting.Add(trail.Waiting)
	sdagg.Values.Receiving.Add(trail.Receiving)
	if trail.Failed.Valid {
		sdagg.Values.Failed.Add(trail.Failed.Bool)
	}
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

// AggregatedRate is an aggregation of a Rate metric
type AggregatedRate struct {
	Count   float64 `json:"count"`
	NzCount float64 `json:"nz_count"`
}

// Add a boolean to the aggregated rate
func (ar *AggregatedRate) Add(b bool) {
	ar.Count++
	if b {
		ar.NzCount++
	}
}

// IsDefined implements easyjson.Optional
func (ar AggregatedRate) IsDefined() bool {
	return ar.Count != 0
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

// Calc populates the float fields for min and max and calculates the average value
func (am *AggregatedMetric) Calc(count float64) {
	am.Min = metrics.D(am.minD)
	am.Max = metrics.D(am.maxD)
	am.Avg = metrics.D(am.sumD) / count
}

type aggregationBucket map[*metrics.SampleTags][]*httpext.Trail

type durations []time.Duration

func (d durations) Len() int           { return len(d) }
func (d durations) Swap(i, j int)      { d[i], d[j] = d[j], d[i] }
func (d durations) Less(i, j int) bool { return d[i] < d[j] }

// Used when there are fewer samples in the bucket (so we can interpolate)
// and for benchmark comparisons and verification of the quickselect
// algorithm (it should return exactly the same results if interpolation isn't used).
func (d durations) SortGetNormalBounds(
	radius, iqrLowerCoef, iqrUpperCoef float64, interpolate bool,
) (min, max time.Duration) {
	if len(d) == 0 {
		return
	}
	sort.Sort(d)
	last := float64(len(d) - 1)

	getValue := func(percentile float64) time.Duration {
		i := percentile * last
		// If interpolation is not enabled, we round the resulting slice position
		// and return the value there.
		if !interpolate {
			return d[int(math.Round(i))]
		}

		// Otherwise, we calculate (with linear interpolation) the value that
		// should fall at that percentile, given the values above and below it.
		floor := d[int(math.Floor(i))]
		ceil := d[int(math.Ceil(i))]
		posDiff := i - math.Floor(i)
		return floor + time.Duration(float64(ceil-floor)*posDiff)
	}

	// See https://en.wikipedia.org/wiki/Quartile#Outliers for details
	radius = math.Min(0.5, radius) // guard against a radius greater than 50%, see AggregationOutlierIqrRadius
	q1 := getValue(0.5 - radius)   // get Q1, the (interpolated) value at a `radius` distance before the median
	q3 := getValue(0.5 + radius)   // get Q3, the (interpolated) value at a `radius` distance after the median
	iqr := float64(q3 - q1)        // calculate the interquartile range (IQR)

	min = q1 - time.Duration(iqrLowerCoef*iqr) // lower fence, anything below this is an outlier
	max = q3 + time.Duration(iqrUpperCoef*iqr) // upper fence, anything above this is an outlier
	return min, max
}

// Reworked and translated in Go from:
// https://github.com/haifengl/smile/blob/master/math/src/main/java/smile/sort/QuickSelect.java
// Originally Copyright (c) 2010 Haifeng Li
// Licensed under the Apache License, Version 2.0
//
// This could potentially be implemented as a standalone function
// that only depends on the sort.Interface methods, but that would
// probably introduce some performance overhead because of the
// dynamic dispatch.
func (d durations) quickSelect(k int) time.Duration { //nolint:gocognit
	n := len(d)
	l := 0
	ir := n - 1

	var i, j, mid int
	for {
		if ir <= l+1 {
			if ir == l+1 && d[ir] < d[l] {
				d.Swap(l, ir)
			}
			return d[k]
		}
		mid = (l + ir) >> 1
		d.Swap(mid, l+1)
		if d[l] > d[ir] {
			d.Swap(l, ir)
		}
		if d[l+1] > d[ir] {
			d.Swap(l+1, ir)
		}
		if d[l] > d[l+1] {
			d.Swap(l, l+1)
		}
		i = l + 1
		j = ir
		for {
			for i++; d[i] < d[l+1]; i++ {
			}
			for j--; d[j] > d[l+1]; j-- {
			}
			if j < i {
				break
			}
			d.Swap(i, j)
		}
		d.Swap(l+1, j)
		if j >= k {
			ir = j - 1
		}
		if j <= k {
			l = i
		}
	}
}

// Uses Quickselect to avoid sorting the whole slice
// https://en.wikipedia.org/wiki/Quickselect
func (d durations) SelectGetNormalBounds(radius, iqrLowerCoef, iqrUpperCoef float64) (min, max time.Duration) {
	if len(d) == 0 {
		return
	}
	radius = math.Min(0.5, radius)
	last := float64(len(d) - 1)

	q1 := d.quickSelect(int(math.Round(last * (0.5 - radius))))
	q3 := d.quickSelect(int(math.Round(last * (0.5 + radius))))

	iqr := float64(q3 - q1)
	min = q1 - time.Duration(iqrLowerCoef*iqr)
	max = q3 + time.Duration(iqrUpperCoef*iqr)
	return
}

//easyjson:json
type samples []*Sample
