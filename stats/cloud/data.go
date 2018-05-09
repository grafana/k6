/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2018 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package cloud

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/lib/netext"
	"github.com/loadimpact/k6/stats"
)

const DataTypeSingle = "Point"
const DataTypeMap = "Points"
const DataTypeAggregatedHTTPReqs = "AggregatedPoints"

// Timestamp is used for sending times encoded as microsecond UNIX timestamps to the cloud servers
type Timestamp time.Time

// Equal will return true if the difference between the timestamps is less than 1 microsecond
func (ct Timestamp) Equal(other Timestamp) bool {
	diff := time.Time(ct).Sub(time.Time(other))
	return diff > -time.Microsecond && diff < time.Microsecond
}

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
		return fmt.Errorf("Unknown sample type '%s'", tmpSample.Type)
	}

	if err := json.Unmarshal(tmpSample.Data, &s.Data); err != nil {
		return err
	}

	*ct = s
	return nil
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
		Type:   DataTypeMap,
		Metric: "http_req_li_all",
		Data: &SampleDataMap{
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

// This is currently used only for benchmark comparisons and tests.
func (d durations) SortGetNormalBounds(radius, iqrLowerCoef, iqrUpperCoef float64) (min, max time.Duration) {
	if len(d) == 0 {
		return
	}
	sort.Sort(d)
	last := float64(len(d) - 1)
	radius = math.Min(0.5, radius)
	q1 := d[int(last*(0.5-radius))]
	q3 := d[int(last*(0.5+radius))]
	iqr := float64(q3 - q1)
	min = q1 - time.Duration(iqrLowerCoef*iqr)
	max = q3 + time.Duration(iqrUpperCoef*iqr)
	return
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
func (d durations) quickSelect(k int) time.Duration {
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

	q1 := d.quickSelect(int(last * (0.5 - radius)))
	q3 := d.quickSelect(int(last * (0.5 + radius)))

	iqr := float64(q3 - q1)
	min = q1 - time.Duration(iqrLowerCoef*iqr)
	max = q3 + time.Duration(iqrUpperCoef*iqr)
	return
}
