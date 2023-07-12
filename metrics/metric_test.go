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
		"Trend":   {Trend, NewTrendSink()},
		"Rate":    {Rate, &RateSink{}},
	}

	for name, data := range testdata {
		name, data := name, data
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := NewRegistry()
			m := r.newMetric("my_metric", data.Type)
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

			r := NewRegistry()
			m := r.MustNewMetric("metric", Trend)
			sm, err := m.AddSubmetric(name)
			if expected.err {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, sm)
			assert.EqualValues(t, expected.tags, sm.Tags.Map())
		})
	}
}

func TestParseMetricName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                 string
		metricNameExpression string
		wantMetricName       string
		wantTags             []string
		wantErr              bool
	}{
		{
			name:                 "metric name without tags",
			metricNameExpression: "test_metric",
			wantMetricName:       "test_metric",
			wantErr:              false,
		},
		{
			name:                 "metric name with single tag",
			metricNameExpression: "test_metric{abc:123}",
			wantMetricName:       "test_metric",
			wantTags:             []string{"abc:123"},
			wantErr:              false,
		},
		{
			name:                 "metric name with multiple tags",
			metricNameExpression: "test_metric{abc:123,easyas:doremi}",
			wantMetricName:       "test_metric",
			wantTags:             []string{"abc:123", "easyas:doremi"},
			wantErr:              false,
		},
		{
			name:                 "metric name with multiple spaced tags",
			metricNameExpression: "test_metric{abc:123, easyas:doremi}",
			wantMetricName:       "test_metric",
			wantTags:             []string{"abc:123", "easyas:doremi"},
			wantErr:              false,
		},
		{
			name:                 "metric name with group tag",
			metricNameExpression: "test_metric{group:::mygroup}",
			wantMetricName:       "test_metric",
			wantTags:             []string{"group:::mygroup"},
			wantErr:              false,
		},
		{
			name:                 "metric name with valid name and repeated curly braces tokens in tags definition",
			metricNameExpression: "http_req_duration{name:http://${}.com}",
			wantMetricName:       "http_req_duration",
			wantTags:             []string{"name:http://${}.com"},
			wantErr:              false,
		},
		{
			name:                 "metric name with valid name and repeated curly braces and colon tokens in tags definition",
			metricNameExpression: "http_req_duration{name:http://${}.com,url:ssh://github.com:grafana/k6}",
			wantMetricName:       "http_req_duration",
			wantTags:             []string{"name:http://${}.com", "url:ssh://github.com:grafana/k6"},
			wantErr:              false,
		},
		{
			name:                 "metric name with tag definition missing `:value`",
			metricNameExpression: "test_metric{easyas}",
			wantErr:              true,
		},
		{
			name:                 "metric name with tag definition missing value",
			metricNameExpression: "test_metric{easyas:}",
			wantErr:              true,
		},
		{
			name:                 "metric name with mixed valid and invalid tag definitions",
			metricNameExpression: "test_metric{abc:123,easyas:}",
			wantErr:              true,
		},
		{
			name:                 "metric name with valid name and unmatched opening tags definition token",
			metricNameExpression: "test_metric{abc:123,easyas:doremi",
			wantErr:              true,
		},
		{
			name:                 "metric name with valid name and unmatched closing tags definition token",
			metricNameExpression: "test_metricabc:123,easyas:doremi}",
			wantErr:              true,
		},
		{
			name:                 "metric name with valid name and invalid starting tags definition token",
			metricNameExpression: "test_metric}abc:123,easyas:doremi}",
			wantErr:              true,
		},
		{
			name:                 "metric name with valid name and invalid curly braces in tags definition",
			metricNameExpression: "test_metric}abc{bar",
			wantErr:              true,
		},
		{
			name:                 "metric name with valid name and trailing characters after closing curly brace in tags definition",
			metricNameExpression: "test_metric{foo:ba}r",
			wantErr:              true,
		},
	}
	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotMetricName, gotTags, gotErr := ParseMetricName(tt.metricNameExpression)

			assert.Equal(t,
				gotErr != nil, tt.wantErr,
				"ParseMetricName() error = %v, wantErr %v", gotErr, tt.wantErr,
			)

			if gotErr != nil {
				assert.ErrorIs(t,
					gotErr, ErrMetricNameParsing,
					"ParseMetricName() error chain should contain ErrMetricNameParsing",
				)
			}

			assert.Equal(t,
				gotMetricName, tt.wantMetricName,
				"ParseMetricName() gotMetricName = %v, want %v", gotMetricName, tt.wantMetricName,
			)

			assert.Equal(t,
				gotTags, tt.wantTags,
				"ParseMetricName() gotTags = %v, want %v", gotTags, tt.wantTags,
			)
		})
	}
}
