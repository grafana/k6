package statsd

import (
	"net"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/metrics"
)

type getOutputFn func(
	logger logrus.FieldLogger,
	addr, namespace null.String,
	bufferSize null.Int,
	pushInterval types.NullDuration,
) (*Output, error)

func baseTest(t *testing.T,
	getOutput getOutputFn,
	checkResult func(t *testing.T, samples []metrics.SampleContainer, expectedOutput, output string),
) {
	t.Helper()
	testNamespace := "testing.things." // to be dynamic

	addr, err := net.ResolveUDPAddr("udp", "localhost:0")
	require.NoError(t, err)
	listener, err := net.ListenUDP("udp", addr) // we want to listen on a random port
	require.NoError(t, err)
	ch := make(chan string, 20)
	end := make(chan struct{})
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

	pushInterval := types.NullDurationFrom(time.Millisecond * 10)
	collector, err := getOutput(
		testutils.NewLogger(t),
		null.StringFrom(listener.LocalAddr().String()),
		null.StringFrom(testNamespace),
		null.IntFrom(5),
		pushInterval,
	)
	require.NoError(t, err)
	require.NoError(t, collector.Start())
	defer func() {
		require.NoError(t, collector.Stop())
	}()

	registry := metrics.NewRegistry()
	newSample := func(m *metrics.Metric, value float64, tags map[string]string) metrics.Sample {
		return metrics.Sample{
			TimeSeries: metrics.TimeSeries{
				Metric: m,
				Tags:   registry.RootTagSet().WithTagsFromMap(tags),
			},
			Time:  time.Now(),
			Value: value,
		}
	}

	myCounter, err := registry.NewMetric("my_counter", metrics.Counter)
	require.NoError(t, err)
	myGauge, err := registry.NewMetric("my_gauge", metrics.Gauge)
	require.NoError(t, err)
	myTrend, err := registry.NewMetric("my_trend", metrics.Trend)
	require.NoError(t, err)
	myRate, err := registry.NewMetric("my_rate", metrics.Rate)
	require.NoError(t, err)
	myCheck, err := registry.NewMetric("my_check", metrics.Rate)
	require.NoError(t, err)

	testMatrix := []struct {
		input  []metrics.SampleContainer
		output string
	}{
		{
			input: []metrics.SampleContainer{
				newSample(myCounter, 12, map[string]string{
					"tag1": "value1",
					"tag3": "value3",
				}),
			},
			output: "testing.things.my_counter:12|c",
		},
		{
			input: []metrics.SampleContainer{
				newSample(myGauge, 13, map[string]string{
					"tag1": "value1",
					"tag3": "value3",
				}),
			},
			output: "testing.things.my_gauge:13.000000|g",
		},
		{
			input: []metrics.SampleContainer{
				newSample(myTrend, 14, map[string]string{
					"tag1": "value1",
					"tag3": "value3",
				}),
			},
			output: "testing.things.my_trend:14.000000|ms",
		},
		{
			input: []metrics.SampleContainer{
				newSample(myRate, 15, map[string]string{
					"tag1": "value1",
					"tag3": "value3",
				}),
			},
			output: "testing.things.my_rate:15|c",
		},
		{
			input: []metrics.SampleContainer{
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
		collector.AddMetricSamples(test.input)
		time.Sleep((time.Duration)(pushInterval.Duration))
		output := <-ch
		checkResult(t, test.input, test.output, output)
	}
}
