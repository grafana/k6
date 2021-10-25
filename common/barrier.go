/*
 *
 * xk6-browser - a browser automation extension for k6
 * Copyright (C) 2021 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package common

import (
	"sync/atomic"
	"time"

	"golang.org/x/net/context"
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
	ch, evCancelFn := createWaitForEventHandler(frame.ctx, frame, []string{EventFrameNavigation}, func(data interface{}) bool { return true })
	go func() {
		defer evCancelFn() // Remove event handler
		atomic.AddInt64(&b.count, 1)
		select {
		case <-frame.ctx.Done():
		case <-time.After(time.Duration(frame.manager.timeoutSettings.navigationTimeout()) * time.Second):
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
