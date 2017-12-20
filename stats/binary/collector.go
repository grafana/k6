/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
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

package binary

import (
	"bufio"
	"context"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/loadimpact/k6/stats"
	"github.com/spf13/afero"
)

// Collection represents a binary collector that writes
// samples to a gob binary file
type Collector struct {
	file    io.WriteCloser
	writer  *bufio.Writer
	encoder *gob.Encoder
	recvr   chan []stats.Sample

	closed bool
	lock   sync.Mutex
}

func init() {
	// register type passed as interface{}
	gob.Register(stats.CounterSink{})
	gob.Register(stats.GaugeSink{})
	gob.Register(stats.TrendSink{})
	gob.Register(stats.RateSink{})
	gob.Register(stats.DummySink{})
}

// New returns a collector ready to start writing samples
func New(fs afero.Fs, fname string) (*Collector, error) {
	if len(fname) == 0 {
		return nil, fmt.Errorf("you must specify a filename => -o bin=filename.k6")
	}

	if fname == "-" {
		return &Collector{
			file:  os.Stdout,
			recvr: make(chan []stats.Sample),
		}, nil
	}

	logfile, err := fs.Create(fname)
	if err != nil {
		return nil, err
	}
	return &Collector{
		file:  logfile,
		recvr: make(chan []stats.Sample),
	}, nil
}

// Init prepares the bufio writer and the gob encoder
func (c *Collector) Init() error {
	c.writer = bufio.NewWriter(c.file)
	c.encoder = gob.NewEncoder(c.writer)
	return nil
}

// Run writes samples received via the receiver channel and handle
// the flush of data to file once the iterations are completed
func (c *Collector) Run(ctx context.Context) {
	for {
		select {
		case samples := <-c.recvr:
			if err := c.encoder.Encode(samples); err != nil {
				fmt.Println(err)
			}

		case <-ctx.Done():
			// HACK: even with the break there's still the ctx.Done() being
			// called twice

			if c.closed {
				return
			}

			c.lock.Lock()
			c.closed = true
			c.lock.Unlock()

			close(c.recvr)
			c.writer.Flush()
			break
		}
	}
}

// Collect receives chunk of new sample that are sent to the
// receiver channel for processing
func (c *Collector) Collect(samples []stats.Sample) {
	go func() {
		c.recvr <- samples
	}()
}

// Link returns nothing as this does not creates a linkable resource
func (c *Collector) Link() string {
	return ""
}
