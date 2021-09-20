package prometheus

import (
	"context"
	"fmt"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"
	"github.com/prometheus/prometheus/storage/remote"
	"github.com/sirupsen/logrus"
	"go.k6.io/k6/output"
)

type Output struct {
	client remote.WriteClient
	config Config

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
	fmt.Printf("%+v\n", config)
	remoteConfig, err := config.ConstructRemoteConfig()
	if err != nil {
		return nil, err
	}

	client, err := remote.NewWriteClient("prw", remoteConfig)
	if err != nil {
		return nil, err
	}

	return &Output{
		client: client,
		config: config,
		logger: params.Logger,
	}, nil
}

func (*Output) Description() string {
	return "Send k6 metrics to prometheus remote-write endpoints"
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
	samplesContainers := o.GetBufferedSamples()
	promTimeSeries := newTimeSeries()

	for _, samplesContainer := range samplesContainers {
		promTimeSeries.addSamples(samplesContainer.GetSamples())
	}

	o.logger.Info("number of time series:", len(promTimeSeries.ts))

	req := prompb.WriteRequest{
		Timeseries: promTimeSeries.ts,
	}

	if buf, err := proto.Marshal(&req); err != nil {
		o.logger.WithError(err).Fatal("Failed to marshal timeseries")
	} else {
		encoded := snappy.Encode(nil, buf)

		if err = o.client.Store(context.Background(), encoded); err != nil {
			o.logger.WithError(err).Fatal("Failed to store timeseries")
		}
	}
}
