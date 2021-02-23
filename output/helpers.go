/*
 *
 * k6 - a next-generation load testing tool
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

package output

import (
	"fmt"
	"sync"
	"time"

	"github.com/loadimpact/k6/stats"
)

// SampleBuffer is a simple thread-safe buffer for metric samples. It should be
// used by most outputs, since we generally want to flush metric samples to the
// remote service asynchronously. We want to do it only every several seconds,
// and we don't want to block the Engine in the meantime.
type SampleBuffer struct {
	sync.Mutex
	buffer []stats.SampleContainer
	maxLen int
}

// AddMetricSamples adds the given metric samples to the internal buffer.
func (sc *SampleBuffer) AddMetricSamples(samples []stats.SampleContainer) {
	sc.Lock()
	sc.buffer = append(sc.buffer, samples...)
	sc.Unlock()
}

// GetBufferedSamples returns the currently buffered metric samples and makes a
// new internal buffer with some hopefully realistic size.
func (sc *SampleBuffer) GetBufferedSamples() (buffered []stats.SampleContainer) {
	sc.Lock()
	buffered = sc.buffer
	if len(buffered) > sc.maxLen {
		sc.maxLen = len(buffered)
	}
	// Make the new buffer halfway between the previously allocated size and the
	// maximum buffer size we've seen so far, to hopefully reduce copying a bit.
	sc.buffer = make([]stats.SampleContainer, 0, (len(buffered)+sc.maxLen)/2)
	sc.Unlock()
	return buffered
}

// PeriodicFlusher is a small helper for asynchronously flushing buffered metric
// samples on regular intervals. The biggest benefit is having a Stop() method
// that waits for one last flush before it returns.
type PeriodicFlusher struct {
	period        time.Duration
	flushCallback func()
	stop          chan struct{}
	stopped       chan struct{}
}

func (pf *PeriodicFlusher) run() {
	ticker := time.NewTicker(pf.period)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			pf.flushCallback()
		case <-pf.stop:
			pf.flushCallback()
			close(pf.stopped)
			return
		}
	}
}

// Stop waits for the periodic flusher flush one last time and exit. You can
// safely call Stop() multiple times from different goroutines.
func (pf *PeriodicFlusher) Stop() {
	select {
	case <-pf.stop:
		// Aldready stopped
	default:
		close(pf.stop)
	}
	<-pf.stopped
}

// NewPeriodicFlusher creates a new PeriodicFlusher and starts its goroutine.
func NewPeriodicFlusher(period time.Duration, flushCallback func()) (*PeriodicFlusher, error) {
	if period <= 0 {
		return nil, fmt.Errorf("metric flush period should be positive but was %s", period)
	}

	pf := &PeriodicFlusher{
		period:        period,
		flushCallback: flushCallback,
		stop:          make(chan struct{}),
		stopped:       make(chan struct{}),
	}

	go pf.run()

	return pf, nil
}
