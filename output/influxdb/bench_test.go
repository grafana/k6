package influxdb

import (
	"io"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.k6.io/k6/metrics"
)

func benchmarkInfluxdb(b *testing.B, t time.Duration) {
	metric, err := metrics.NewRegistry().NewMetric("test_gauge", metrics.Gauge)
	require.NoError(b, err)

	testOutputCycle(b, func(rw http.ResponseWriter, r *http.Request) {
		for {
			time.Sleep(t)
			m, _ := io.CopyN(ioutil.Discard, r.Body, 1<<18) // read 1/4 mb a time
			if m == 0 {
				break
			}
		}
		rw.WriteHeader(204)
	}, func(tb testing.TB, c *Output) {
		b = tb.(*testing.B)
		b.ResetTimer()

		samples := make(metrics.Samples, 10)
		for i := 0; i < len(samples); i++ {
			samples[i] = metrics.Sample{
				Metric: metric,
				Time:   time.Now(),
				Tags: metrics.NewSampleTags(map[string]string{
					"something": "else",
					"VU":        "21",
					"else":      "something",
				}),
				Value: 2.0,
			}
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			c.AddMetricSamples([]metrics.SampleContainer{samples})
			time.Sleep(time.Nanosecond * 20)
		}
	})
}

func BenchmarkInfluxdb1Second(b *testing.B) {
	benchmarkInfluxdb(b, time.Second)
}

func BenchmarkInfluxdb2Second(b *testing.B) {
	benchmarkInfluxdb(b, 2*time.Second)
}

func BenchmarkInfluxdb100Milliseconds(b *testing.B) {
	benchmarkInfluxdb(b, 100*time.Millisecond)
}
