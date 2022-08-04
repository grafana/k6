package json

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/klauspost/compress/gzip"
	"github.com/mailru/easyjson/jwriter"
	"github.com/sirupsen/logrus"

	"go.k6.io/k6/metrics"
	"go.k6.io/k6/output"
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
	out         io.Writer
	closeFn     func() error
	seenMetrics map[string]struct{}
	thresholds  map[string]metrics.Thresholds
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
		o.closeFn = func() error {
			return w.Flush()
		}
		o.out = w
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
			o.out = outfile
		} else {
			o.closeFn = func() error {
				_ = w.Flush()
				return logfile.Close()
			}
			o.out = logfile
		}
	}

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
func (o *Output) SetThresholds(thresholds map[string]metrics.Thresholds) {
	if len(thresholds) == 0 {
		return
	}
	o.thresholds = make(map[string]metrics.Thresholds, len(thresholds))
	for name, t := range thresholds {
		o.thresholds[name] = t
	}
}

func (o *Output) flushMetrics() {
	samples := o.GetBufferedSamples()
	start := time.Now()
	var count int
	jw := new(jwriter.Writer)
	for _, sc := range samples {
		samples := sc.GetSamples()
		count += len(samples)
		for _, sample := range samples {
			sample := sample
			o.handleMetric(sample.Metric, jw)
			wrapSample(sample).MarshalEasyJSON(jw)
			jw.RawByte('\n')
		}
	}

	if _, err := jw.DumpTo(o.out); err != nil {
		// Skip metric if it can't be made into JSON or envelope is null.
		o.logger.WithError(err).Error("Sample couldn't be marshalled to JSON")
	}
	if count > 0 {
		o.logger.WithField("t", time.Since(start)).WithField("count", count).Debug("Wrote metrics to JSON")
	}
}

func (o *Output) handleMetric(m *metrics.Metric, jw *jwriter.Writer) {
	if _, ok := o.seenMetrics[m.Name]; ok {
		return
	}
	o.seenMetrics[m.Name] = struct{}{}

	wrapped := metricEnvelope{
		Type:   "Metric",
		Metric: m.Name,
	}
	wrapped.Data.Name = m.Name
	wrapped.Data.Type = m.Type
	wrapped.Data.Contains = m.Contains
	wrapped.Data.Submetrics = m.Submetrics

	if ts, ok := o.thresholds[m.Name]; ok {
		wrapped.Data.Thresholds = ts
	}

	wrapped.MarshalEasyJSON(jw)
	jw.RawByte('\n')
}
