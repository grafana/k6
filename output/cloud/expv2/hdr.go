package expv2

import (
	"math"
	"math/bits"
	"time"

	"go.k6.io/k6/metrics"
	"go.k6.io/k6/output/cloud/expv2/pbcloud"
	"google.golang.org/protobuf/types/known/timestamppb"
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

// newHistogram creates an histogram of the provided values.
//
// TODO: after the aggregation layer probably this constructor
// doesn't make any sense in this way.
// It aggregates calling addToBucket time-to-time so
// trimzeros has to be called at the end of the process.
func newHistogram(values []float64) histogram {
	h := histogram{}
	if len(values) < 1 {
		return h
	}

	for i := 0; i < len(values); i++ {
		h.addToBucket(values[i])
	}

	h.trimzeros()
	return h
}

// addToBucket increments the counter of the bucket
// releated to the provided value.
// If the value is lower or higher than the trackable limits
// then it is counted into specific buckets.
// All the stats are also updated accordingly.
//
// TODO: add the test case in units doing
// addToBucket + trimzeros + addToBucket
// so calling addToBucket after trimzeros was called.
// We don't expect to have this case but the current API
// would support it so we need to make sure it works or refactor.
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
	blen := uint32(len(h.Buckets))
	if blen == 0 {
		h.FirstNotZeroBucket = index
		h.LastNotZeroBucket = index
	} else {
		if index < h.FirstNotZeroBucket {
			h.FirstNotZeroBucket = index
		}
		if index > h.LastNotZeroBucket {
			h.LastNotZeroBucket = index
		}
	}

	if index >= blen {
		h.grow(index)
	}
	h.Buckets[index]++
}

// grow expands the buckets slice
// with zeros up to the required index
func (h *histogram) grow(index uint32) {
	i := int(index)
	if len(h.Buckets)-1 > i {
		panic("buckets is already bigger than requested index")
	}
	if cap(h.Buckets) > i {
		// See https://go.dev/ref/spec#Slice_expressions
		// "For slices, the upper index bound is
		// the slice capacity cap(a) rather than the length"
		h.Buckets = h.Buckets[:index+1]
	} else {
		length := i + 1
		// let's make two times larger of
		// the current request
		newBuckets := make([]uint32, length, (len(h.Buckets)+length)*2)
		copy(newBuckets, h.Buckets)
		h.Buckets = newBuckets
	}
}

// trimzeros removes all buckets that have a zero value
// from the begin and from the end until
// the first not zero bucket.
func (h *histogram) trimzeros() {
	if h.Count < 1 || len(h.Buckets) < 1 {
		return
	}

	// all the counters are set to zero, we can remove all
	if h.FirstNotZeroBucket == 0 && h.LastNotZeroBucket == 0 {
		h.Buckets = []uint32{}
		return
	}

	h.Buckets = h.Buckets[h.FirstNotZeroBucket : h.LastNotZeroBucket+1]
}

// histogramAsProto converts the histogram into the equivalent Protobuf version.
func histogramAsProto(h *histogram, time time.Time) *pbcloud.TrendHdrValue {
	hval := &pbcloud.TrendHdrValue{
		Time:              timestamppb.New(time),
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
	// the lowest trackable value is zero
	// negative number are not expected
	if val < 0 {
		return 0
	}

	upscaled := uint32(math.Ceil(val))

	// k is a power of 2 closest to 10^precision_points
	// At the moment the precision_points is a fixed value set to 2.
	//
	// i.e 2^7  = 128  ~  100 = 10^2
	//     2^10 = 1024 ~ 1000 = 10^3
	// f(x) = 3*x + 1 - empiric formula that works for us
	// since f(2)=7 and f(3)=10
	const k = uint32(7)

	// 256 = 1 << (k+1)
	if upscaled < 256 {
		return upscaled
	}

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

func (h *histogram) IsEmpty() bool {
	return h.Count == 0
}

func (h *histogram) Add(s metrics.Sample) {
	h.addToBucket(s.Value)
}

func (h *histogram) Format(time.Duration) map[string]float64 {
	panic("output/cloud/expv2/histogram.Format is not expected to be called")
}
