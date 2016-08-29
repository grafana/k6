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
	mutex     sync.Mutex
}

func (e *Engine) Run(ctx context.Context) error {
	e.ctx = ctx

	e.Status.StartTime = time.Now()
	e.Status.Running = true
	defer func() {
		e.Status.Running = false
		e.Status.VUs = 0
	}()

	<-ctx.Done()

	return nil
}

func (e *Engine) Scale(vus int64) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	l := int64(len(e.cancelers))
	switch {
	case l < vus:
		for i := int64(len(e.cancelers)); i < vus; i++ {
			vu, err := e.Runner.NewVU()
			if err != nil {
				return err
			}

			if err := vu.Reconfigure(i + 1); err != nil {
				return err
			}

			ctx, cancel := context.WithCancel(e.ctx)
			e.cancelers = append(e.cancelers, cancel)
			go e.runVU(ctx, vu)
		}
	case l > vus:
		for _, cancel := range e.cancelers[vus+1:] {
			cancel()
		}
		e.cancelers = e.cancelers[:vus]
	}

	e.Status.VUs = int64(len(e.cancelers))

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
