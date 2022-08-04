package influxdb

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	client "github.com/influxdata/influxdb1-client/v2"
	"github.com/sirupsen/logrus"

	"go.k6.io/k6/metrics"
	"go.k6.io/k6/output"
)

// FieldKind defines Enum for tag-to-field type conversion
type FieldKind int

const (
	// String field (default)
	String FieldKind = iota
	// Int field
	Int
	// Float field
	Float
	// Bool field
	Bool
)

// Output is the influxdb Output struct
type Output struct {
	output.SampleBuffer

	Client    client.Client
	Config    Config
	BatchConf client.BatchPointsConfig

	logger          logrus.FieldLogger
	params          output.Params
	fieldKinds      map[string]FieldKind
	periodicFlusher *output.PeriodicFlusher
	semaphoreCh     chan struct{}
	wg              sync.WaitGroup
}

// New returns new influxdb output
func New(params output.Params) (output.Output, error) {
	return newOutput(params)
}

func newOutput(params output.Params) (*Output, error) {
	conf, err := GetConsolidatedConfig(params.JSONConfig, params.Environment, params.ConfigArgument)
	if err != nil {
		return nil, err
	}
	cl, err := MakeClient(conf)
	if err != nil {
		return nil, err
	}
	batchConf := MakeBatchConfig(conf)
	if conf.ConcurrentWrites.Int64 <= 0 {
		return nil, errors.New("influxdb's ConcurrentWrites must be a positive number")
	}
	fldKinds, err := MakeFieldKinds(conf)
	return &Output{
		params: params,
		logger: params.Logger.WithFields(logrus.Fields{
			"output": "InfluxDBv1",
		}),
		Client:      cl,
		Config:      conf,
		BatchConf:   batchConf,
		fieldKinds:  fldKinds,
		semaphoreCh: make(chan struct{}, conf.ConcurrentWrites.Int64),
		wg:          sync.WaitGroup{},
	}, err
}

func (o *Output) extractTagsToValues(tags map[string]string, values map[string]interface{}) map[string]interface{} {
	for tag, kind := range o.fieldKinds {
		if val, ok := tags[tag]; ok {
			var v interface{}
			var err error
			switch kind {
			case String:
				v = val
			case Bool:
				v, err = strconv.ParseBool(val)
			case Float:
				v, err = strconv.ParseFloat(val, 64)
			case Int:
				v, err = strconv.ParseInt(val, 10, 64)
			}
			if err == nil {
				values[tag] = v
			} else {
				values[tag] = val
			}
			delete(tags, tag)
		}
	}
	return values
}

func (o *Output) batchFromSamples(containers []metrics.SampleContainer) (client.BatchPoints, error) {
	batch, err := client.NewBatchPoints(o.BatchConf)
	if err != nil {
		return nil, fmt.Errorf("couldn't make a batch: %w", err)
	}

	type cacheItem struct {
		tags   map[string]string
		values map[string]interface{}
	}
	cache := map[*metrics.SampleTags]cacheItem{}
	for _, container := range containers {
		samples := container.GetSamples()
		for _, sample := range samples {
			var tags map[string]string
			values := make(map[string]interface{})
			if cached, ok := cache[sample.Tags]; ok {
				tags = cached.tags
				for k, v := range cached.values {
					values[k] = v
				}
			} else {
				tags = sample.Tags.CloneTags()
				o.extractTagsToValues(tags, values)
				cache[sample.Tags] = cacheItem{tags, values}
			}
			values["value"] = sample.Value
			var p *client.Point
			p, err = client.NewPoint(
				sample.Metric.Name,
				tags,
				values,
				sample.Time,
			)
			if err != nil {
				return nil, fmt.Errorf("couldn't make point from sample: %w", err)
			}
			batch.AddPoint(p)
		}
	}

	return batch, nil
}

// Description returns a human-readable description of the output.
func (o *Output) Description() string {
	return fmt.Sprintf("InfluxDBv1 (%s)", o.Config.Addr.String)
}

// Start tries to open the specified JSON file and starts the goroutine for
// metric flushing. If gzip encoding is specified, it also handles that.
func (o *Output) Start() error {
	o.logger.Debug("Starting...")
	// Try to create the database if it doesn't exist. Failure to do so is USUALLY harmless; it
	// usually means we're either a non-admin user to an existing DB or connecting over UDP.
	_, err := o.Client.Query(client.NewQuery("CREATE DATABASE "+o.BatchConf.Database, "", ""))
	if err != nil {
		o.logger.WithError(err).Debug("Couldn't create database; most likely harmless")
	}

	pf, err := output.NewPeriodicFlusher(o.Config.PushInterval.TimeDuration(), o.flushMetrics)
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
	o.wg.Wait()
	return nil
}

func (o *Output) flushMetrics() {
	samples := o.GetBufferedSamples()
	if len(samples) < 1 {
		return
	}

	o.logger.Debug("Committing...")
	o.wg.Add(1)
	o.semaphoreCh <- struct{}{}
	go func() {
		defer func() {
			<-o.semaphoreCh
			o.wg.Done()
		}()

		o.logger.WithField("samples", len(samples)).Debug("Writing...")

		batch, err := o.batchFromSamples(samples)
		if err != nil {
			o.logger.WithError(err).Error("Couldn't create batch from samples")
			return
		}

		o.logger.WithField("points", len(batch.Points())).Debug("Writing...")
		startTime := time.Now()
		if err := o.Client.Write(batch); err != nil {
			msg := "Couldn't write stats"
			if strings.Contains(err.Error(), "unauthorized access") {
				msg += ", InfluxDB v2.x isn't supported by this output, if you are using it you may consider to use the extension https://github.com/grafana/xk6-output-influxdb" //nolint:lll
			}
			o.logger.WithError(err).Error(msg)
			return
		}
		t := time.Since(startTime)
		o.logger.WithField("t", t).Debug("Batch written!")

		if t > o.Config.PushInterval.TimeDuration() {
			o.logger.WithField("t", t).
				Warn("The flush operation took higher than the expected set push interval. If you see this message multiple times then the setup or configuration need to be adjusted to achieve a sustainable rate.") //nolint:lll
		}
	}()
}
