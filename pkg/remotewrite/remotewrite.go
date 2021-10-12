package remotewrite

import (
	"context"
	"fmt"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"
	"github.com/prometheus/prometheus/storage/remote"
	"github.com/sirupsen/logrus"
	"go.k6.io/k6/output"
	"go.k6.io/k6/stats"
)

type Output struct {
	config Config

	client          remote.WriteClient
	metrics         *metricsStorage
	mapping         Mapping
	periodicFlusher *output.PeriodicFlusher
	output.SampleBuffer

	logger logrus.FieldLogger
}

var _ output.Output = new(Output)

func New(params output.Params) (*Output, error) {
	config, err := GetConsolidatedConfig(params.JSONConfig, params.Environment, params.ConfigArgument)
	if err != nil {
		return nil, err
	}

	remoteConfig, err := config.ConstructRemoteConfig()
	if err != nil {
		return nil, err
	}

	// name is used to differentiate clients in metrics
	client, err := remote.NewWriteClient("xk6-prwo", remoteConfig)
	if err != nil {
		return nil, err
	}

	params.Logger.Info(fmt.Sprintf("Prometheus: configuring remote-write with %s mapping", config.Mapping.String))

	return &Output{
		client:  client,
		config:  config,
		metrics: newMetricsStorage(),
		mapping: NewMapping(config.Mapping.String),
		logger:  params.Logger,
	}, nil
}

func (*Output) Description() string {
	return "Output k6 metrics to prometheus remote-write endpoint"
}

func (o *Output) Start() error {
	if periodicFlusher, err := output.NewPeriodicFlusher(time.Duration(o.config.FlushPeriod.Duration), o.flush); err != nil {
		return err
	} else {
		o.periodicFlusher = periodicFlusher
	}
	o.logger.Debug("Prometheus: starting remote-write")

	return nil
}

func (o *Output) Stop() error {
	o.logger.Debug("Prometheus: stopping remote-write")
	o.periodicFlusher.Stop()
	return nil
}

func (o *Output) flush() {
	start := time.Now()
	defer func() {
		flushDuration := time.Since(start)
		if flushDuration > time.Duration(o.config.FlushPeriod.Duration) {
			// There is no intermediary storage so warn if remote write endpoint becomes too slow
			o.logger.Warn(fmt.Sprintf("Remote write took %s while flush period is %s. Some samples may be dropped.",
				flushDuration.String(), o.config.FlushPeriod.String()))
		}
	}()

	samplesContainers := o.GetBufferedSamples()

	// Remote write endpoint accepts TimeSeries structure defined in gRPC. It must:
	// a) contain Labels array
	// b) have a __name__ label: without it, metric might be unquerable or even rejected
	// as a metric without a name. This behaviour depends on underlying storage used.
	// c) not have duplicate timestamps within 1 timeseries, see https://github.com/prometheus/prometheus/issues/9210
	// Prometheus write handler processes only some fields as of now, so here we'll add only them.
	promTimeSeries := o.convertToTimeSeries(samplesContainers)

	o.logger.Debug("Number of time series: ", len(promTimeSeries))

	req := prompb.WriteRequest{
		Timeseries: promTimeSeries,
	}

	if buf, err := proto.Marshal(&req); err != nil {
		o.logger.WithError(err).Fatal("Failed to marshal timeseries")
	} else {
		encoded := snappy.Encode(nil, buf) // this call can panic

		if err = o.client.Store(context.Background(), encoded); err != nil {
			o.logger.Error("Failed to store timeseries")
		}
	}
}

func (o *Output) convertToTimeSeries(samplesContainers []stats.SampleContainer) []prompb.TimeSeries {
	promTimeSeries := make([]prompb.TimeSeries, 0)

	for _, samplesContainer := range samplesContainers {
		samples := samplesContainer.GetSamples()

		for _, sample := range samples {
			// Prometheus remote write treats each label array in TimeSeries as the same
			// for all Samples in those TimeSeries (https://github.com/prometheus/prometheus/blob/03d084f8629477907cab39fc3d314b375eeac010/storage/remote/write_handler.go#L75).
			// But K6 metrics can have different tags per each Sample so in order not to
			// lose info in tags or assign tags wrongly, let's store each Sample in a different TimeSeries, for now.
			// This approach also allows to avoid hard to replicate issues with duplicate timestamps.

			labels, err := tagsToPrometheusLabels(sample.Tags, o.config)
			if err != nil {
				o.logger.Error(err)
			}

			if newts, err := o.metrics.transform(o.mapping, sample, labels); err != nil {
				o.logger.Error(err)
			} else {
				promTimeSeries = append(promTimeSeries, newts...)
			}
		}

		// Do not blow up if remote endpoint is overloaded and responds too slowly.
		// TODO: consider other approaches
		if len(promTimeSeries) > 15000 {
			break
		}
	}

	return promTimeSeries
}
