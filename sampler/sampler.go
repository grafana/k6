package sampler

import (
	"sync"
	"time"
)

const (
	DefaultIntent = iota
	TimeIntent
)

const (
	StatsType = iota
	CounterType
)

type Fields map[string]interface{}

type Entry struct {
	Metric *Metric
	Time   time.Time
	Fields map[string]interface{}
	Value  int64
}

func (e *Entry) WithField(key string, value interface{}) *Entry {
	e.Fields[key] = value
	return e
}

func (e *Entry) WithFields(fields Fields) *Entry {
	for key, value := range fields {
		e.Fields[key] = value
	}
	return e
}

func (e *Entry) Int(v int) {
	e.Value = int64(v)
	e.Metric.Write(e)
}

func (e *Entry) Int64(v int64) {
	e.Value = v
	e.Metric.Write(e)
}

func (e *Entry) Duration(d time.Duration) {
	e.Value = d.Nanoseconds()
	e.Metric.Intent = TimeIntent
	e.Metric.Write(e)
}

type Metric struct {
	Name    string
	Sampler *Sampler
	Entries []*Entry

	Type   int
	Intent int

	entryMutex sync.Mutex
}

func (m *Metric) Entry() *Entry {
	return &Entry{Metric: m, Fields: make(map[string]interface{})}
}

func (m *Metric) WithField(key string, value interface{}) *Entry {
	return m.Entry().WithField(key, value)
}

func (m *Metric) WithFields(fields Fields) *Entry {
	return m.Entry().WithFields(fields)
}

func (m *Metric) Int(v int) {
	m.Entry().Int(v)
}

func (m *Metric) Int64(v int64) {
	m.Entry().Int64(v)
}

func (m *Metric) Duration(d time.Duration) {
	m.Entry().Duration(d)
}

func (m *Metric) Write(e *Entry) {
	m.entryMutex.Lock()
	defer m.entryMutex.Unlock()

	m.Entries = append(m.Entries, e)
	m.Sampler.Write(m, e)
}

func (m *Metric) Min() int64 {
	var min int64
	for _, e := range m.Entries {
		if min == 0 || e.Value < min {
			min = e.Value
		}
	}
	return min
}

func (m *Metric) Max() int64 {
	var max int64
	for _, e := range m.Entries {
		if e.Value > max {
			max = e.Value
		}
	}
	return max
}

func (m *Metric) Avg() int64 {
	if len(m.Entries) == 0 {
		return 0
	}

	var sum int64
	for _, e := range m.Entries {
		sum += e.Value
	}
	return sum / int64(len(m.Entries))
}

func (m *Metric) Med() int64 {
	return m.Entries[(len(m.Entries)/2)-1].Value
}

type Sampler struct {
	Metrics map[string]*Metric
	Outputs []Output
	OnError func(error)

	MetricMutex sync.Mutex
}

func New() *Sampler {
	return &Sampler{Metrics: make(map[string]*Metric)}
}

func (s *Sampler) Get(name string) *Metric {
	s.MetricMutex.Lock()
	defer s.MetricMutex.Unlock()

	metric, ok := s.Metrics[name]
	if !ok {
		metric = &Metric{Name: name, Sampler: s}
		s.Metrics[name] = metric
	}
	return metric
}

func (s *Sampler) GetAs(name string, t int) *Metric {
	m := s.Get(name)
	m.Type = t
	return m
}

func (s *Sampler) Stats(name string) *Metric {
	return s.GetAs(name, StatsType)
}

func (s *Sampler) Counter(name string) *Metric {
	return s.GetAs(name, CounterType)
}

func (s *Sampler) Write(m *Metric, e *Entry) {
	for _, out := range s.Outputs {
		if err := out.Write(m, e); err != nil {
			if s.OnError != nil {
				s.OnError(err)
			}
		}
	}
}

func (s *Sampler) Commit() error {
	for _, out := range s.Outputs {
		if err := out.Commit(); err != nil {
			return err
		}
	}
	return nil
}

type Output interface {
	Write(m *Metric, e *Entry) error
	Commit() error
}
