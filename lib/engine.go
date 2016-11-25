package lib

import (
	"context"
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/stats"
	"github.com/robertkrimen/otto"
	"gopkg.in/guregu/null.v3"
	"strconv"
	"sync"
	"time"
)

const (
	TickRate          = 1 * time.Millisecond
	ThresholdTickRate = 2 * time.Second
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
	Stages    []Stage
	Collector stats.Collector
	Pause     sync.WaitGroup
	Metrics   map[*stats.Metric]stats.Sink

	thresholds  map[string][]*otto.Script
	thresholdVM *otto.Otto

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
			AtTime:  null.IntFrom(0),
		},
		Metrics:     make(map[*stats.Metric]stats.Sink),
		thresholds:  make(map[string][]*otto.Script),
		thresholdVM: otto.New(),
	}
	e.Pause.Add(1)

	return e, nil
}

func (e *Engine) Apply(opts Options) error {
	if opts.Run.Valid {
		e.SetRunning(opts.Run.Bool)
	}
	if opts.VUsMax.Valid {
		if err := e.SetMaxVUs(opts.VUs.Int64); err != nil {
			return err
		}
	}
	if opts.VUs.Valid {
		if !opts.VUsMax.Valid {
			if err := e.SetMaxVUs(opts.VUs.Int64); err != nil {
				return err
			}
		}
		if err := e.SetVUs(opts.VUs.Int64); err != nil {
			return err
		}
	}
	if opts.Duration.Valid {
		duration, err := time.ParseDuration(opts.Duration.String)
		if err != nil {
			return err
		}
		e.Stages = []Stage{Stage{Duration: null.IntFrom(int64(duration))}}
	}

	if opts.Quit.Valid {
		e.Status.Quit = opts.Quit
	}
	if opts.QuitOnTaint.Valid {
		e.Status.QuitOnTaint = opts.QuitOnTaint
	}

	for metric, thresholds := range opts.Thresholds {
		for _, src := range thresholds {
			if err := e.AddThreshold(metric, src); err != nil {
				return err
			}
		}
	}

	return nil
}

func (e *Engine) Run(ctx context.Context, opts Options) error {
	e.ctx = ctx
	e.nextID = 1

	e.Apply(opts)

	if e.Collector != nil {
		go e.Collector.Run(ctx)
	} else {
		log.Debug("Engine: No Collector")
	}

	go e.runThresholds(ctx)

	e.consumeEngineStats()

	ticker := time.NewTicker(TickRate)
	lastTick := time.Now()

loop:
	for {
		select {
		case now := <-ticker.C:
			timeDelta := now.Sub(lastTick)
			e.Status.AtTime.Int64 += int64(timeDelta)
			lastTick = now

			stage, left, ok := StageAt(e.Stages, time.Duration(e.Status.AtTime.Int64))
			if stage.StartVUs.Valid && stage.EndVUs.Valid {
				progress := (float64(stage.Duration.Int64-int64(left)) / float64(stage.Duration.Int64))
				vus := Lerp(stage.StartVUs.Int64, stage.EndVUs.Int64, progress)
				e.SetVUs(vus)
			}

			for _, vu := range e.vus {
				e.consumeBuffer(vu.Buffer)
			}

			if !ok {
				e.SetRunning(false)

				if e.Status.Quit.Bool {
					break loop
				} else {
					log.Info("Test finished, press Ctrl+C to exit")
					<-ctx.Done()
					break loop
				}
			}

			if e.Status.QuitOnTaint.Bool && e.Status.Tainted.Bool {
				log.Warn("Test tainted, ending early...")
				break loop
			}

			e.consumeEngineStats()
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

func (e *Engine) IsRunning() bool {
	return e.ctx != nil
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
	if e.Status.VUs.Int64 == v {
		return nil
	}

	log.WithFields(log.Fields{"from": e.Status.VUs.Int64, "to": v}).Debug("Setting VUs")

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
	if e.Status.VUsMax.Int64 == v {
		return nil
	}

	log.WithFields(log.Fields{"from": e.Status.VUsMax.Int64, "to": v}).Debug("Setting Max VUs")

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

func (e *Engine) AddThreshold(metric, src string) error {
	script, err := e.thresholdVM.Compile("__threshold__", src)
	if err != nil {
		return err
	}

	e.thresholds[metric] = append(e.thresholds[metric], script)

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

func (e *Engine) runThresholds(ctx context.Context) {
	ticker := time.NewTicker(ThresholdTickRate)
	for {
		select {
		case <-ticker.C:
			for m, sink := range e.Metrics {
				scripts, ok := e.thresholds[m.Name]
				if !ok {
					continue
				}

				sample := sink.Format()
				for key, value := range sample {
					if m.Contains == stats.Time {
						value = value / float64(time.Millisecond)
					}
					// log.WithFields(log.Fields{"k": key, "v": value}).Debug("setting threshold data")
					e.thresholdVM.Set(key, value)
				}

				taint := false
				for _, script := range scripts {
					v, err := e.thresholdVM.Run(script.String())
					if err != nil {
						log.WithError(err).WithField("metric", m.Name).Error("Threshold Error")
						taint = true
						continue
					}
					// log.WithFields(log.Fields{"metric": m.Name, "v": v, "s": sample}).Debug("threshold tick")
					bV, err := v.ToBoolean()
					if err != nil {
						log.WithError(err).WithField("metric", m.Name).Error("Threshold result is invalid")
						taint = true
						continue
					}
					if !bV {
						taint = true
					}
				}

				for key, _ := range sample {
					e.thresholdVM.Set(key, otto.UndefinedValue())
				}

				if taint {
					m.Tainted = true
					e.Status.Tainted.Bool = true
				}
			}
		case <-ctx.Done():
			return
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
		case stats.Rate:
			s = &stats.RateSink{}
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
