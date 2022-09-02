package remotewrite

import (
	"math"
	"sort"
	"time"

	"go.k6.io/k6/metrics"
)

// The following functions are an attempt to add ad-hoc optimization to TrendSink,
// and are a partial copy-paste from k6/metrics.
// TODO: re-write & refactor this once metrics refactoring progresses in k6.

type trendSink struct {
	Values   []float64
	Count    uint64
	Min, Max float64
	Sum, Avg float64
	Med      float64
}

func (t *trendSink) Add(s metrics.Sample) {
	// insert into sorted array instead of sorting anew on each addition
	index := sort.Search(len(t.Values), func(i int) bool {
		return t.Values[i] > s.Value
	})
	t.Values = append(t.Values, 0)
	copy(t.Values[index+1:], t.Values[index:])
	t.Values[index] = s.Value

	t.Count += 1
	t.Sum += s.Value
	t.Avg = t.Sum / float64(t.Count)

	if s.Value > t.Max {
		t.Max = s.Value
	}
	if s.Value < t.Min || t.Count == 1 {
		t.Min = s.Value
	}

	if (t.Count & 0x01) == 0 {
		t.Med = (t.Values[(t.Count/2)-1] + t.Values[(t.Count/2)]) / 2
	} else {
		t.Med = t.Values[t.Count/2]
	}
}

func (t *trendSink) P(pct float64) float64 {
	switch t.Count {
	case 0:
		return 0
	case 1:
		return t.Values[0]
	default:
		// If percentile falls on a value in Values slice, we return that value.
		// If percentile does not fall on a value in Values slice, we calculate (linear interpolation)
		// the value that would fall at percentile, given the values above and below that percentile.
		i := pct * (float64(t.Count) - 1.0)
		j := t.Values[int(math.Floor(i))]
		k := t.Values[int(math.Ceil(i))]
		f := i - math.Floor(i)
		return j + (k-j)*f
	}
}

func (t *trendSink) Calc() {
	// added just for implementing the k6 metrics.Sink interface
	// the values are already re-synced for every new addition
}

func (t *trendSink) Format(time.Duration) map[string]float64 {
	return map[string]float64{
		"min":   t.Min,
		"max":   t.Max,
		"avg":   t.Avg,
		"med":   t.Med,
		"p(90)": t.P(0.90),
		"p(95)": t.P(0.95),
	}
}
