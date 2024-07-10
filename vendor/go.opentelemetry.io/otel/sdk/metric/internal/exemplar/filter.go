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
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// SampledFilter returns a [Reservoir] wrapping r that will only offer measurements
// to r if the passed context associated with the measurement contains a sampled
// [go.opentelemetry.io/otel/trace.SpanContext].
func SampledFilter[N int64 | float64](r Reservoir[N]) Reservoir[N] {
	return filtered[N]{Reservoir: r}
}

type filtered[N int64 | float64] struct {
	Reservoir[N]
}

func (f filtered[N]) Offer(ctx context.Context, t time.Time, n N, a []attribute.KeyValue) {
	if trace.SpanContextFromContext(ctx).IsSampled() {
		f.Reservoir.Offer(ctx, t, n, a)
	}
}
