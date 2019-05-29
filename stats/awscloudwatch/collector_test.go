package awscloudwatch

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/loadimpact/k6/stats"
	"github.com/stretchr/testify/require"
)

func TestCollector(t *testing.T) {
	t.Run("Reports samples to AWS Cloud Watch client", func(t *testing.T) {
		at, _ := time.Parse(time.RFC3339Nano, "2019-05-22T11:20:46.099126+02:00")
		samples := []stats.SampleContainer{
			stats.Sample{
				Metric: stats.New("http_req_duration", stats.Trend, stats.Time),
				Time:   at,
				Tags: stats.NewSampleTags(map[string]string{
					"group":  "",
					"method": "GET",
					"proto":  "HTTP/1.1",
					"status": "200",
				}),
				Value: 41427.507,
			},
		}

		fakeClient := newFakeCloudwatchClient()
		collector := New(fakeClient)
		ctx, cancelCollector := context.WithCancel(context.Background())

		go collector.Run(ctx)
		collector.Collect(samples)
		<-time.After(time.Second + time.Millisecond)
		cancelCollector()

		require.Equal(t, []*sample{{
			Metric: "http_req_duration",
			Time:   at,
			Value:  41427.507,
			Tags: map[string]string{
				"group":  "",
				"method": "GET",
				"proto":  "HTTP/1.1",
				"status": "200",
			},
		}}, fakeClient.reportedSamples())
	})
}

func newFakeCloudwatchClient() *fakeCloudwatchClient {
	return &fakeCloudwatchClient{}
}

type fakeCloudwatchClient struct {
	samples []*sample
	lock    sync.Mutex
}

func (fcl *fakeCloudwatchClient) reportSamples(samples []*sample) error {
	fcl.lock.Lock()
	fcl.samples = append(fcl.samples, samples...)
	fcl.lock.Unlock()

	return nil
}

func (fcl *fakeCloudwatchClient) address() string {
	return "fake"
}

func (fcl *fakeCloudwatchClient) reportedSamples() []*sample {
	fcl.lock.Lock()
	samples := fcl.samples
	fcl.lock.Unlock()

	return samples
}
