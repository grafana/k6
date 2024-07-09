// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package aggregate // import "go.opentelemetry.io/otel/sdk/metric/internal/aggregate"

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/internal/exemplar"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

type sumValue[N int64 | float64] struct {
	n   N
	res exemplar.Reservoir[N]
}

// valueMap is the storage for sums.
type valueMap[N int64 | float64] struct {
	sync.Mutex
	newRes func() exemplar.Reservoir[N]
	limit  limiter[sumValue[N]]
	values map[attribute.Set]sumValue[N]
}

func newValueMap[N int64 | float64](limit int, r func() exemplar.Reservoir[N]) *valueMap[N] {
	return &valueMap[N]{
		newRes: r,
		limit:  newLimiter[sumValue[N]](limit),
		values: make(map[attribute.Set]sumValue[N]),
	}
}

func (s *valueMap[N]) measure(ctx context.Context, value N, fltrAttr attribute.Set, droppedAttr []attribute.KeyValue) {
	t := now()

	s.Lock()
	defer s.Unlock()

	attr := s.limit.Attributes(fltrAttr, s.values)
	v, ok := s.values[attr]
	if !ok {
		v.res = s.newRes()
	}

	v.n += value
	v.res.Offer(ctx, t, value, droppedAttr)

	s.values[attr] = v
}

// newSum returns an aggregator that summarizes a set of measurements as their
// arithmetic sum. Each sum is scoped by attributes and the aggregation cycle
// the measurements were made in.
func newSum[N int64 | float64](monotonic bool, limit int, r func() exemplar.Reservoir[N]) *sum[N] {
	return &sum[N]{
		valueMap:  newValueMap[N](limit, r),
		monotonic: monotonic,
		start:     now(),
	}
}

// sum summarizes a set of measurements made as their arithmetic sum.
type sum[N int64 | float64] struct {
	*valueMap[N]

	monotonic bool
	start     time.Time
}

func (s *sum[N]) delta(dest *metricdata.Aggregation) int {
	t := now()

	// If *dest is not a metricdata.Sum, memory reuse is missed. In that case,
	// use the zero-value sData and hope for better alignment next cycle.
	sData, _ := (*dest).(metricdata.Sum[N])
	sData.Temporality = metricdata.DeltaTemporality
	sData.IsMonotonic = s.monotonic

	s.Lock()
	defer s.Unlock()

	n := len(s.values)
	dPts := reset(sData.DataPoints, n, n)

	var i int
	for attr, val := range s.values {
		dPts[i].Attributes = attr
		dPts[i].StartTime = s.start
		dPts[i].Time = t
		dPts[i].Value = val.n
		val.res.Collect(&dPts[i].Exemplars)
		// Do not report stale values.
		delete(s.values, attr)
		i++
	}
	// The delta collection cycle resets.
	s.start = t

	sData.DataPoints = dPts
	*dest = sData

	return n
}

func (s *sum[N]) cumulative(dest *metricdata.Aggregation) int {
	t := now()

	// If *dest is not a metricdata.Sum, memory reuse is missed. In that case,
	// use the zero-value sData and hope for better alignment next cycle.
	sData, _ := (*dest).(metricdata.Sum[N])
	sData.Temporality = metricdata.CumulativeTemporality
	sData.IsMonotonic = s.monotonic

	s.Lock()
	defer s.Unlock()

	n := len(s.values)
	dPts := reset(sData.DataPoints, n, n)

	var i int
	for attr, value := range s.values {
		dPts[i].Attributes = attr
		dPts[i].StartTime = s.start
		dPts[i].Time = t
		dPts[i].Value = value.n
		value.res.Collect(&dPts[i].Exemplars)
		// TODO (#3006): This will use an unbounded amount of memory if there
		// are unbounded number of attribute sets being aggregated. Attribute
		// sets that become "stale" need to be forgotten so this will not
		// overload the system.
		i++
	}

	sData.DataPoints = dPts
	*dest = sData

	return n
}

// newPrecomputedSum returns an aggregator that summarizes a set of
// observatrions as their arithmetic sum. Each sum is scoped by attributes and
// the aggregation cycle the measurements were made in.
func newPrecomputedSum[N int64 | float64](monotonic bool, limit int, r func() exemplar.Reservoir[N]) *precomputedSum[N] {
	return &precomputedSum[N]{
		valueMap:  newValueMap[N](limit, r),
		monotonic: monotonic,
		start:     now(),
	}
}

// precomputedSum summarizes a set of observatrions as their arithmetic sum.
type precomputedSum[N int64 | float64] struct {
	*valueMap[N]

	monotonic bool
	start     time.Time

	reported map[attribute.Set]N
}

func (s *precomputedSum[N]) delta(dest *metricdata.Aggregation) int {
	t := now()
	newReported := make(map[attribute.Set]N)

	// If *dest is not a metricdata.Sum, memory reuse is missed. In that case,
	// use the zero-value sData and hope for better alignment next cycle.
	sData, _ := (*dest).(metricdata.Sum[N])
	sData.Temporality = metricdata.DeltaTemporality
	sData.IsMonotonic = s.monotonic

	s.Lock()
	defer s.Unlock()

	n := len(s.values)
	dPts := reset(sData.DataPoints, n, n)

	var i int
	for attr, value := range s.values {
		delta := value.n - s.reported[attr]

		dPts[i].Attributes = attr
		dPts[i].StartTime = s.start
		dPts[i].Time = t
		dPts[i].Value = delta
		value.res.Collect(&dPts[i].Exemplars)

		newReported[attr] = value.n
		// Unused attribute sets do not report.
		delete(s.values, attr)
		i++
	}
	// Unused attribute sets are forgotten.
	s.reported = newReported
	// The delta collection cycle resets.
	s.start = t

	sData.DataPoints = dPts
	*dest = sData

	return n
}

func (s *precomputedSum[N]) cumulative(dest *metricdata.Aggregation) int {
	t := now()

	// If *dest is not a metricdata.Sum, memory reuse is missed. In that case,
	// use the zero-value sData and hope for better alignment next cycle.
	sData, _ := (*dest).(metricdata.Sum[N])
	sData.Temporality = metricdata.CumulativeTemporality
	sData.IsMonotonic = s.monotonic

	s.Lock()
	defer s.Unlock()

	n := len(s.values)
	dPts := reset(sData.DataPoints, n, n)

	var i int
	for attr, val := range s.values {
		dPts[i].Attributes = attr
		dPts[i].StartTime = s.start
		dPts[i].Time = t
		dPts[i].Value = val.n
		val.res.Collect(&dPts[i].Exemplars)

		// Unused attribute sets do not report.
		delete(s.values, attr)
		i++
	}

	sData.DataPoints = dPts
	*dest = sData

	return n
}
