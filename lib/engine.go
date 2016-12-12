package lib

import (
	"context"
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/k6/stats"
	"github.com/robertkrimen/otto"
	"gopkg.in/guregu/null.v3"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

const (
	TickRate          = 1 * time.Millisecond
	ThresholdTickRate = 2 * time.Second
	ShutdownTimeout   = 10 * time.Second
)

var (
	MetricVUs    = &stats.Metric{Name: "vus", Type: stats.Gauge}
	MetricVUsMax = &stats.Metric{Name: "vus_max", Type: stats.Gauge}
	MetricRuns   = &stats.Metric{Name: "runs", Type: stats.Gauge}
	MetricErrors = &stats.Metric{Name: "errors", Type: stats.Counter}

	ErrTooManyVUs = errors.New("More VUs than the maximum requested")
	ErrMaxTooLow  = errors.New("Can't lower max below current VU count")

	// Special error used to taint a test, without printing an error.
	ErrVUWantsTaint = errors.New("silent taint")
)

type vuEntry struct {
	VU     VU
	Buffer []stats.Sample
	Cancel context.CancelFunc
}

type Engine struct {
	Runner    Runner
	Status    Status
	Stages    []Stage
	Collector stats.Collector
	Pause     sync.WaitGroup
	Metrics   map[*stats.Metric]stats.Sink

	Thresholds  map[string][]*Threshold
	thresholdVM *otto.Otto

	ctx    context.Context
	vus    []*vuEntry
	nextID int64

	vuMutex   sync.Mutex
	waitGroup sync.WaitGroup
}

func NewEngine(r Runner) (*Engine, error) {
	e := &Engine{
		Runner: r,
		Status: Status{
			Running:      null.BoolFrom(false),
			Tainted:      null.BoolFrom(false),
			VUs:          null.IntFrom(0),
			VUsMax:       null.IntFrom(0),
			AtTime:       null.IntFrom(0),
			Linger:       null.BoolFrom(false),
			AbortOnTaint: null.BoolFrom(false),
			Acceptance:   null.FloatFrom(0.0),
		},
		Metrics:     make(map[*stats.Metric]stats.Sink),
		Thresholds:  make(map[string][]*Threshold),
		thresholdVM: otto.New(),
	}
	e.Pause.Add(1)

	return e, nil
}

func (e *Engine) Apply(opts Options) error {
	if opts.Paused.Valid {
		e.SetRunning(!opts.Paused.Bool)
	}
	if opts.VUsMax.Valid {
		if err := e.SetMaxVUs(opts.VUsMax.Int64); err != nil {
			return err
		}
	}
	if opts.VUs.Valid {
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

	if opts.Linger.Valid {
		e.Status.Linger = opts.Linger
	}
	if opts.AbortOnTaint.Valid {
		e.Status.AbortOnTaint = opts.AbortOnTaint
	}
	if opts.Acceptance.Valid {
		e.Status.Acceptance = opts.Acceptance
	}

	if opts.Thresholds != nil {
		e.Thresholds = opts.Thresholds

		// Make sure all scripts are compiled!
		for m, scripts := range e.Thresholds {
			for i, script := range scripts {
				if script.Script != nil {
					continue
				}

				s, err := e.thresholdVM.Compile(fmt.Sprintf("threshold$%s:%i", m, i), script.Source)
				if err != nil {
					return err
				}
				script.Script = s
				scripts[i] = script
			}
		}
	}

	return nil
}

func (e *Engine) Run(ctx context.Context) error {
	subctx, cancel := context.WithCancel(context.Background())
	e.ctx = subctx
	e.nextID = 1

	if err := e.Apply(e.Runner.GetOptions()); err != nil {
		return err
	}

	if e.Collector != nil {
		e.waitGroup.Add(1)
		go func() {
			e.Collector.Run(subctx)
			log.Debug("Engine: Collector shut down")
			e.waitGroup.Done()
		}()
	} else {
		log.Debug("Engine: No Collector")
	}

	e.waitGroup.Add(1)
	go func() {
		e.runThresholds(subctx)
		log.Debug("Engine: Thresholds shut down")
		e.waitGroup.Done()
	}()

	e.consumeEngineStats()

	ticker := time.NewTicker(TickRate)
	lastTick := time.Now()

loop:
	for {
		select {
		case now := <-ticker.C:
			// Don't tick at all if the engine is paused.
			if !e.Status.Running.Bool {
				continue
			}

			// Track time deltas to ensure smooth interpolation even in the face of latency.
			timeDelta := now.Sub(lastTick)
			e.Status.AtTime.Int64 += int64(timeDelta)
			lastTick = now

			// Handle stages and VU interpolation.
			stage, left, ok := StageAt(e.Stages, time.Duration(e.Status.AtTime.Int64))
			if stage.StartVUs.Valid && stage.EndVUs.Valid {
				progress := (float64(stage.Duration.Int64-int64(left)) / float64(stage.Duration.Int64))
				vus := Lerp(stage.StartVUs.Int64, stage.EndVUs.Int64, progress)
				if err := e.SetVUs(vus); err != nil {
					log.WithError(err).Error("Engine: Couldn't interpolate")
				}
			}

			// Consume sample buffers. We use copies to avoid a race condition with runVU();
			// concurrent append() calls on the same list will result in a crash.
			for _, vu := range e.vus {
				buffer := vu.Buffer
				if buffer == nil {
					continue
				}
				vu.Buffer = nil
				e.consumeBuffer(buffer)
			}

			// If the test has ended, either pause or shut down.
			if !ok {
				e.SetRunning(false)

				if e.Status.Linger.Bool {
					continue
				}
				break loop
			}

			// Check the taint rate acceptance to decide taint status.
			taintRate := float64(e.Status.Taints) / float64(e.Status.Runs)
			e.Status.Tainted.Bool = taintRate > e.Status.Acceptance.Float64

			// If the test is tainted, and we've requested --abort-on-taint, shut down.
			if e.Status.AbortOnTaint.Bool && e.Status.Tainted.Bool {
				log.Warn("Test tainted, ending early...")
				break loop
			}

			// Update internal metrics.
			e.consumeEngineStats()
		case <-ctx.Done():
			break loop
		}
	}

	// Without this, VUs will remain frozen and not shut down when asked.
	if !e.Status.Running.Bool {
		e.Pause.Done()
	}

	_ = e.SetVUs(0)
	_ = e.SetMaxVUs(0)
	e.SetRunning(false)
	e.consumeEngineStats()

	e.ctx = nil
	cancel()

	log.Debug("Engine: Waiting for subsystem shutdown...")

	done := make(chan interface{})
	go func() {
		e.waitGroup.Wait()
		close(done)
	}()
	timeout := time.After(ShutdownTimeout)
	select {
	case <-done:
	case <-timeout:
		log.Warn("VUs took too long to finish, shutting down anyways")
	}

	return nil
}

func (e *Engine) IsRunning() bool {
	return e.ctx != nil
}

func (e *Engine) TotalTime() (total time.Duration, finite bool) {
	for _, stage := range e.Stages {
		if stage.Duration.Valid {
			total += time.Duration(stage.Duration.Int64)
			finite = true
		}
	}
	return total, finite
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
		e.waitGroup.Add(1)
		id := e.nextID
		go func() {
			id := id
			e.runVU(ctx, id, entry)
			log.WithField("id", id).Debug("Engine: VU terminated")
			e.waitGroup.Done()
		}()
		e.nextID++
	}
	for i := current - 1; i >= v; i-- {
		log.WithField("id", i).Debug("Engine: Terminating VU...")
		entry := e.vus[i]
		entry.Cancel()
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

		atomic.AddInt64(&e.Status.Runs, 1)

		if err != nil {
			atomic.AddInt64(&e.Status.Taints, 1)

			samples = append(samples, stats.Sample{
				Metric: MetricErrors,
				Time:   time.Now(),
				Tags:   map[string]string{"vu": idString, "error": err.Error()},
				Value:  float64(1),
			})

			if err != ErrVUWantsTaint {
				if s, ok := err.(fmt.Stringer); ok {
					log.Error(s.String())
				} else {
					log.WithError(err).Error("Runtime Error")
				}
			}
		}

		if samples != nil {
			buffer := vu.Buffer
			if buffer == nil {
				buffer = samples
			} else {
				buffer = append(buffer, samples...)
			}
			vu.Buffer = buffer
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
				scripts, ok := e.Thresholds[m.Name]
				if !ok {
					continue
				}

				sample := sink.Format()
				for key, value := range sample {
					if m.Contains == stats.Time {
						value = value / float64(time.Millisecond)
					}
					// log.WithFields(log.Fields{"k": key, "v": value}).Debug("setting threshold data")
					if err := e.thresholdVM.Set(key, value); err != nil {
						log.WithError(err).Error("Threshold Context Error")
						return
					}
				}

				taint := false
				for _, script := range scripts {
					v, err := e.thresholdVM.Run(script.Script)
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
						script.Failed = true
					}
				}

				for key, _ := range sample {
					if err := e.thresholdVM.Set(key, otto.UndefinedValue()); err != nil {
						log.WithError(err).Error("Threshold Teardown Error")
						return
					}
				}

				if taint {
					m.Tainted = true
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
	if e.Collector != nil {
		e.Collector.Collect(buffer)
	}
}

func (e *Engine) consumeEngineStats() {
	t := time.Now()
	e.consumeBuffer([]stats.Sample{
		stats.Sample{Metric: MetricVUs, Time: t, Value: float64(e.Status.VUs.Int64)},
		stats.Sample{Metric: MetricVUsMax, Time: t, Value: float64(e.Status.VUsMax.Int64)},
		stats.Sample{Metric: MetricRuns, Time: t, Value: float64(e.Status.Runs)},
	})
}
