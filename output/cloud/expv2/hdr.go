package expv2

import (
	"math"
	"math/bits"
	"sort"

	"go.k6.io/k6/internal/output/cloud/expv2/pbcloud"
)

const (
	// defaultMinimumResolution is the default resolution used by histogram.
	// It allows to have a higher granularity compared to the basic 1.0 value,
	// supporting floating points up to 3 digits.
	defaultMinimumResolution = .001

	// lowestTrackable represents the minimum value that the histogram tracks.
	// Essentially, it excludes negative numbers.
	// Most of metrics tracked by histograms are durations
	// where we don't expect negative numbers.
	//
	// In the future, we may expand and include them,
	// probably after https://github.com/grafana/k6/issues/763.
	lowestTrackable = 0
)

// histogram represents a distribution
// of metrics samples' values as histogram.
//
// The histogram is the representation of base-2 exponential Histogram with two layers.
// The first layer has primary buckets in the form of a power of two, and a second layer of buckets
// for each primary bucket with an equally distributed amount of buckets inside.
//
// The histogram has a series of (N * 2^m) buckets, where:
// N = a power of 2 that defines the number of primary buckets
// m = a power of 2 that defines the number of the secondary buckets
// The current version is: f(N = 25, m = 7) = 3200.
type histogram struct {
	// Buckets stores the counters for each bin of the histogram.
	// It does not include counters for the untrackable values,
	// because they contain exception cases and require to be tracked in a dedicated way.
	Buckets map[uint32]uint32

	// ExtraLowBucket counts occurrences of observed values smaller
	// than the minimum trackable value.
	ExtraLowBucket uint32

	// ExtraLowBucket counts occurrences of observed values bigger
	// than the maximum trackable value.
	ExtraHighBucket uint32

	// Max is the absolute maximum observed value.
	Max float64

	// Min is the absolute minimum observed value.
	Min float64

	// Sum is the sum of all observed values.
	Sum float64

	// Count is counts the amount of observed values.
	Count uint32

	// MinimumResolution represents resolution used by Histogram.
	// In principle, it is a multiplier factor for the tracked values.
	MinimumResolution float64
}

func newHistogram() *histogram {
	return &histogram{
		MinimumResolution: defaultMinimumResolution,
		Buckets:           make(map[uint32]uint32),
		Max:               -math.MaxFloat64,
		Min:               math.MaxFloat64,
	}
}

// addToBucket increments the counter of the bucket of the provided value.
// If the value is lower or higher than the trackable limits
// then it is counted into specific buckets. All the stats are also updated accordingly.
func (h *histogram) addToBucket(v float64) {
	if v > h.Max {
		h.Max = v
	}
	if v < h.Min {
		h.Min = v
	}

	h.Count++
	h.Sum += v

	v /= h.MinimumResolution

	if v < lowestTrackable {
		h.ExtraLowBucket++
		return
	}
	if v > math.MaxInt64 {
		h.ExtraHighBucket++
		return
	}

	h.Buckets[resolveBucketIndex(v)]++
}

// histogramAsProto converts the histogram into the equivalent Protobuf version.
func histogramAsProto(h *histogram, time int64) *pbcloud.TrendHdrValue {
	var (
		indexes  []uint32
		counters []uint32
		spans    []*pbcloud.BucketSpan
	)

	// allocate only if at least one item is available, in the case of only
	// untrackable values, then Indexes and Buckets are expected to be empty.
	if len(h.Buckets) > 0 {
		indexes = make([]uint32, 0, len(h.Buckets))
		for index := range h.Buckets {
			indexes = append(indexes, index)
		}
		sort.Slice(indexes, func(i, j int) bool {
			return indexes[i] < indexes[j]
		})

		// init the counters
		counters = make([]uint32, 1, len(h.Buckets))
		counters[0] = h.Buckets[indexes[0]]
		// open the first span
		spans = append(spans, &pbcloud.BucketSpan{Offset: indexes[0], Length: 1})
	}

	for i := 1; i < len(indexes); i++ {
		counters = append(counters, h.Buckets[indexes[i]])

		// if the current and the previous indexes are not consecutive
		// consider as closed the current on-going span and start a new one.
		if zerosBetween := indexes[i] - indexes[i-1] - 1; zerosBetween > 0 {
			spans = append(spans, &pbcloud.BucketSpan{Offset: zerosBetween, Length: 1})
			continue
		}

		spans[len(spans)-1].Length++
	}

	hval := &pbcloud.TrendHdrValue{
		Time:          timestampAsProto(time),
		MinValue:      h.Min,
		MaxValue:      h.Max,
		Sum:           h.Sum,
		Count:         h.Count,
		Counters:      counters,
		Spans:         spans,
		MinResolution: h.MinimumResolution,
	}
	if h.ExtraLowBucket > 0 {
		hval.ExtraLowValuesCounter = &h.ExtraLowBucket
	}
	if h.ExtraHighBucket > 0 {
		hval.ExtraHighValuesCounter = &h.ExtraHighBucket
	}
	return hval
}

// resolveBucketIndex returns the index
// of the bucket in the histogram for the provided value.
func resolveBucketIndex(val float64) uint32 {
	if val < lowestTrackable {
		return 0
	}

	// We upscale to the next integer to ensure that each sample falls
	// within a specific bucket, even when the value is fractional.
	// This avoids under-representing the distribution in the histogram.
	upscaled := uint64(math.Ceil(val))

	// In histograms, bucket boundaries are usually defined as multiples of powers of 2,
	// allowing for efficient computation of bucket indexes.
	//
	// We define k=7 in our case, because it allows for sufficient granularity in the
	// distribution (2^7=128 primary buckets of which each can be further
	// subdivided if needed).
	//
	// k is the constant balancing factor between granularity and
	// computational efficiency.
	//
	// In our case:
	// i.e 2^7  = 128  ~  100 = 10^2
	//     2^10 = 1024 ~ 1000 = 10^3
	// f(x) = 3*x + 1 - empiric formula that works for us
	// since f(2)=7 and f(3)=10
	const k = uint64(7)

	// 256 = 1 << (k+1)
	if upscaled < 256 {
		return uint32(upscaled)
	}

	// `nkdiff` helps us find the right bucket for `upscaled`. It does so by determining the
	// index for the "major" bucket (a set of values within a power of two range) and then
	// the "sub" bucket within that major bucket. This system provides us with a fine level
	// of granularity within a computationally efficient bucketing system. The result is a
	// histogram that provides a detailed representation of the distribution of values.
	//
	// Here we use some math to get simple formula
	// derivation:
	// let u = upscaled
	// let n = msb(u) - most significant digit position
	// i.e. n = floor(log(u, 2))
	//   major_bucket_index = n - k + 1
	//   sub_bucket_index = u>>(n - k) - (1<<k)
	//   bucket = major_bucket_index << k + sub_bucket_index =
	//          = (n-k+1)<<k + u>>(n-k) - (1<<k) =
	//          = (n-k)<<k + u>>(n-k)
	//
	nkdiff := uint64(bits.Len64(upscaled>>k)) - 1 //nolint:gosec // msb index

	// We cast safely downscaling because we don't expect we may hit the uint32 limit
	// with the bucket index. The bucket represented from the index as MaxUint32
	// would be a very huge number bigger than the trackable limits.
	return uint32((nkdiff << k) + (upscaled >> nkdiff)) //nolint:gosec
}

// Add implements the metricValue interface.
func (h *histogram) Add(v float64) {
	h.addToBucket(v)
}
