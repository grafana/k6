package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMetric(t *testing.T) {
	t.Parallel()
	testdata := map[string]struct {
		Type     MetricType
		SinkType Sink
	}{
		"Counter": {Counter, &CounterSink{}},
		"Gauge":   {Gauge, &GaugeSink{}},
		"Trend":   {Trend, &TrendSink{}},
		"Rate":    {Rate, &RateSink{}},
	}

	for name, data := range testdata {
		name, data := name, data
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			m := newMetric("my_metric", data.Type)
			assert.Equal(t, "my_metric", m.Name)
			assert.IsType(t, data.SinkType, m.Sink)
		})
	}
}

func TestAddSubmetric(t *testing.T) {
	t.Parallel()
	testdata := map[string]struct {
		err  bool
		tags map[string]string
	}{
		"":                        {true, nil},
		"  ":                      {true, nil},
		"a":                       {false, map[string]string{"a": ""}},
		"a:1":                     {false, map[string]string{"a": "1"}},
		" a : 1 ":                 {false, map[string]string{"a": "1"}},
		"a,b":                     {false, map[string]string{"a": "", "b": ""}},
		` a:"",b: ''`:             {false, map[string]string{"a": "", "b": ""}},
		`a:1,b:2`:                 {false, map[string]string{"a": "1", "b": "2"}},
		` a : 1, b : 2 `:          {false, map[string]string{"a": "1", "b": "2"}},
		`a : '1' , b : "2"`:       {false, map[string]string{"a": "1", "b": "2"}},
		`" a" : ' 1' , b : "2 " `: {false, map[string]string{" a": " 1", "b": "2 "}}, //nolint:gocritic
	}

	for name, expected := range testdata {
		name, expected := name, expected
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			m := newMetric("metric", Trend)
			sm, err := m.AddSubmetric(name)
			if expected.err {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, sm)
			assert.EqualValues(t, expected.tags, sm.Tags.tags)
		})
	}
}
