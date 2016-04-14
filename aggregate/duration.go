package aggregate

import (
	"time"
)

type DurationStat struct {
	Min, Max, Avg, Med time.Duration

	// TODO: Implement a rolling average/median algorithm instead.
	Values []time.Duration
}

func (s *DurationStat) Ingest(d time.Duration) {
	if d < s.Min || s.Min == time.Duration(0) {
		s.Min = d
	}
	if d > s.Max {
		s.Max = d
	}
	s.Values = append(s.Values, d)
}

func (s *DurationStat) End() {
	sum := time.Duration(0)
	for _, d := range s.Values {
		sum += d
	}
	s.Avg = sum / time.Duration(len(s.Values))
	s.Med = s.Values[len(s.Values)/2]
}
