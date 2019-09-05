package influxdb

import (
	"io"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/loadimpact/k6/stats"
)

func benchmarkInfluxdb(b *testing.B, t time.Duration) {
	testCollectorCycle(b, func(rw http.ResponseWriter, r *http.Request) {
		for {
			time.Sleep(t)
			m, _ := io.CopyN(ioutil.Discard, r.Body, 1<<18) // read 1/4 mb a time
			if m == 0 {
				break
			}
		}
		rw.WriteHeader(204)
	}, func(tb testing.TB, c *Collector) {
		b = tb.(*testing.B)
		b.ResetTimer()

		var samples = make(stats.Samples, 10)
		for i := 0; i < len(samples); i++ {
			samples[i] = stats.Sample{
				Metric: stats.New("testGauge", stats.Gauge),
				Time:   time.Now(),
				Tags: stats.NewSampleTags(map[string]string{
					"something": "else",
					"VU":        "21",
					"else":      "something",
				}),
				Value: 2.0,
			}
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			c.Collect([]stats.SampleContainer{samples})
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
