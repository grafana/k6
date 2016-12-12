package v2

import (
	"encoding/json"
	"github.com/loadimpact/k6/api/common"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
	"github.com/manyminds/api2go/jsonapi"
	"github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newRequestWithEngine(engine *lib.Engine, method, target string, body io.Reader) *http.Request {
	r := httptest.NewRequest(method, target, body)
	return r.WithContext(common.WithEngine(r.Context(), engine))
}

func TestGetStatus(t *testing.T) {
	engine, err := lib.NewEngine(nil)
	assert.NoError(t, err)

	rw := httptest.NewRecorder()
	NewHandler().ServeHTTP(rw, newRequestWithEngine(engine, "GET", "/v2/status", nil))
	res := rw.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode)

	t.Run("document", func(t *testing.T) {
		var doc jsonapi.Document
		assert.NoError(t, json.Unmarshal(rw.Body.Bytes(), &doc))
		if !assert.NotNil(t, doc.Data.DataObject) {
			return
		}
		assert.Equal(t, "status", doc.Data.DataObject.Type)
	})

	t.Run("status", func(t *testing.T) {
		var status Status
		assert.NoError(t, jsonapi.Unmarshal(rw.Body.Bytes(), &status))
		assert.True(t, status.Running.Valid)
		assert.True(t, status.Tainted.Valid)
		assert.True(t, status.VUs.Valid)
		assert.True(t, status.VUsMax.Valid)
	})
}

func TestGetMetrics(t *testing.T) {
	engine, err := lib.NewEngine(nil)
	assert.NoError(t, err)

	engine.Metrics = map[*stats.Metric]stats.Sink{
		&stats.Metric{
			Name:     "my_metric",
			Type:     stats.Trend,
			Contains: stats.Time,
		}: &stats.TrendSink{},
	}

	rw := httptest.NewRecorder()
	NewHandler().ServeHTTP(rw, newRequestWithEngine(engine, "GET", "/v2/metrics", nil))
	res := rw.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode)

	t.Run("document", func(t *testing.T) {
		var doc jsonapi.Document
		assert.NoError(t, json.Unmarshal(rw.Body.Bytes(), &doc))
		if !assert.NotNil(t, doc.Data.DataArray) {
			return
		}
		assert.Equal(t, "metrics", doc.Data.DataArray[0].Type)
	})

	t.Run("metrics", func(t *testing.T) {
		var metrics []Metric
		assert.NoError(t, jsonapi.Unmarshal(rw.Body.Bytes(), &metrics))
		if !assert.Len(t, metrics, 1) {
			return
		}
		assert.Equal(t, "my_metric", metrics[0].Name)
		assert.True(t, metrics[0].Type.Valid)
		assert.Equal(t, stats.Trend, metrics[0].Type.Type)
		assert.True(t, metrics[0].Contains.Valid)
		assert.Equal(t, stats.Time, metrics[0].Contains.Type)
	})
}

func TestGetMetric(t *testing.T) {
	engine, err := lib.NewEngine(nil)
	assert.NoError(t, err)

	engine.Metrics = map[*stats.Metric]stats.Sink{
		&stats.Metric{
			Name:     "my_metric",
			Type:     stats.Trend,
			Contains: stats.Time,
		}: &stats.TrendSink{},
	}

	t.Run("nonexistent", func(t *testing.T) {
		rw := httptest.NewRecorder()
		NewHandler().ServeHTTP(rw, newRequestWithEngine(engine, "GET", "/v2/metrics/notreal", nil))
		res := rw.Result()
		assert.Equal(t, http.StatusNotFound, res.StatusCode)
	})

	t.Run("real", func(t *testing.T) {
		rw := httptest.NewRecorder()
		NewHandler().ServeHTTP(rw, newRequestWithEngine(engine, "GET", "/v2/metrics/my_metric", nil))
		res := rw.Result()
		assert.Equal(t, http.StatusOK, res.StatusCode)

		t.Run("document", func(t *testing.T) {
			var doc jsonapi.Document
			assert.NoError(t, json.Unmarshal(rw.Body.Bytes(), &doc))
			if !assert.NotNil(t, doc.Data.DataObject) {
				return
			}
			assert.Equal(t, "metrics", doc.Data.DataObject.Type)
		})

		t.Run("metric", func(t *testing.T) {
			var metric Metric
			assert.NoError(t, jsonapi.Unmarshal(rw.Body.Bytes(), &metric))
			assert.Equal(t, "my_metric", metric.Name)
			assert.True(t, metric.Type.Valid)
			assert.Equal(t, stats.Trend, metric.Type.Type)
			assert.True(t, metric.Contains.Valid)
			assert.Equal(t, stats.Time, metric.Contains.Type)
		})
	})
}
