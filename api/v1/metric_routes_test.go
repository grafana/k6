package v1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/internal/lib/testutils/minirunner"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
)

func TestGetMetrics(t *testing.T) {
	t.Parallel()

	testState := getTestRunState(t, lib.Options{}, &minirunner.MiniRunner{})
	testMetric, err := testState.Registry.NewMetric("my_metric", metrics.Trend, metrics.Time)
	require.NoError(t, err)
	cs := getControlSurface(t, testState)

	testMetric.Tainted = null.BoolFrom(true)
	mem := metricsObserverMock{
		ObsMetricsFunc: func() []*metrics.Metric {
			return []*metrics.Metric{testMetric}
		},
	}
	cs.MetricsEngine = mem

	rw := httptest.NewRecorder()
	NewHandler(cs).ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/v1/metrics", nil))
	res := rw.Result()
	t.Cleanup(func() {
		assert.NoError(t, res.Body.Close())
	})
	assert.Equal(t, http.StatusOK, res.StatusCode)

	t.Run("document", func(t *testing.T) {
		t.Parallel()

		var doc MetricsJSONAPI
		assert.NoError(t, json.Unmarshal(rw.Body.Bytes(), &doc))
		if !assert.NotNil(t, doc.Data) {
			return
		}
		assert.Equal(t, "metrics", doc.Data[0].Type)
	})

	t.Run("metrics", func(t *testing.T) {
		t.Parallel()

		var envelop MetricsJSONAPI
		assert.NoError(t, json.Unmarshal(rw.Body.Bytes(), &envelop))

		metricsData := envelop.Data
		if !assert.Len(t, metricsData, 1) {
			return
		}

		metric := metricsData[0].Attributes

		assert.Equal(t, "my_metric", metricsData[0].ID)
		assert.True(t, metric.Type.Valid)
		assert.Equal(t, metrics.Trend, metric.Type.Type)
		assert.True(t, metric.Contains.Valid)
		assert.Equal(t, metrics.Time, metric.Contains.Type)
		assert.True(t, metric.Tainted.Valid)
		assert.True(t, metric.Tainted.Bool)

		resMetrics := envelop.Metrics()
		assert.Len(t, resMetrics, 1)
		assert.Equal(t, resMetrics[0].Name, "my_metric")
	})
}

func TestGetMetric(t *testing.T) {
	t.Parallel()

	testState := getTestRunState(t, lib.Options{}, &minirunner.MiniRunner{})
	testMetric, err := testState.Registry.NewMetric("my_metric", metrics.Trend, metrics.Time)
	require.NoError(t, err)
	cs := getControlSurface(t, testState)

	testMetric.Tainted = null.BoolFrom(true)
	mem := metricsObserverMock{
		ObsMetricByNameFunc: func(id string) (*metrics.Metric, bool) {
			if id != "my_metric" {
				return nil, false
			}
			return testMetric, true
		},
	}
	cs.MetricsEngine = mem

	t.Run("nonexistent", func(t *testing.T) {
		t.Parallel()

		rw := httptest.NewRecorder()
		NewHandler(cs).ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/v1/metrics/notreal", nil))
		res := rw.Result()
		t.Cleanup(func() {
			assert.NoError(t, res.Body.Close())
		})
		assert.Equal(t, http.StatusNotFound, res.StatusCode)
	})

	t.Run("real", func(t *testing.T) {
		t.Parallel()

		rw := httptest.NewRecorder()
		NewHandler(cs).ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/v1/metrics/my_metric", nil))
		res := rw.Result()
		t.Cleanup(func() {
			assert.NoError(t, res.Body.Close())
		})
		assert.Equal(t, http.StatusOK, res.StatusCode)

		t.Run("document", func(t *testing.T) {
			t.Parallel()

			var doc metricJSONAPI
			assert.NoError(t, json.Unmarshal(rw.Body.Bytes(), &doc))

			assert.Equal(t, "metrics", doc.Data.Type)
		})

		t.Run("metric", func(t *testing.T) {
			var envelop metricJSONAPI

			assert.NoError(t, json.Unmarshal(rw.Body.Bytes(), &envelop))

			metric := envelop.Data.Attributes

			assert.Equal(t, "my_metric", envelop.Data.ID)
			assert.True(t, metric.Type.Valid)
			assert.Equal(t, metrics.Trend, metric.Type.Type)
			assert.True(t, metric.Contains.Valid)
			assert.Equal(t, metrics.Time, metric.Contains.Type)
			assert.True(t, metric.Tainted.Valid)
			assert.True(t, metric.Tainted.Bool)
		})
	})
}

type metricsObserverMock struct {
	ObsMetricByNameFunc func(string) (*metrics.Metric, bool)
	ObsMetricsFunc      func() []*metrics.Metric
}

func (mem metricsObserverMock) ObservedMetrics() []*metrics.Metric {
	return mem.ObsMetricsFunc()
}

func (mem metricsObserverMock) ObservedMetricByName(id string) (*metrics.Metric, bool) {
	return mem.ObsMetricByNameFunc(id)
}

func (mem metricsObserverMock) GetMetricsWithBreachedThresholdsCount() uint32 {
	panic("GetMetricsWithBreachedThresholdsCount is not expected to be called")
}
