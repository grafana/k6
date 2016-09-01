package lib

import (
	"context"
	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/speedboat/stats"
	"strconv"
	"sync"
	"time"
)

var (
	MetricVUs       = &stats.Metric{Name: "vus", Type: stats.Gauge}
	MetricVUsPooled = &stats.Metric{Name: "vus_pooled", Type: stats.Gauge}
	MetricErrors    = &stats.Metric{Name: "errors", Type: stats.Counter}
)

type Engine struct {
	Runner  Runner
	Status  Status
	Metrics map[*stats.Metric][]stats.Sample

	ctx       context.Context
	cancelers []context.CancelFunc
	pool      []VU

	vuMutex sync.Mutex
	mMutex  sync.Mutex
}

func NewEngine(r Runner) *Engine {
	return &Engine{
		Runner:  r,
		Metrics: make(map[*stats.Metric][]stats.Sample),
	}
}

func (e *Engine) Run(ctx context.Context, prepared int64) error {
	e.ctx = ctx

	e.pool = make([]VU, prepared)
	for i := int64(0); i < prepared; i++ {
		vu, err := e.Runner.NewVU()
		if err != nil {
			return err
		}
		e.pool[i] = vu
	}

	e.Status.StartTime = time.Now()
	e.Status.Running = true
	e.Status.VUs = 0
	e.Status.Pooled = prepared

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

	e.Status.Running = false
	e.Status.VUs = 0
	e.Status.Pooled = 0

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

	e.Status.VUs = int64(len(e.cancelers))
	e.Status.Pooled = int64(len(e.pool))

	return nil
}

func (e *Engine) reportInternalStats() {
	e.mMutex.Lock()
	t := time.Now()
	e.Metrics[MetricVUs] = append(
		e.Metrics[MetricVUs],
		stats.Sample{Time: t, Tags: nil, Value: float64(len(e.cancelers))},
	)
	e.Metrics[MetricVUsPooled] = append(
		e.Metrics[MetricVUsPooled],
		stats.Sample{Time: t, Tags: nil, Value: float64(len(e.pool))},
	)
	e.mMutex.Unlock()
}

func (e *Engine) runVU(ctx context.Context, id int64, vu VU) {
	idString := strconv.FormatInt(id, 10)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			samples, err := vu.RunOnce(ctx)
			e.mMutex.Lock()
			if err != nil {
				log.WithField("vu", id).WithError(err).Error("Runtime Error")
				e.Metrics[MetricErrors] = append(e.Metrics[MetricErrors], stats.Sample{
					Time:  time.Now(),
					Tags:  map[string]string{"vu": idString, "error": err.Error()},
					Value: float64(1),
				})
			}
			for _, s := range samples {
				e.Metrics[s.Metric] = append(e.Metrics[s.Metric], s.Sample)
			}
			e.mMutex.Unlock()
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
