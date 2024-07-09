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

import "go.opentelemetry.io/otel/attribute"

// overflowSet is the attribute set used to record a measurement when adding
// another distinct attribute set to the aggregate would exceed the aggregate
// limit.
var overflowSet = attribute.NewSet(attribute.Bool("otel.metric.overflow", true))

// limiter limits aggregate values.
type limiter[V any] struct {
	// aggLimit is the maximum number of metric streams that can be aggregated.
	//
	// Any metric stream with attributes distinct from any set already
	// aggregated once the aggLimit will be meet will instead be aggregated
	// into an "overflow" metric stream. That stream will only contain the
	// "otel.metric.overflow"=true attribute.
	aggLimit int
}

// newLimiter returns a new Limiter with the provided aggregation limit.
func newLimiter[V any](aggregation int) limiter[V] {
	return limiter[V]{aggLimit: aggregation}
}

// Attributes checks if adding a measurement for attrs will exceed the
// aggregation cardinality limit for the existing measurements. If it will,
// overflowSet is returned. Otherwise, if it will not exceed the limit, or the
// limit is not set (limit <= 0), attr is returned.
func (l limiter[V]) Attributes(attrs attribute.Set, measurements map[attribute.Set]V) attribute.Set {
	if l.aggLimit > 0 {
		_, exists := measurements[attrs]
		if !exists && len(measurements) >= l.aggLimit-1 {
			return overflowSet
		}
	}

	return attrs
}
