package lib

import (
	"context"
	"time"
)

type State struct {
	StartTime time.Time `json:"startTime"`

	Running bool  `json:"running"`
	VUs     int64 `json:"vus"`
}

type Engine struct {
	Runner Runner
	State  State
}

func (e *Engine) Run(ctx context.Context) error {
	e.State.StartTime = time.Now()
	e.State.Running = true
	defer func() {
		e.State.Running = false
		e.State.VUs = 0
	}()

	<-ctx.Done()

	return nil
}

func (e *Engine) Scale(vus int64) error {
	e.State.VUs = vus
	return nil
}
