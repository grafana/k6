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

package exemplar // import "go.opentelemetry.io/otel/sdk/metric/internal/exemplar"

import (
	"context"
	"sort"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

// Histogram returns a [Reservoir] that samples the last measurement that falls
// within a histogram bucket. The histogram bucket upper-boundaries are define
// by bounds.
//
// The passed bounds will be sorted by this function.
func Histogram[N int64 | float64](bounds []float64) Reservoir[N] {
	sort.Float64s(bounds)
	return &histRes[N]{
		bounds:  bounds,
		storage: newStorage[N](len(bounds) + 1),
	}
}

type histRes[N int64 | float64] struct {
	*storage[N]

	// bounds are bucket bounds in ascending order.
	bounds []float64
}

func (r *histRes[N]) Offer(ctx context.Context, t time.Time, n N, a []attribute.KeyValue) {
	r.store[sort.SearchFloat64s(r.bounds, float64(n))] = newMeasurement(ctx, t, n, a)
}
