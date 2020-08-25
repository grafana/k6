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

package json

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
)

type Collector struct {
	closeFn     func() error
	fname       string
	seenMetrics []string
	logger      logrus.FieldLogger

	encoder *json.Encoder

	buffer     []stats.Sample
	bufferLock sync.Mutex
}

// Verify that Collector implements lib.Collector
var _ lib.Collector = &Collector{}

func (c *Collector) HasSeenMetric(str string) bool {
	for _, n := range c.seenMetrics {
		if n == str {
			return true
		}
	}
	return false
}

// New return new JSON collector
func New(logger logrus.FieldLogger, fs afero.Fs, fname string) (*Collector, error) {
	c := &Collector{
		fname:  fname,
		logger: logger,
	}
	if fname == "" || fname == "-" {
		c.encoder = json.NewEncoder(os.Stdout)
		c.closeFn = func() error {
			return nil
		}
		return c, nil
	}
	logfile, err := fs.Create(c.fname)
	if err != nil {
		return nil, err
	}

	if strings.HasSuffix(c.fname, ".gz") {
		outfile := gzip.NewWriter(logfile)

		c.closeFn = func() error {
			_ = outfile.Close()
			return logfile.Close()
		}
		c.encoder = json.NewEncoder(outfile)
	} else {
		c.closeFn = logfile.Close
		c.encoder = json.NewEncoder(logfile)
	}

	return c, nil
}

func (c *Collector) Init() error {
	return nil
}

func (c *Collector) SetRunStatus(status lib.RunStatus) {}

func (c *Collector) Run(ctx context.Context) {
	const timeout = 200
	c.logger.Debug("JSON output: Running!")
	ticker := time.NewTicker(time.Millisecond * timeout)
	defer func() {
		_ = c.closeFn()
	}()
	for {
		select {
		case <-ticker.C:
			c.commit()
		case <-ctx.Done():
			c.commit()
			return
		}
	}
}

func (c *Collector) HandleMetric(m *stats.Metric) {
	if c.HasSeenMetric(m.Name) {
		return
	}

	c.seenMetrics = append(c.seenMetrics, m.Name)
	err := c.encoder.Encode(WrapMetric(m))
	if err != nil {
		c.logger.WithField("filename", c.fname).WithError(err).Warning(
			"JSON: Envelope is nil or Metric couldn't be marshalled to JSON")
		return
	}
}

func (c *Collector) Collect(scs []stats.SampleContainer) {
	c.bufferLock.Lock()
	defer c.bufferLock.Unlock()
	for _, sc := range scs {
		c.buffer = append(c.buffer, sc.GetSamples()...)
	}
}

func (c *Collector) commit() {
	c.bufferLock.Lock()
	samples := c.buffer
	c.buffer = nil
	c.bufferLock.Unlock()
	start := time.Now()
	var count int
	for _, sc := range samples {
		samples := sc.GetSamples()
		count += len(samples)
		for _, sample := range sc.GetSamples() {
			sample := sample
			c.HandleMetric(sample.Metric)
			err := c.encoder.Encode(WrapSample(&sample))
			if err != nil {
				// Skip metric if it can't be made into JSON or envelope is null.
				c.logger.WithField("filename", c.fname).WithError(err).Warning(
					"JSON: Sample couldn't be marshalled to JSON")
				continue
			}
		}
	}
	if count > 0 {
		c.logger.WithField("filename", c.fname).WithField("t", time.Since(start)).
			WithField("count", count).Debug("JSON: Wrote JSON metrics")
	}
}

func (c *Collector) Link() string {
	return ""
}

// GetRequiredSystemTags returns which sample tags are needed by this collector
func (c *Collector) GetRequiredSystemTags() stats.SystemTagSet {
	return stats.SystemTagSet(0) // There are no required tags for this collector
}
