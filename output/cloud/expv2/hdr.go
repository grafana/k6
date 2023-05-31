package expv2

import (
	"math"
	"math/bits"

	"go.k6.io/k6/output/cloud/expv2/pbcloud"
)

const (
	// lowestTrackable represents the minimum value that the histogram tracks.
	// Essentially, it excludes negative numbers.
	// Most of metrics tracked by histograms are durations
	// where we don't expect negative numbers.
	//
	// In the future, we may expand and include them,
	// probably after https://github.com/grafana/k6/issues/763.
	lowestTrackable = 0

	// highestTrackable represents the maximum
	// value that the histogram is able to track with high accuracy (0.1% of error).
	// It should be a high enough
	// and rationale value for the k6 context; 2^30 = 1_073_741_824
	highestTrackable = 1 << 30
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
	// It does not include the first and the last absolute bucket,
	// because they contain exception cases
	// and they requires to be tracked in a dedicated way.
	//
	// It is expected to start and end with a non-zero bucket,
	// in this way we can avoid extra allocation for not significant buckets.
	// All the zero buckets in between are preserved.
	Buckets []uint32

	// ExtraLowBucket counts occurrences of observed values smaller
	// than the minimum trackable value.
	ExtraLowBucket uint32

	// ExtraLowBucket counts occurrences of observed values bigger
	// than the maximum trackable value.
	ExtraHighBucket uint32

	// FirstNotZeroBucket represents the index of the first bucket
	// with a significant counter in the Buckets slice (a not zero value).
	// In this way, all the buckets before can be omitted.
	FirstNotZeroBucket uint32

	// LastNotZeroBucket represents the index of the last bucket
	// with a significant counter in the Buckets slice (a not zero value).
	// In this way, all the buckets after can be omitted.
	LastNotZeroBucket uint32

	// Max is the absolute maximum observed value.
	Max float64

	// Min is the absolute minimum observed value.
	Min float64

	// Sum is the sum of all observed values.
	Sum float64

	// Count is counts the amount of observed values.
	Count uint32
}

// addToBucket increments the counter of the bucket of the provided value.
// If the value is lower or higher than the trackable limits
// then it is counted into specific buckets. All the stats are also updated accordingly.
func (h *histogram) addToBucket(v float64) {
	if h.Count == 0 {
		h.Max, h.Min = v, v
	} else {
		if v > h.Max {
			h.Max = v
		}
		if v < h.Min {
			h.Min = v
		}
	}

	h.Count++
	h.Sum += v

	if v > highestTrackable {
		h.ExtraHighBucket++
		return
	}
	if v < lowestTrackable {
		h.ExtraLowBucket++
		return
	}

	index := resolveBucketIndex(v)

	// they grow the current Buckets slice if there isn't enough capacity.
	//
	// An example with growRight:
	// With Buckets [4, 1] and index equals to 5
	// then we expect a slice like [4,1,0,0,0,0]
	// then the counter at 5th position will be incremented
	// generating the final slice [4,1,0,0,0,1]
	switch {
	case len(h.Buckets) == 0:
		h.init(index)
	case index < h.FirstNotZeroBucket:
		h.prependBuckets(index)
	case index > h.LastNotZeroBucket:
		h.appendBuckets(index)
	default:
		h.Buckets[index-h.FirstNotZeroBucket]++
	}
}

func (h *histogram) init(index uint32) {
	h.FirstNotZeroBucket = index
	h.LastNotZeroBucket = index
	h.Buckets = make([]uint32, 1, 32)
	h.Buckets[0] = 1
}

// prependBuckets expands the buckets slice with zeros up to the required index,
// then it increments the required bucket.
func (h *histogram) prependBuckets(index uint32) {
	if h.FirstNotZeroBucket <= index {
		panic("buckets is already contains the requested index")
	}

	newLen := (h.FirstNotZeroBucket - index) + uint32(len(h.Buckets))

	// TODO: we may consider to swap by sub-groups
	// e.g  [4, 1] => [4, 1, 0, 0] => [0, 0, 4, 1]
	// It requires a benchmark if it is better than just copy it.

	newBuckets := make([]uint32, newLen)
	copy(newBuckets[h.FirstNotZeroBucket-index:], h.Buckets)
	h.Buckets = newBuckets

	// Update the stats
	h.Buckets[0] = 1
	h.FirstNotZeroBucket = index
}

// appendBuckets expands the buckets slice with zeros buckets till the required index,
// then it increments the required bucket.
// If the slice has enough capacity then it reuses it without allocate.
func (h *histogram) appendBuckets(index uint32) {
	if h.LastNotZeroBucket >= index {
		panic("buckets is already bigger than requested index")
	}

	newLen := index - h.FirstNotZeroBucket + 1

	if uint32(cap(h.Buckets)) > newLen {
		// See https://go.dev/ref/spec#Slice_expressions
		// "For slices, the upper index bound is
		// the slice capacity cap(a) rather than the length"
		h.Buckets = h.Buckets[:newLen]
	} else {
		newBuckets := make([]uint32, newLen)
		copy(newBuckets, h.Buckets)
		h.Buckets = newBuckets
	}

	// Update the stats
	h.Buckets[len(h.Buckets)-1] = 1
	h.LastNotZeroBucket = index
}

// histogramAsProto converts the histogram into the equivalent Protobuf version.
func histogramAsProto(h *histogram, time int64) *pbcloud.TrendHdrValue {
	hval := &pbcloud.TrendHdrValue{
		Time:              timestampAsProto(time),
		MinResolution:     1.0,
		SignificantDigits: 2,
		LowerCounterIndex: h.FirstNotZeroBucket,
		MinValue:          h.Min,
		MaxValue:          h.Max,
		Sum:               h.Sum,
		Count:             h.Count,
		Counters:          h.Buckets,
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
	upscaled := uint32(math.Ceil(val))

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
	const k = uint32(7)

	// 256 = 1 << (k+1)
	if upscaled < 256 {
		return upscaled
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
	nkdiff := uint32(bits.Len32(upscaled>>k) - 1) // msb index
	return (nkdiff << k) + (upscaled >> nkdiff)
}

func (h *histogram) Add(v float64) {
	h.addToBucket(v)
}
