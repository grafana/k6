/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
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

package statsd

import (
	"fmt"
	"time"

	"github.com/DataDog/datadog-go/statsd"
	"github.com/sirupsen/logrus"

	"go.k6.io/k6/output"
	"go.k6.io/k6/stats"
)

// New creates a new statsd connector client
func New(params output.Params) (output.Output, error) {
	return newOutput(params)
}

func newOutput(params output.Params) (*Output, error) {
	conf, err := getConsolidatedConfig(params.JSONConfig, params.Environment, params.ConfigArgument)
	if err != nil {
		return nil, err
	}
	logger := params.Logger.WithFields(logrus.Fields{"output": "statsd"})

	return &Output{
		config: conf,
		logger: logger,
	}, nil
}

var _ output.Output = &Output{}

// Output sends result data to statsd daemons with the ability to send to datadog as well
type Output struct {
	output.SampleBuffer

	periodicFlusher *output.PeriodicFlusher

	config config

	logger logrus.FieldLogger
	client *statsd.Client
}

func (o *Output) dispatch(entry stats.Sample) error {
	var tagList []string
	if o.config.EnableTags.Bool {
		tagList = processTags(o.config.TagBlocklist, entry.Tags.CloneTags())
	}

	switch entry.Metric.Type {
	case stats.Counter:
		return o.client.Count(entry.Metric.Name, int64(entry.Value), tagList, 1)
	case stats.Trend:
		return o.client.TimeInMilliseconds(entry.Metric.Name, entry.Value, tagList, 1)
	case stats.Gauge:
		return o.client.Gauge(entry.Metric.Name, entry.Value, tagList, 1)
	case stats.Rate:
		if check, ok := entry.Tags.Get("check"); ok {
			return o.client.Count(
				checkToString(check, entry.Value),
				1,
				tagList,
				1,
			)
		}
		return o.client.Count(entry.Metric.Name, int64(entry.Value), tagList, 1)
	default:
		return fmt.Errorf("unsupported metric type %s", entry.Metric.Type)
	}
}

func checkToString(check string, value float64) string {
	label := "pass"
	if value == 0 {
		label = "fail"
	}
	return "check." + check + "." + label
}

// Description returns a human-readable description of the output.
func (o *Output) Description() string {
	return fmt.Sprintf("statsd (%s)", o.config.Addr.String)
}

// Start tries to open a connection to specified statsd service and starts the goroutine for
// metric flushing.
func (o *Output) Start() error {
	o.logger.Debug("Starting...")

	var err error
	if address := o.config.Addr.String; address == "" {
		err = fmt.Errorf(
			"connection string is invalid. Received: \"%+s\"",
			address,
		)
		o.logger.Error(err)

		return err
	}

	o.client, err = statsd.NewBuffered(o.config.Addr.String, int(o.config.BufferSize.Int64))

	if err != nil {
		o.logger.Errorf("Couldn't make buffered client, %s", err)
		return err
	}

	if namespace := o.config.Namespace.String; namespace != "" {
		o.client.Namespace = namespace
	}

	pf, err := output.NewPeriodicFlusher(o.config.PushInterval.TimeDuration(), o.flushMetrics)
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
	return o.client.Close()
}

func (o *Output) flushMetrics() {
	samples := o.GetBufferedSamples()
	start := time.Now()
	var count int
	var errorCount int
	for _, sc := range samples {
		samples := sc.GetSamples()
		count += len(samples)
		o.logger.
			WithField("samples", len(samples)).
			Debug("Pushing metrics to server")

		for _, entry := range samples {
			if err := o.dispatch(entry); err != nil {
				// No need to return error if just one metric didn't go through
				o.logger.WithError(err).Debugf("Error while sending metric %s", entry.Metric.Name)
				errorCount++
			}
		}
	}

	if count > 0 {
		if errorCount != 0 {
			o.logger.Warnf("Couldn't send %d out of %d metrics. Enable verbose logging with --verbose to see individual errors",
				errorCount, count)
		}
		if err := o.client.Flush(); err != nil {
			o.logger.
				WithError(err).
				Error("Couldn't flush a batch")
		}
		o.logger.WithField("t", time.Since(start)).WithField("count", count).Debug("Wrote metrics to statsd")
	}
}
