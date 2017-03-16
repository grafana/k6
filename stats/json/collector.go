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
	"context"
	"encoding/json"
	"io"

	log "github.com/Sirupsen/logrus"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
	"github.com/spf13/afero"
)

type Collector struct {
	outfile     io.WriteCloser
	fname       string
	seenMetrics []string
}

func (c *Collector) HasSeenMetric(str string) bool {
	for _, n := range c.seenMetrics {
		if n == str {
			return true
		}
	}
	return false
}

func New(fname string, fs afero.Fs, opts lib.Options) (*Collector, error) {
	logfile, err := fs.Create(fname)
	if err != nil {
		return nil, err
	}

	t := make([]string, 16)
	return &Collector{
		outfile:     logfile,
		fname:       fname,
		seenMetrics: t,
	}, nil
}

func (c *Collector) Init() {
}

func (c *Collector) String() string {
	return "JSON"
}

func (c *Collector) Run(ctx context.Context) {
	log.WithField("filename", c.fname).Debug("JSON: Writing JSON metrics")
	<-ctx.Done()
	_ = c.outfile.Close()
}

func (c *Collector) HandleMetric(m *stats.Metric) {
	if c.HasSeenMetric(m.Name) {
		return
	}

	c.seenMetrics = append(c.seenMetrics, m.Name)
	env := WrapMetric(m)
	row, err := json.Marshal(env)

	if env == nil || err != nil {
		log.WithField("filename", c.fname).Warning(
			"JSON: Envelope is nil or Metric couldn't be marshalled to JSON")
		return
	}

	row = append(row, '\n')
	_, err = c.outfile.Write(row)
	if err != nil {
		log.WithField("filename", c.fname).Error("JSON: Error writing to file")
	}
}

func (c *Collector) Collect(samples []stats.Sample) {
	for _, sample := range samples {
		c.HandleMetric(sample.Metric)

		env := WrapSample(&sample)
		row, err := json.Marshal(env)

		if err != nil || env == nil {
			// Skip metric if it can't be made into JSON or envelope is null.
			log.WithField("filename", c.fname).Warning(
				"JSON: Envelope is nil or Sample couldn't be marshalled to JSON")
			continue
		}
		row = append(row, '\n')
		_, err = c.outfile.Write(row)
		if err != nil {
			log.WithField("filename", c.fname).Error("JSON: Error writing to file")
			continue
		}
	}
}
