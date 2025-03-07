package expv2

import (
	"sort"

	"go.k6.io/k6/internal/ds/histogram"
	"go.k6.io/k6/internal/output/cloud/expv2/pbcloud"
)

// histogramAsProto converts the histogram into the equivalent Protobuf version.
func histogramAsProto(h *histogram.Hdr, time int64) *pbcloud.TrendHdrValue {
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
