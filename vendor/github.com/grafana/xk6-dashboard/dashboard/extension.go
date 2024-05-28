// SPDX-FileCopyrightText: 2021 - 2023 Iván Szkiba
// SPDX-FileCopyrightText: 2023 Raintank, Inc. dba Grafana Labs
//
// SPDX-License-Identifier: AGPL-3.0-only
// SPDX-License-Identifier: MIT

// Package dashboard contains dashboard extension code.
package dashboard

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/pkg/browser"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/output"
)

// extension holds the runtime state of the dashboard extension.
type extension struct {
	*eventSource

	assets  *assets
	proc    *process
	options *options

	buffer *output.SampleBuffer

	flusher *output.PeriodicFlusher
	noFlush atomic.Bool

	server *webServer

	cumulative *meter

	seenMetrics []string

	period time.Duration

	name string

	param *paramData
}

var _ output.Output = (*extension)(nil)

// OutputName defines the output name for dashnoard extension.
const OutputName = "web-dashboard"

// New creates new dashboard extension instance.
func New(params output.Params) (output.Output, error) {
	assets := newCustomizedAssets(new(process).fromParams(params))

	return newWithAssets(params, assets)
}

func newWithAssets(params output.Params, assets *assets) (*extension, error) {
	opts, err := getopts(params.ConfigArgument, params.Environment)
	if err != nil {
		return nil, err
	}

	offset, _ := lib.GetEndOffset(params.ExecutionPlan)
	period := opts.period(offset)

	ext := &extension{
		assets:      assets,
		proc:        &process{logger: params.Logger, fs: params.FS},
		options:     opts,
		name:        params.OutputType,
		buffer:      nil,
		server:      nil,
		flusher:     nil,
		cumulative:  nil,
		seenMetrics: nil,
		param:       newParamData(&params).withPeriod(period).withEndOffest(offset),
		period:      period,
		eventSource: new(eventSource),
	}

	return ext, nil
}

// Description returns a human-readable description of the output.
func (ext *extension) Description() string {
	if ext.options.Port < 0 {
		return ext.name
	}

	return fmt.Sprintf("%s %s", ext.name, ext.options.url())
}

// SetThresholds saves thresholds provided by k6 runtime.
func (ext *extension) SetThresholds(thresholds map[string]metrics.Thresholds) {
	ext.param.withThresholds(thresholds)
}

// Start starts metrics aggregation and event streaming.
func (ext *extension) Start() error {
	if len(ext.options.Record) != 0 {
		ext.addEventListener(newRecorder(ext.options.Record, ext.proc))
	}

	brf := newReporter(ext.options.Export, ext.assets, ext.proc)

	ext.addEventListener(brf)

	if ext.options.Port >= 0 {
		ext.server = newWebServer(ext.assets.ui, brf, ext.proc.logger)
		ext.addEventListener(ext.server)

		addr, err := ext.server.listenAndServe(ext.options.addr())
		if err != nil {
			return err
		}

		if ext.options.Port == 0 {
			ext.options.Port = addr.Port
		}

		if ext.options.Open {
			_ = browser.OpenURL(ext.options.url())
		}
	}

	ext.cumulative = newMeter(0, time.Now(), ext.options.Tags)
	ext.seenMetrics = make([]string, 0)

	if err := ext.fireStart(); err != nil {
		return err
	}

	ext.buffer = new(output.SampleBuffer)

	now := time.Now()

	ext.fireEvent(configEvent, ext.assets.config)
	ext.fireEvent(paramEvent, ext.param)

	ext.updateAndSend(nil, newMeter(ext.period, now, ext.cumulative.tags), startEvent, now)

	flusher, err := output.NewPeriodicFlusher(ext.period, ext.flush)
	if err != nil {
		return err
	}

	ext.flusher = flusher

	return nil
}

// Stop flushes any remaining metrics and stops the extension.
// k6 core will call WithStopWithTestError instead of this one.
func (ext *extension) Stop() error {
	return ext.StopWithTestError(nil)
}

// WithStopWithTestError allows output to receive the error value that the test finished with.
// Flushes any remaining metrics and stops the extension.
func (ext *extension) StopWithTestError(testRunErr error) error {
	ext.noFlush.Store(true)

	ext.flusher.Stop()

	now := time.Now()

	ext.updateAndSend(nil, newMeter(ext.period, now, ext.options.Tags), stopEvent, now)

	err := ext.fireStop(testRunErr)
	if err != nil {
		return err
	}

	if ext.server != nil {
		return ext.server.stop()
	}

	return nil
}

// AddMetricSamples adds the given metric samples to the internal buffer.
func (ext *extension) AddMetricSamples(samples []metrics.SampleContainer) {
	ext.buffer.AddMetricSamples(samples)
}

func (ext *extension) flush() {
	samples := ext.buffer.GetBufferedSamples()
	now := time.Now()

	ext.updateAndSend(samples, ext.cumulative, cumulativeEvent, now)
	ext.evaluateAndSend(ext.cumulative, now)

	if !ext.noFlush.Load() { // skip the last fraction period for sanpshot (called when flusher stops)
		ext.updateAndSend(samples, ext.cumulative.toSnapshot(ext.period, now), snapshotEvent, now)
	}
}

func (ext *extension) updateAndSend(
	containers []metrics.SampleContainer,
	met *meter,
	event string,
	now time.Time,
) {
	data, err := met.update(containers, now)
	if err != nil {
		ext.proc.logger.WithError(err).Warn("Error while processing samples")

		return
	}

	newbies, updated := met.newbies(ext.seenMetrics)
	if len(newbies) != 0 {
		ext.seenMetrics = updated
		ext.fireEvent(metricEvent, newbies)
	}

	ext.fireEvent(event, data)
}

func (ext *extension) evaluateAndSend(met *meter, now time.Time) {
	failures := met.evaluate(now)
	if len(failures) > 0 {
		ext.fireEvent(thresholdEvent, failures)
	}
}

type paramData struct {
	Thresholds map[string][]string `json:"thresholds,omitempty"`
	Scenarios  []string            `json:"scenarios,omitempty"`
	EndOffset  time.Duration       `json:"endOffset,omitempty"`
	Period     time.Duration       `json:"period,omitempty"`
	Tags       []string            `json:"tags,omitempty"`
	ScriptPath string              `json:"scriptPath,omitempty"`
	Aggregates map[string][]string `json:"aggregates,omitempty"`
}

func newParamData(params *output.Params) *paramData {
	param := new(paramData)

	for name := range params.ScriptOptions.Scenarios {
		param.Scenarios = append(param.Scenarios, name)
	}

	if params.ScriptPath != nil {
		param.ScriptPath = params.ScriptPath.String()
	}

	param.Aggregates = map[string][]string{
		metrics.Counter.String(): counterAggregateNames,
		metrics.Gauge.String():   gaugeAggregateNames,
		metrics.Rate.String():    rateAggregateNames,
		metrics.Trend.String():   trendAggregateNames,
	}

	return param
}

func (param *paramData) withTags(tags []string) *paramData {
	param.Tags = tags

	return param
}

func (param *paramData) withThresholds(thresholds map[string]metrics.Thresholds) *paramData {
	if len(thresholds) == 0 {
		return param
	}

	param.Thresholds = make(map[string][]string, len(thresholds))

	for name, value := range thresholds {
		param.Thresholds[name] = thresholdsSources(value)
	}

	return param
}

func (param *paramData) withPeriod(period time.Duration) *paramData {
	param.Period = time.Duration(period.Milliseconds())

	return param
}

func (param *paramData) withEndOffest(offset time.Duration) *paramData {
	param.EndOffset = time.Duration(offset.Milliseconds())

	return param
}
