package lib

import (
	"context"
	log "github.com/Sirupsen/logrus"
	"sync"
	"time"
)

type Engine struct {
	Runner Runner
	Status Status

	ctx       context.Context
	cancelers []context.CancelFunc
	pool      []VU
	mutex     sync.Mutex
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

	<-ctx.Done()

	e.cancelers = nil
	e.pool = nil

	e.Status.Running = false
	e.Status.VUs = 0
	e.Status.Pooled = 0

	return nil
}

func (e *Engine) Scale(vus int64) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	l := int64(len(e.cancelers))
	switch {
	case l < vus:
		for i := int64(len(e.cancelers)); i < vus; i++ {
			vu, err := e.getVU()
			if err != nil {
				return err
			}

			if err := vu.Reconfigure(i + 1); err != nil {
				return err
			}

			ctx, cancel := context.WithCancel(e.ctx)
			e.cancelers = append(e.cancelers, cancel)
			go func() {
				e.runVU(ctx, vu)

				e.mutex.Lock()
				e.pool = append(e.pool, vu)
				e.mutex.Unlock()
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

func (e *Engine) runVU(ctx context.Context, vu VU) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if err := vu.RunOnce(ctx); err != nil {
				log.WithError(err).Error("Runtime Error")
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
