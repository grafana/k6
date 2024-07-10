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

package metric // import "go.opentelemetry.io/otel/sdk/metric"

import (
	"os"
	"runtime"

	"go.opentelemetry.io/otel/sdk/metric/internal/exemplar"
	"go.opentelemetry.io/otel/sdk/metric/internal/x"
)

// reservoirFunc returns the appropriately configured exemplar reservoir
// creation func based on the passed InstrumentKind and user defined
// environment variables.
//
// Note: This will only return non-nil values when the experimental exemplar
// feature is enabled and the OTEL_METRICS_EXEMPLAR_FILTER environment variable
// is not set to always_off.
func reservoirFunc[N int64 | float64](agg Aggregation) func() exemplar.Reservoir[N] {
	if !x.Exemplars.Enabled() {
		return nil
	}

	// https://github.com/open-telemetry/opentelemetry-specification/blob/d4b241f451674e8f611bb589477680341006ad2b/specification/metrics/sdk.md#exemplar-defaults
	resF := func() func() exemplar.Reservoir[N] {
		// Explicit bucket histogram aggregation with more than 1 bucket will
		// use AlignedHistogramBucketExemplarReservoir.
		a, ok := agg.(AggregationExplicitBucketHistogram)
		if ok && len(a.Boundaries) > 0 {
			cp := make([]float64, len(a.Boundaries))
			copy(cp, a.Boundaries)
			return func() exemplar.Reservoir[N] {
				bounds := cp
				return exemplar.Histogram[N](bounds)
			}
		}

		var n int
		if a, ok := agg.(AggregationBase2ExponentialHistogram); ok {
			// Base2 Exponential Histogram Aggregation SHOULD use a
			// SimpleFixedSizeExemplarReservoir with a reservoir equal to the
			// smaller of the maximum number of buckets configured on the
			// aggregation or twenty (e.g. min(20, max_buckets)).
			n = int(a.MaxSize)
			if n > 20 {
				n = 20
			}
		} else {
			// https://github.com/open-telemetry/opentelemetry-specification/blob/e94af89e3d0c01de30127a0f423e912f6cda7bed/specification/metrics/sdk.md#simplefixedsizeexemplarreservoir
			//   This Exemplar reservoir MAY take a configuration parameter for
			//   the size of the reservoir. If no size configuration is
			//   provided, the default size MAY be the number of possible
			//   concurrent threads (e.g. number of CPUs) to help reduce
			//   contention. Otherwise, a default size of 1 SHOULD be used.
			n = runtime.NumCPU()
			if n < 1 {
				// Should never be the case, but be defensive.
				n = 1
			}
		}

		return func() exemplar.Reservoir[N] {
			return exemplar.FixedSize[N](n)
		}
	}

	// https://github.com/open-telemetry/opentelemetry-specification/blob/d4b241f451674e8f611bb589477680341006ad2b/specification/configuration/sdk-environment-variables.md#exemplar
	const filterEnvKey = "OTEL_METRICS_EXEMPLAR_FILTER"

	switch os.Getenv(filterEnvKey) {
	case "always_on":
		return resF()
	case "always_off":
		return exemplar.Drop[N]
	case "trace_based":
		fallthrough
	default:
		newR := resF()
		return func() exemplar.Reservoir[N] {
			return exemplar.SampledFilter(newR())
		}
	}
}
