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

package json

import (
	"bufio"
	"compress/gzip"
	stdlibjson "encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"go.k6.io/k6/output"
	"go.k6.io/k6/stats"
)

// TODO: add option for emitting proper JSON files (https://github.com/k6io/k6/issues/737)
const flushPeriod = 200 * time.Millisecond // TODO: make this configurable

// Output funnels all passed metrics to an (optionally gzipped) JSON file.
type Output struct {
	output.SampleBuffer

	params          output.Params
	periodicFlusher *output.PeriodicFlusher

	logger      logrus.FieldLogger
	filename    string
	encoder     *stdlibjson.Encoder
	closeFn     func() error
	seenMetrics map[string]struct{}
	thresholds  map[string][]*stats.Threshold
}

// New returns a new JSON output.
func New(params output.Params) (output.Output, error) {
	return &Output{
		params:   params,
		filename: params.ConfigArgument,
		logger: params.Logger.WithFields(logrus.Fields{
			"output":   "json",
			"filename": params.ConfigArgument,
		}),
		seenMetrics: make(map[string]struct{}),
	}, nil
}

// Description returns a human-readable description of the output.
func (o *Output) Description() string {
	if o.filename == "" || o.filename == "-" {
		return "json(stdout)"
	}
	return fmt.Sprintf("json (%s)", o.filename)
}

// Start tries to open the specified JSON file and starts the goroutine for
// metric flushing. If gzip encoding is specified, it also handles that.
func (o *Output) Start() error {
	o.logger.Debug("Starting...")

	if o.filename == "" || o.filename == "-" {
		w := bufio.NewWriter(o.params.StdOut)
		o.encoder = stdlibjson.NewEncoder(w)
		o.closeFn = func() error {
			return w.Flush()
		}
	} else {
		logfile, err := o.params.FS.Create(o.filename)
		if err != nil {
			return err
		}
		w := bufio.NewWriter(logfile)

		if strings.HasSuffix(o.filename, ".gz") {
			outfile := gzip.NewWriter(w)

			o.closeFn = func() error {
				_ = outfile.Close()
				_ = w.Flush()
				return logfile.Close()
			}
			o.encoder = stdlibjson.NewEncoder(outfile)
		} else {
			o.closeFn = func() error {
				_ = w.Flush()
				return logfile.Close()
			}
			o.encoder = stdlibjson.NewEncoder(w)
		}
	}

	o.encoder.SetEscapeHTML(false)

	pf, err := output.NewPeriodicFlusher(flushPeriod, o.flushMetrics)
	if err != nil {
		return err
	}
	o.logger.Debug("Started!")
	o.periodicFlusher = pf

	return nil
}

// Stop flushes any remaining metrics and stops the goroutine.
func (o *Output) Stop() error {
	o.logger.Debug("Stopping...")
	defer o.logger.Debug("Stopped!")
	o.periodicFlusher.Stop()
	return o.closeFn()
}

// SetThresholds receives the thresholds before the output is Start()-ed.
func (o *Output) SetThresholds(thresholds map[string]stats.Thresholds) {
	ths := make(map[string][]*stats.Threshold)
	for name, t := range thresholds {
		ths[name] = append(ths[name], t.Thresholds...)
	}
	o.thresholds = ths
}

func (o *Output) flushMetrics() {
	samples := o.GetBufferedSamples()
	start := time.Now()
	var count int
	for _, sc := range samples {
		samples := sc.GetSamples()
		count += len(samples)
		for _, sample := range samples {
			sample := sample
			sample.Metric.Thresholds.Thresholds = o.thresholds[sample.Metric.Name]
			o.handleMetric(sample.Metric)
			err := o.encoder.Encode(WrapSample(sample))
			if err != nil {
				// Skip metric if it can't be made into JSON or envelope is null.
				o.logger.WithError(err).Error("Sample couldn't be marshalled to JSON")
			}
		}
	}
	if count > 0 {
		o.logger.WithField("t", time.Since(start)).WithField("count", count).Debug("Wrote metrics to JSON")
	}
}

func (o *Output) handleMetric(m *stats.Metric) {
	if _, ok := o.seenMetrics[m.Name]; ok {
		return
	}
	o.seenMetrics[m.Name] = struct{}{}

	err := o.encoder.Encode(wrapMetric(m))
	if err != nil {
		o.logger.WithError(err).Error("Metric couldn't be marshalled to JSON")
	}
}
