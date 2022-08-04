package json

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"io"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/output"
)

func getValidator(t testing.TB, expected []string) func(io.Reader) {
	return func(rawJSONLines io.Reader) {
		s := bufio.NewScanner(rawJSONLines)
		i := 0
		for s.Scan() {
			i++
			if i > len(expected) {
				t.Errorf("Read unexpected line number %d, expected only %d entries", i, len(expected))
				continue
			}
			assert.JSONEq(t, expected[i-1], string(s.Bytes()))
		}
		assert.NoError(t, s.Err())
		assert.Equal(t, len(expected), i)
	}
}

func generateTestMetricSamples(t testing.TB) ([]metrics.SampleContainer, func(io.Reader)) {
	registry := metrics.NewRegistry()

	metric1, err := registry.NewMetric("my_metric1", metrics.Gauge)
	require.NoError(t, err)

	_, err = metric1.AddSubmetric("a:1,b:2")
	require.NoError(t, err)

	metric2, err := registry.NewMetric("my_metric2", metrics.Counter, metrics.Data)
	require.NoError(t, err)

	time1 := time.Date(2021, time.February, 24, 13, 37, 10, 0, time.UTC)
	time2 := time1.Add(10 * time.Second)
	time3 := time2.Add(10 * time.Second)

	connTags := metrics.NewSampleTags(map[string]string{"key": "val"})

	samples := []metrics.SampleContainer{
		metrics.Sample{Time: time1, Metric: metric1, Value: float64(1), Tags: metrics.NewSampleTags(map[string]string{"tag1": "val1"})},
		metrics.Sample{Time: time1, Metric: metric1, Value: float64(2), Tags: metrics.NewSampleTags(map[string]string{"tag2": "val2"})},
		metrics.ConnectedSamples{Samples: []metrics.Sample{
			{Time: time2, Metric: metric2, Value: float64(3), Tags: connTags},
			{Time: time2, Metric: metric1, Value: float64(4), Tags: connTags},
		}, Time: time2, Tags: connTags},
		metrics.Sample{Time: time3, Metric: metric2, Value: float64(5), Tags: metrics.NewSampleTags(map[string]string{"tag3": "val3"})},
	}
	expected := []string{
		`{"type":"Metric","data":{"name":"my_metric1","type":"gauge","contains":"default","thresholds":["rate<0.01","p(99)<250"],"submetrics":[{"name":"my_metric1{a:1,b:2}","suffix":"a:1,b:2","tags":{"a":"1","b":"2"}}]},"metric":"my_metric1"}`,
		`{"type":"Point","data":{"time":"2021-02-24T13:37:10Z","value":1,"tags":{"tag1":"val1"}},"metric":"my_metric1"}`,
		`{"type":"Point","data":{"time":"2021-02-24T13:37:10Z","value":2,"tags":{"tag2":"val2"}},"metric":"my_metric1"}`,
		`{"type":"Metric","data":{"name":"my_metric2","type":"counter","contains":"data","thresholds":[],"submetrics":null},"metric":"my_metric2"}`,
		`{"type":"Point","data":{"time":"2021-02-24T13:37:20Z","value":3,"tags":{"key":"val"}},"metric":"my_metric2"}`,
		`{"type":"Point","data":{"time":"2021-02-24T13:37:20Z","value":4,"tags":{"key":"val"}},"metric":"my_metric1"}`,
		`{"type":"Point","data":{"time":"2021-02-24T13:37:30Z","value":5,"tags":{"tag3":"val3"}},"metric":"my_metric2"}`,
	}

	return samples, getValidator(t, expected)
}

func TestJsonOutputStdout(t *testing.T) {
	t.Parallel()

	stdout := new(bytes.Buffer)
	out, err := New(output.Params{
		Logger: testutils.NewLogger(t),
		StdOut: stdout,
	})
	require.NoError(t, err)

	setThresholds(t, out)
	require.NoError(t, out.Start())

	samples, validateResults := generateTestMetricSamples(t)
	out.AddMetricSamples(samples[:2])
	out.AddMetricSamples(samples[2:])
	require.NoError(t, out.Stop())
	validateResults(stdout)
}

func TestJsonOutputFileError(t *testing.T) {
	t.Parallel()

	stdout := new(bytes.Buffer)
	fs := afero.NewReadOnlyFs(afero.NewMemMapFs())
	out, err := New(output.Params{
		Logger:         testutils.NewLogger(t),
		StdOut:         stdout,
		FS:             fs,
		ConfigArgument: "/json-output",
	})
	require.NoError(t, err)
	assert.Error(t, out.Start())
}

func TestJsonOutputFile(t *testing.T) {
	t.Parallel()

	stdout := new(bytes.Buffer)
	fs := afero.NewMemMapFs()
	out, err := New(output.Params{
		Logger:         testutils.NewLogger(t),
		StdOut:         stdout,
		FS:             fs,
		ConfigArgument: "/json-output",
	})
	require.NoError(t, err)

	setThresholds(t, out)
	require.NoError(t, out.Start())

	samples, validateResults := generateTestMetricSamples(t)
	out.AddMetricSamples(samples[:2])
	out.AddMetricSamples(samples[2:])
	require.NoError(t, out.Stop())

	assert.Empty(t, stdout.Bytes())
	file, err := fs.Open("/json-output")
	require.NoError(t, err)
	validateResults(file)
	assert.NoError(t, file.Close())
}

func TestJsonOutputFileGzipped(t *testing.T) {
	t.Parallel()

	stdout := new(bytes.Buffer)
	fs := afero.NewMemMapFs()
	out, err := New(output.Params{
		Logger:         testutils.NewLogger(t),
		StdOut:         stdout,
		FS:             fs,
		ConfigArgument: "/json-output.gz",
	})
	require.NoError(t, err)

	setThresholds(t, out)
	require.NoError(t, out.Start())

	samples, validateResults := generateTestMetricSamples(t)
	out.AddMetricSamples(samples[:2])
	out.AddMetricSamples(samples[2:])
	require.NoError(t, out.Stop())

	assert.Empty(t, stdout.Bytes())
	file, err := fs.Open("/json-output.gz")
	require.NoError(t, err)
	reader, err := gzip.NewReader(file)
	require.NoError(t, err)
	validateResults(reader)
	assert.NoError(t, file.Close())
}

func TestWrapSampleWithSamplePointer(t *testing.T) {
	t.Parallel()
	out := wrapSample(metrics.Sample{
		Metric: &metrics.Metric{},
	})
	assert.NotEqual(t, out, (*sampleEnvelope)(nil))
}

func setThresholds(t *testing.T, out output.Output) {
	t.Helper()

	jout, ok := out.(*Output)
	require.True(t, ok)

	ts := metrics.NewThresholds([]string{"rate<0.01", "p(99)<250"})
	jout.SetThresholds(map[string]metrics.Thresholds{"my_metric1": ts})
}
