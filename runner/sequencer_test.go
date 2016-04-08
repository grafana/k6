package runner

import (
	"testing"
	"time"
)

func TestAddCount(t *testing.T) {
	seq := NewSequencer()
	if seq.Count() != 0 {
		t.Error("Why does a blank sequencer have metrics!?")
	}
	seq.Add(Metric{Duration: time.Duration(10) * time.Second})
	if seq.Count() != 1 {
		t.Error("Add() didn't seem to add anything")
	}
}

func TestStatDuration(t *testing.T) {
	seq := NewSequencer()
	seq.Metrics = []Metric{
		Metric{Duration: time.Duration(10) * time.Second},
		Metric{Duration: time.Duration(15) * time.Second},
		Metric{Duration: time.Duration(20) * time.Second},
		Metric{Duration: time.Duration(25) * time.Second},
	}
	s := seq.StatDuration()
	if s.Avg != 17.5 {
		t.Error("Wrong average", s.Avg)
	}
	if s.Med != 20 {
		t.Error("Wrong median", s.Med)
	}
	if s.Min != 10 {
		t.Error("Wrong min", s.Min)
	}
	if s.Max != 25 {
		t.Error("Wrong max", s.Max)
	}
}

func TestStatDurationNoMetrics(t *testing.T) {
	seq := NewSequencer()
	s := seq.StatDuration()
	if s.Avg != 0 || s.Med != 0 || s.Min != 0 || s.Max != 0 {
		t.Error("Nonzero values", s)
	}
}
