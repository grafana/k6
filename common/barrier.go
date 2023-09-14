package common

import (
	"context"
	"sync/atomic"
	"time"
)

type Barrier struct {
	count int64
	ch    chan bool
	errCh chan error
}

func NewBarrier() *Barrier {
	return &Barrier{
		count: 1,
		ch:    make(chan bool, 1),
		errCh: make(chan error, 1),
	}
}

func (b *Barrier) AddFrameNavigation(frame *Frame) {
	if frame.parentFrame != nil {
		return // We only care about top-frame navigation
	}
	ch, evCancelFn := createWaitForEventHandler(frame.ctx, frame, []string{EventFrameNavigation}, func(data any) bool {
		return true
	})
	go func() {
		defer evCancelFn() // Remove event handler
		atomic.AddInt64(&b.count, 1)
		select {
		case <-frame.ctx.Done():
		case <-time.After(frame.manager.timeoutSettings.navigationTimeout()):
			b.errCh <- ErrTimedOut
		case <-ch:
			b.ch <- true
		}
		atomic.AddInt64(&b.count, -1)
	}()
}

func (b *Barrier) Wait(ctx context.Context) error {
	if atomic.AddInt64(&b.count, -1) == 0 {
		return nil
	}

	select {
	case <-ctx.Done():
	case <-b.ch:
		return nil
	case err := <-b.errCh:
		return err
	}
	return nil
}
