package testutil

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/stats"
	"github.com/loadimpact/k6/stats/statsd/common"
	"github.com/stretchr/testify/require"
	null "gopkg.in/guregu/null.v3"
)

// BaseTest is a helper function to test statsd/datadog collector throughtly
func BaseTest(t *testing.T,
	getCollector func(common.Config) (*common.Collector, error),
	checkResult func(t *testing.T, samples []stats.SampleContainer, expectedOutput, output string),
) {
	t.Helper()
	var (
		testNamespace = "testing.things." // to be dynamic
	)

	addr, err := net.ResolveUDPAddr("udp", "localhost:0")
	require.NoError(t, err)
	listener, err := net.ListenUDP("udp", addr) // we want to listen on a random port
	require.NoError(t, err)
	var ch = make(chan string, 20)
	var end = make(chan struct{})
	defer close(end)

	go func() {
		defer close(ch)
		var buf [4096]byte
		for {
			select {
			case <-end:
				return
			default:
				n, _, err := listener.ReadFromUDP(buf[:])
				require.NoError(t, err)
				ch <- string(buf[:n])
			}
		}
	}()
	var baseConfig = common.NewConfig().Apply(common.Config{
		Addr:         null.StringFrom(listener.LocalAddr().String()),
		Namespace:    null.StringFrom(testNamespace),
		BufferSize:   null.IntFrom(5),
		PushInterval: types.NullDurationFrom(time.Millisecond * 10),
	})

	collector, err := getCollector(baseConfig)
	require.NoError(t, err)
	require.NoError(t, collector.Init())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go collector.Run(ctx)
	newSample := func(m *stats.Metric, value float64, tags map[string]string) stats.Sample {
		return stats.Sample{Time: time.Now(),
			Metric: m, Value: value, Tags: stats.IntoSampleTags(&tags)}
	}

	myCounter := stats.New("my_counter", stats.Counter)
	myGauge := stats.New("my_gauge", stats.Gauge)
	myTrend := stats.New("my_trend", stats.Trend)
	myRate := stats.New("my_rate", stats.Rate)
	myCheck := stats.New("my_check", stats.Rate)
	var testMatrix = []struct {
		input  []stats.SampleContainer
		output string
	}{
		{
			input: []stats.SampleContainer{
				newSample(myCounter, 12, map[string]string{
					"tag1": "value1",
					"tag3": "value3",
				}),
			},
			output: "testing.things.my_counter:12|c",
		},
		{
			input: []stats.SampleContainer{
				newSample(myGauge, 13, map[string]string{
					"tag1": "value1",
					"tag3": "value3",
				}),
			},
			output: "testing.things.my_gauge:13.000000|g",
		},
		{
			input: []stats.SampleContainer{
				newSample(myTrend, 14, map[string]string{
					"tag1": "value1",
					"tag3": "value3",
				}),
			},
			output: "testing.things.my_trend:14.000000|ms",
		},
		{
			input: []stats.SampleContainer{
				newSample(myRate, 15, map[string]string{
					"tag1": "value1",
					"tag3": "value3",
				}),
			},
			output: "testing.things.my_rate:15|c",
		},
		{
			input: []stats.SampleContainer{
				newSample(myCheck, 16, map[string]string{
					"tag1":  "value1",
					"tag3":  "value3",
					"check": "max<100",
				}),
				newSample(myCheck, 0, map[string]string{
					"tag1":  "value1",
					"tag3":  "value3",
					"check": "max>100",
				}),
			},
			output: "testing.things.check.max<100.pass:1|c\ntesting.things.check.max>100.fail:1|c",
		},
	}
	for _, test := range testMatrix {
		collector.Collect(test.input)
		time.Sleep((time.Duration)(baseConfig.PushInterval.Duration))
		output := <-ch
		checkResult(t, test.input, test.output, output)
	}
}
