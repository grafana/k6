package influxdb

import (
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.k6.io/k6/metrics"
)

func benchmarkInfluxdb(b *testing.B, t time.Duration) {
	registry := metrics.NewRegistry()
	metric, err := registry.NewMetric("test_gauge", metrics.Gauge)
	require.NoError(b, err)
	tags := registry.RootTagSet().WithTagsFromMap(map[string]string{
		"something": "else",
		"VU":        "21",
		"else":      "something",
	})
	testOutputCycle(b, func(rw http.ResponseWriter, r *http.Request) {
		for {
			time.Sleep(t)
			m, _ := io.CopyN(io.Discard, r.Body, 1<<18) // read 1/4 mb a time
			if m == 0 {
				break
			}
		}
		rw.WriteHeader(http.StatusNoContent)
	}, func(tb testing.TB, c *Output) {
		b, _ := tb.(*testing.B)

		b.ResetTimer()

		samples := make(metrics.Samples, 10)
		for i := 0; i < len(samples); i++ {
			samples[i] = metrics.Sample{
				TimeSeries: metrics.TimeSeries{
					Metric: metric,
					Tags:   tags,
				},
				Time:  time.Now(),
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
