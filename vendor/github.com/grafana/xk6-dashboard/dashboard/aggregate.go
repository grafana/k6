// SPDX-FileCopyrightText: 2023 Raintank, Inc. dba Grafana Labs
//
// SPDX-License-Identifier: AGPL-3.0-only

package dashboard

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/tidwall/gjson"
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/output"
)

const gzSuffix = ".gz"

type aggregator struct {
	registry *registry
	buffer   *output.SampleBuffer

	input   io.ReadCloser
	writer  io.WriteCloser
	encoder *json.Encoder

	logger logrus.FieldLogger

	options *options

	cumulative *meter

	timestamp time.Time

	once sync.Once

	seenMetrics []string
}

func closer(what io.Closer, logger logrus.FieldLogger) {
	if closeErr := what.Close(); closeErr != nil {
		logger.Error(closeErr)
	}
}

func aggregate(input, output string, opts *options, proc *process) error {
	agg := new(aggregator)

	agg.registry = newRegistry()
	agg.options = opts
	agg.logger = proc.logger
	agg.seenMetrics = make([]string, 0)

	var inputFile, outputFile afero.File
	var err error

	if inputFile, err = proc.fs.Open(input); err != nil {
		return err
	}

	agg.input = inputFile

	defer closer(inputFile, proc.logger)

	if strings.HasSuffix(input, gzSuffix) {
		if agg.input, err = gzip.NewReader(inputFile); err != nil {
			return err
		}

		defer closer(agg.input, proc.logger)
	}

	if outputFile, err = proc.fs.Create(output); err != nil {
		return err
	}

	agg.writer = outputFile

	defer closer(outputFile, proc.logger)

	if strings.HasSuffix(output, gzSuffix) {
		agg.writer = gzip.NewWriter(outputFile)

		defer closer(agg.writer, proc.logger)
	}

	agg.encoder = json.NewEncoder(agg.writer)

	return agg.run()
}

func (agg *aggregator) run() error {
	param := new(paramData)

	param.Period = time.Duration(agg.options.Period.Milliseconds())

	agg.fireEvent(paramEvent, param)

	scanner := bufio.NewScanner(agg.input)

	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		if err := agg.processLine(scanner.Bytes()); err != nil {
			return err
		}
	}

	now := agg.timestamp

	agg.updateAndSend(nil, newMeter(agg.options.Period, now, agg.options.Tags), stopEvent, now)

	return nil
}

func (agg *aggregator) addMetricSamples(samples []metrics.SampleContainer) {
	firstTime := samples[0].GetSamples()[0].Time

	agg.once.Do(func() {
		agg.cumulative = newMeter(0, firstTime, agg.options.Tags)
		agg.timestamp = firstTime
		agg.buffer = new(output.SampleBuffer)

		agg.updateAndSend(
			nil,
			newMeter(agg.options.Period, firstTime, agg.options.Tags),
			startEvent,
			firstTime,
		)
	})

	if firstTime.Sub(agg.timestamp) > agg.options.Period {
		agg.flush()
		agg.timestamp = firstTime
	}

	agg.buffer.AddMetricSamples(samples)
}

func (agg *aggregator) flush() {
	flushed := agg.buffer.GetBufferedSamples()
	if len(flushed) == 0 {
		return
	}

	samples := flushed[len(flushed)-1].GetSamples()
	now := samples[len(samples)-1].Time

	agg.updateAndSend(
		flushed,
		newMeter(agg.options.Period, now, agg.options.Tags),
		snapshotEvent,
		now,
	)
	agg.updateAndSend(flushed, agg.cumulative, cumulativeEvent, now)
}

func (agg *aggregator) updateAndSend(
	containers []metrics.SampleContainer,
	met *meter,
	event string,
	now time.Time,
) {
	data, err := met.update(containers, now)
	if err != nil {
		agg.logger.WithError(err).Warn("Error while processing samples")

		return
	}

	newbies, updated := met.newbies(agg.seenMetrics)
	if len(newbies) != 0 {
		agg.seenMetrics = updated
		agg.fireEvent(metricEvent, newbies)
	}

	agg.fireEvent(event, data)
}

func (agg *aggregator) fireEvent(event string, data interface{}) {
	if err := agg.encoder.Encode(recorderEnvelope{Name: event, Data: data}); err != nil {
		agg.logger.Warn(err)
	}
}

func (agg *aggregator) processLine(data []byte) error {
	typ := gjson.GetBytes(data, "type").String()

	if typ == typeMetric {
		return agg.processMetric(data)
	}

	if typ == typePoint {
		return agg.processPoint(data)
	}

	return nil
}

func (agg *aggregator) processMetric(data []byte) error {
	var metricType metrics.MetricType

	err := metricType.UnmarshalText([]byte(gjson.GetBytes(data, "data.type").String()))
	if err != nil {
		return err
	}

	var valueType metrics.ValueType

	err = valueType.UnmarshalText([]byte(gjson.GetBytes(data, "data.contains").String()))
	if err != nil {
		return err
	}

	name := gjson.GetBytes(data, "data.name").String()

	tres := gjson.GetBytes(data, "data.thresholds").Array()

	thresholds := make([]string, 0, len(tres))

	for _, res := range tres {
		thresholds = append(thresholds, res.String())
	}

	_, err = agg.registry.getOrNew(name, metricType, valueType, thresholds)

	return err
}

func (agg *aggregator) processPoint(data []byte) error {
	timestamp := gjson.GetBytes(data, "data.time").Time()
	name := gjson.GetBytes(data, "metric").String()

	metric := agg.registry.Get(name)
	if metric == nil {
		return fmt.Errorf("%w: %s", errUnknownMetric, name)
	}

	tags := agg.tagSetFrom(gjson.GetBytes(data, "data.tags"))

	sample := metrics.Sample{ //nolint:exhaustruct
		Time:  timestamp,
		Value: gjson.GetBytes(data, "data.value").Float(),
		TimeSeries: metrics.TimeSeries{ //nolint:exhaustruct
			Metric: metric,
			Tags:   tags,
		},
	}

	container := metrics.ConnectedSamples{ //nolint:exhaustruct
		Samples: []metrics.Sample{sample},
		Time:    sample.Time,
		Tags:    tags,
	}

	agg.addMetricSamples([]metrics.SampleContainer{container})

	return nil
}

func (agg *aggregator) tagSetFrom(res gjson.Result) *metrics.TagSet {
	asMap := res.Map()
	if len(asMap) == 0 {
		return nil
	}

	set := agg.registry.Registry.RootTagSet()

	for key, value := range asMap {
		set = set.With(key, value.String())
	}

	return set
}

var errUnknownMetric = errors.New("unknown metric")

const (
	typeMetric = "Metric"
	typePoint  = "Point"
)
