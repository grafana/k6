package writer

import (
	"context"
	"net/url"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	promConfig "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
	"github.com/prometheus/prometheus/storage/remote"
	"github.com/sirupsen/logrus"
	"go.k6.io/k6/output"
	"go.k6.io/k6/stats"
)

type Writer struct {
	client  remote.WriteClient
	samples []stats.Sample
	flusher *output.PeriodicFlusher
	lock    sync.Mutex
	logger  logrus.FieldLogger
	done    chan struct{}
}

func (w *Writer) Description() string {
	return "Send k6 metrics to prometheus remote-write endpoints"
}

func (w *Writer) Start() error {
	pf, err := output.NewPeriodicFlusher(time.Second, w.flush)
	w.flusher = pf
	w.logger.WithError(err).Info("Starting remote-write...")
	return nil
}

func (w *Writer) AddMetricSamples(containers []stats.SampleContainer) {
	w.lock.Lock()
	for _, container := range containers {
		w.samples = append(w.samples, container.GetSamples()...)
	}
	w.lock.Unlock()
}

func (w *Writer) Stop() error {
	w.flusher.Stop()
	return nil
}

func (w *Writer) emptyQueue() []stats.Sample {
	w.lock.Lock()
	samples := w.samples
	w.samples = nil
	w.lock.Unlock()
	return samples
}

func (w *Writer) flush() {
	samples := w.emptyQueue()
	// TODO unify the loop?
	formatted, err := w.transform(samples)
	if err != nil {
		w.logger.WithError(err).Error("could not transform samples")
	}

	for _, sample := range formatted {
		buf, err := proto.Marshal(sample)
		if err := proto.Unmarshal(buf, sample); err != nil {
			w.logger.WithError(err).Info("proto unmarshal is off", sample)
		}
		if err != nil {
			w.logger.WithError(err).Fatal("could not marshal timeseries")
		}

		encoded := snappy.Encode(nil, buf)

		err = w.client.Store(context.Background(), encoded)
		if err != nil {
			w.logger.WithError(err).Info("could not store timeseries")
		}
	}
}

func (w *Writer) transform(samples []stats.Sample) ([]*prompb.TimeSeries, error) {
	var ts = map[string]*prompb.TimeSeries{}

	for _, sample := range samples {
		name, _ := sample.Tags.Get("name")
		if ts[name] == nil {
			ts[name] = &prompb.TimeSeries{
				Labels:               nil,
				Samples:              nil,
				Exemplars:            nil,
				XXX_NoUnkeyedLiteral: struct{}{},
				XXX_unrecognized:     nil,
				XXX_sizecache:        0,
			}
		}
		s := ts[name]
		s.Samples = append(s.Samples, prompb.Sample{
			Value:                sample.Value,
			Timestamp:            sample.Time.UnixNano() / int64(time.Millisecond),
			XXX_NoUnkeyedLiteral: struct{}{},
			XXX_unrecognized:     nil,
			XXX_sizecache:        0,
		})

	}

	output := []*prompb.TimeSeries{}

	for _, t := range ts {
		output = append(output, t)
		// logrus.Infof("\n%v\n\n", t)
	}

	return output, nil
}

var _ output.Output = new(Writer)

func New(p output.Params) *Writer {

	cc := promConfig.DefaultHTTPClientConfig
	cc.TLSConfig = promConfig.TLSConfig{
		InsecureSkipVerify: true,
	}
	// cc.BasicAuth = &promConfig.BasicAuth{
	// 	Username: "YOUR USERNAME",
	// 	Password: "YOUR PASSWORD",
	// }
	//u, _ := url.Parse("https://prometheus-blocks-prod-us-central1.grafana.net/api/prom/push")
	u, _ := url.Parse("http://localhost:9090/api/v1/write")
	d := model.Duration(time.Minute * 2)
	conf := &remote.ClientConfig{
		URL: &promConfig.URL{
			URL: u,
		},
		Timeout:          d,
		HTTPClientConfig: cc,
		RetryOnRateLimit: false,
	}

	client, err := remote.NewWriteClient("prw", conf)

	if err != nil {
		panic("could not create prometheus remote-write client")
	}
	return &Writer{
		client: client,
		done:   make(chan struct{}),
		logger: p.Logger,
	}
}
