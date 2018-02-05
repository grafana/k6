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
	"os"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
	log "github.com/sirupsen/logrus"
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

func New(fs afero.Fs, fname string) (*Collector, error) {
	if fname == "" || fname == "-" {
		return &Collector{
			outfile: os.Stdout,
			fname:   "-",
		}, nil
	}

	logfile, err := fs.Create(fname)
	if err != nil {
		return nil, err
	}
	return &Collector{
		outfile: logfile,
		fname:   fname,
	}, nil
}

func (c *Collector) Init() error {
	return nil
}

func (c *Collector) GetOptions() lib.CollectorOptions {
	return lib.CollectorOptions{}
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

func (c *Collector) Link() string {
	return ""
}
