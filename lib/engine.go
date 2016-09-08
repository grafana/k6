package lib

import (
	"context"
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/stats"
	"gopkg.in/guregu/null.v3"
	"strconv"
	"sync"
	"time"
)

var (
	MetricActiveVUs   = &stats.Metric{Name: "vus_active", Type: stats.Gauge}
	MetricInactiveVUs = &stats.Metric{Name: "vus_inactive", Type: stats.Gauge}
	MetricErrors      = &stats.Metric{Name: "errors", Type: stats.Counter}
)

type Engine struct {
	Runner  Runner
	Status  Status
	Metrics map[*stats.Metric]stats.Sink
	Pause   sync.WaitGroup

	ctx       context.Context
	cancelers []context.CancelFunc
	pool      []VU

	vuMutex sync.Mutex
	mMutex  sync.Mutex
}

func NewEngine(r Runner, prepared int64) (*Engine, error) {
	e := &Engine{
		Runner:  r,
		Metrics: make(map[*stats.Metric]stats.Sink),
		pool:    make([]VU, prepared),
	}

	for i := int64(0); i < prepared; i++ {
		vu, err := r.NewVU()
		if err != nil {
			return nil, err
		}
		e.pool[i] = vu
	}

	e.Status.Running = null.BoolFrom(false)
	e.Pause.Add(1)

	e.Status.ActiveVUs = null.IntFrom(0)
	e.Status.InactiveVUs = null.IntFrom(int64(len(e.pool)))

	return e, nil
}

func (e *Engine) Run(ctx context.Context) error {
	e.ctx = ctx

	e.reportInternalStats()
	ticker := time.NewTicker(1 * time.Second)

loop:
	for {
		select {
		case <-ticker.C:
			e.reportInternalStats()
		case <-ctx.Done():
			break loop
		}
	}

	e.cancelers = nil
	e.pool = nil

	e.Status.Running = null.BoolFrom(false)
	e.Status.ActiveVUs = null.IntFrom(0)
	e.Status.InactiveVUs = null.IntFrom(0)
	e.reportInternalStats()

	return nil
}

func (e *Engine) Scale(vus int64) error {
	e.vuMutex.Lock()
	defer e.vuMutex.Unlock()

	l := int64(len(e.cancelers))
	switch {
	case l < vus:
		for i := int64(len(e.cancelers)); i < vus; i++ {
			vu, err := e.getVU()
			if err != nil {
				return err
			}

			id := i + 1
			if err := vu.Reconfigure(id); err != nil {
				return err
			}

			ctx, cancel := context.WithCancel(e.ctx)
			e.cancelers = append(e.cancelers, cancel)
			go func() {
				e.runVU(ctx, id, vu)

				e.vuMutex.Lock()
				e.pool = append(e.pool, vu)
				e.vuMutex.Unlock()
			}()
		}
	case l > vus:
		for _, cancel := range e.cancelers[vus+1:] {
			cancel()
		}
		e.cancelers = e.cancelers[:vus]
	}

	e.Status.ActiveVUs = null.IntFrom(int64(len(e.cancelers)))
	e.Status.InactiveVUs = null.IntFrom(int64(len(e.pool)))

	return nil
}

func (e *Engine) SetRunning(running bool) {
	if running && !e.Status.Running.Bool {
		e.Pause.Done()
		log.Debug("Engine Unpaused")
	} else if !running && e.Status.Running.Bool {
		e.Pause.Add(1)
		log.Debug("Engine Paused")
	}
	e.Status.Running.Bool = running
}

func (e *Engine) reportInternalStats() {
	e.mMutex.Lock()
	t := time.Now()
	e.getSink(MetricActiveVUs).Add(stats.Sample{Time: t, Tags: nil, Value: float64(len(e.cancelers))})
	e.getSink(MetricInactiveVUs).Add(stats.Sample{Time: t, Tags: nil, Value: float64(len(e.pool))})
	e.mMutex.Unlock()
}

func (e *Engine) runVU(ctx context.Context, id int64, vu VU) {
	idString := strconv.FormatInt(id, 10)

waitForPause:
	e.Pause.Wait()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			samples, err := vu.RunOnce(ctx)
			e.mMutex.Lock()
			if err != nil {
				log.WithField("vu", id).WithError(err).Error("Runtime Error")
				e.getSink(MetricErrors).Add(stats.Sample{
					Time:  time.Now(),
					Tags:  map[string]string{"vu": idString, "error": err.Error()},
					Value: float64(1),
				})
			}
			for _, s := range samples {
				e.getSink(s.Metric).Add(s)
			}
			e.mMutex.Unlock()

			if !e.Status.Running.Bool {
				goto waitForPause
			}
		}
	}
}

// Returns a pooled VU if available, otherwise make a new one.
func (e *Engine) getVU() (VU, error) {
	l := len(e.pool)
	if l > 0 {
		vu := e.pool[l-1]
		e.pool = e.pool[:l-1]
		return vu, nil
	}

	log.Warn("More VUs requested than what was prepared; instantiation during tests is costly and may skew results!")
	return e.Runner.NewVU()
}

// Returns a value sink for a metric, created from the type if unavailable.
func (e *Engine) getSink(m *stats.Metric) stats.Sink {
	s, ok := e.Metrics[m]
	if !ok {
		switch m.Type {
		case stats.Counter:
			s = &stats.CounterSink{}
		case stats.Gauge:
			s = &stats.GaugeSink{}
		case stats.Trend:
			s = &stats.TrendSink{}
		}
		e.Metrics[m] = s
	}
	return s
}
