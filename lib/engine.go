package lib

import (
	"context"
	"errors"
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/stats"
	"gopkg.in/guregu/null.v3"
	"strconv"
	"sync"
	"time"
)

var (
	MetricVUs    = &stats.Metric{Name: "vus", Type: stats.Gauge}
	MetricVUsMax = &stats.Metric{Name: "vus_max", Type: stats.Gauge}
	MetricErrors = &stats.Metric{Name: "errors", Type: stats.Counter}

	ErrTooManyVUs = errors.New("More VUs than the maximum requested")
	ErrMaxTooLow  = errors.New("Can't lower max below current VU count")
)

type vuEntry struct {
	VU     VU
	Cancel context.CancelFunc
}

type Engine struct {
	Runner  Runner
	Status  Status
	Metrics map[*stats.Metric]stats.Sink
	Pause   sync.WaitGroup

	ctx    context.Context
	vus    []*vuEntry
	nextID int64

	vuMutex sync.Mutex
	mMutex  sync.Mutex
}

func NewEngine(r Runner) (*Engine, error) {
	e := &Engine{
		Runner: r,
		Status: Status{
			Running: null.BoolFrom(false),
			VUs:     null.IntFrom(0),
			VUsMax:  null.IntFrom(0),
		},
		Metrics: make(map[*stats.Metric]stats.Sink),
	}

	e.Status.Running = null.BoolFrom(false)
	e.Pause.Add(1)

	e.Status.VUs = null.IntFrom(0)
	e.Status.VUsMax = null.IntFrom(0)

	return e, nil
}

func (e *Engine) Run(ctx context.Context) error {
	e.ctx = ctx
	e.nextID = 1

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

	e.vus = nil

	e.Status.Running = null.BoolFrom(false)
	e.Status.VUs = null.IntFrom(0)
	e.Status.VUsMax = null.IntFrom(0)
	e.reportInternalStats()

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

func (e *Engine) SetVUs(v int64) error {
	e.vuMutex.Lock()
	defer e.vuMutex.Unlock()

	if v > e.Status.VUsMax.Int64 {
		return ErrTooManyVUs
	}

	current := e.Status.VUs.Int64
	for i := current; i < v; i++ {
		entry := e.vus[i]
		if entry.Cancel != nil {
			panic(errors.New("ATTEMPTED TO RESCHEDULE RUNNING VU"))
		}

		ctx, cancel := context.WithCancel(e.ctx)
		entry.Cancel = cancel

		if err := entry.VU.Reconfigure(e.nextID); err != nil {
			return err
		}
		go e.runVU(ctx, e.nextID, entry.VU)
		e.nextID++
	}
	for i := current - 1; i >= v; i-- {
		entry := e.vus[i]
		entry.Cancel()
		entry.Cancel = nil
	}

	e.Status.VUs.Int64 = v
	return nil
}

func (e *Engine) SetMaxVUs(v int64) error {
	e.vuMutex.Lock()
	defer e.vuMutex.Unlock()

	if v < e.Status.VUs.Int64 {
		return ErrMaxTooLow
	}

	current := e.Status.VUsMax.Int64
	if v > current {
		vus := e.vus
		for i := current; i < v; i++ {
			vu, err := e.Runner.NewVU()
			if err != nil {
				return err
			}
			vus = append(vus, &vuEntry{VU: vu})
		}
		e.vus = vus
	} else if v < current {
		e.vus = e.vus[:v]
	}

	e.Status.VUsMax.Int64 = v
	return nil
}

func (e *Engine) reportInternalStats() {
	e.mMutex.Lock()
	t := time.Now()
	e.getSink(MetricVUs).Add(stats.Sample{Time: t, Tags: nil, Value: float64(e.Status.VUs.Int64)})
	e.getSink(MetricVUsMax).Add(stats.Sample{Time: t, Tags: nil, Value: float64(e.Status.VUsMax.Int64)})
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
