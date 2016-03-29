package runner

import (
	"time"
)

type Sequencer struct {
	Metrics []Metric
}

type Stat struct {
	Min, Max, Avg, Med float64
}

type Stats struct {
	Duration Stat
}

func NewSequencer() Sequencer {
	return Sequencer{}
}

func (s *Sequencer) Add(m Metric) {
	s.Metrics = append(s.Metrics, m)
}

func (s *Sequencer) Count() int {
	return len(s.Metrics)
}

func (s *Sequencer) StatDuration() (st Stat) {
	count := s.Count()
	if count == 0 {
		return st
	}

	total := time.Duration(0)
	min := time.Duration(0)
	max := time.Duration(0)
	for i := 0; i < count; i++ {
		m := s.Metrics[i]
		total += m.Duration
		if m.Duration < min || min == time.Duration(0) {
			min = m.Duration
		}
		if m.Duration > max {
			max = m.Duration
		}
	}

	avg := time.Duration(total.Nanoseconds() / int64(count))
	med := s.Metrics[len(s.Metrics)/2].Duration

	return Stat{
		Min: min.Seconds(),
		Max: max.Seconds(),
		Avg: avg.Seconds(),
		Med: med.Seconds(),
	}
}

func (s *Sequencer) Stats() (st Stats) {
	st.Duration = s.StatDuration()
	return st
}
