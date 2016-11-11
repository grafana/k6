package lib

import (
	"context"
	"errors"
	"fmt"
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

	// Special error used to taint a test, without printing an error.
	ErrVUWantsTaint = errors.New("[ErrVUWantsTaint is never logged]")
)

type vuEntry struct {
	VU        VU
	Buffer    []stats.Sample
	ExtBuffer stats.Buffer
	Cancel    context.CancelFunc
}

type Engine struct {
	Runner    Runner
	Status    Status
	Collector stats.Collector
	Remaining time.Duration
	Quit      bool
	Pause     sync.WaitGroup

	Metrics map[*stats.Metric]stats.Sink

	ctx    context.Context
	vus    []*vuEntry
	nextID int64

	vuMutex sync.Mutex
}

func NewEngine(r Runner) (*Engine, error) {
	e := &Engine{
		Runner: r,
		Status: Status{
			Running: null.BoolFrom(false),
			Tainted: null.BoolFrom(false),
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

	e.consumeEngineStats()
	interval := 1 * time.Second
	ticker := time.NewTicker(interval)

	if e.Collector != nil {
		go e.Collector.Run(ctx)
	} else {
		log.Debug("Engine: No Collector")
	}

loop:
	for {
		select {
		case <-ticker.C:
			e.consumeEngineStats()

			for _, vu := range e.vus {
				e.consumeBuffer(vu.Buffer)
				vu.Buffer = vu.Buffer[:0]
			}

			// Update the duration counter. This will be replaced with a proper timeline Soon(tm).
			if e.Status.Running.Bool && e.Remaining != 0 {
				e.Remaining -= interval
				if e.Remaining <= 0 {
					e.Remaining = 0
					e.SetRunning(false)

					if !e.Quit {
						log.Info("Test expired, execution paused, pass --quit to exit here")
					} else {
						log.Info("Test ended, bye!")
						break loop
					}
				}
			}
		case <-ctx.Done():
			break loop
		}
	}

	e.vus = nil

	e.Status.Running = null.BoolFrom(false)
	e.Status.VUs = null.IntFrom(0)
	e.Status.VUsMax = null.IntFrom(0)
	e.consumeEngineStats()

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
		go e.runVU(ctx, e.nextID, entry)
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
			entry := &vuEntry{VU: vu}
			if e.Collector != nil {
				entry.ExtBuffer = e.Collector.Buffer()
			}
			vus = append(vus, entry)
		}
		e.vus = vus
	} else if v < current {
		e.vus = e.vus[:v]
	}

	e.Status.VUsMax.Int64 = v
	return nil
}

func (e *Engine) runVU(ctx context.Context, id int64, vu *vuEntry) {
	idString := strconv.FormatInt(id, 10)

waitForPause:
	e.Pause.Wait()

	for {
		samples, err := vu.VU.RunOnce(ctx)

		// If the context is cancelled, the iteration is likely to produce erroneous output
		// due to cancelled HTTP requests and whatnot. Discard output from such runs.
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err != nil {
			e.Status.Tainted.Bool = true

			if err != ErrVUWantsTaint {
				if s, ok := err.(fmt.Stringer); ok {
					log.Error(s.String())
				} else {
					log.WithError(err).Error("Runtime Error")
				}

				samples = append(samples, stats.Sample{
					Metric: MetricErrors,
					Time:   time.Now(),
					Tags:   map[string]string{"vu": idString, "error": err.Error()},
					Value:  float64(1),
				})
			}
		}

		vu.Buffer = append(vu.Buffer, samples...)
		if vu.ExtBuffer != nil {
			vu.ExtBuffer.Add(samples...)
		}

		if !e.Status.Running.Bool {
			goto waitForPause
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

func (e *Engine) consumeBuffer(buffer []stats.Sample) {
	for _, sample := range buffer {
		e.getSink(sample.Metric).Add(sample)
	}
}

func (e *Engine) consumeEngineStats() {
	t := time.Now()
	e.consumeBuffer([]stats.Sample{
		stats.Sample{Metric: MetricVUs, Time: t, Value: float64(e.Status.VUs.Int64)},
		stats.Sample{Metric: MetricVUsMax, Time: t, Value: float64(e.Status.VUsMax.Int64)},
	})
}
