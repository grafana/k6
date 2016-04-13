package aggregate

import (
	"time"
)

type DurationStat struct {
	Min, Max, Avg, Med time.Duration

	// TODO: Implement a rolling average/median algorithm instead.
	values []time.Duration
}

func (s *DurationStat) Ingest(d time.Duration) {
	if d < s.Min || s.Min == time.Duration(0) {
		s.Min = d
	}
	if d > s.Max {
		s.Max = d
	}
	s.values = append(s.values, d)
}

func (s *DurationStat) End() {
	sum := time.Duration(0)
	for _, d := range s.values {
		sum += d
	}
	s.Avg = sum / time.Duration(len(s.values))
	s.Med = s.values[len(s.values)/2]
}
