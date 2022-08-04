package v1

import (
	"encoding/json"
	"net/http"
	"time"

	"go.k6.io/k6/api/common"
)

func handleGetMetrics(rw http.ResponseWriter, r *http.Request) {
	engine := common.GetEngine(r.Context())

	var t time.Duration
	if engine.ExecutionScheduler != nil {
		t = engine.ExecutionScheduler.GetState().GetCurrentTestRunDuration()
	}

	engine.MetricsEngine.MetricsLock.Lock()
	metrics := newMetricsJSONAPI(engine.MetricsEngine.ObservedMetrics, t)
	engine.MetricsEngine.MetricsLock.Unlock()

	data, err := json.Marshal(metrics)
	if err != nil {
		apiError(rw, "Encoding error", err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = rw.Write(data)
}

func handleGetMetric(rw http.ResponseWriter, r *http.Request, id string) {
	engine := common.GetEngine(r.Context())

	var t time.Duration
	if engine.ExecutionScheduler != nil {
		t = engine.ExecutionScheduler.GetState().GetCurrentTestRunDuration()
	}

	engine.MetricsEngine.MetricsLock.Lock()
	metric, ok := engine.MetricsEngine.ObservedMetrics[id]
	if !ok {
		engine.MetricsEngine.MetricsLock.Unlock()
		apiError(rw, "Not Found", "No metric with that ID was found", http.StatusNotFound)
		return
	}
	wrappedMetric := newMetricEnvelope(metric, t)
	engine.MetricsEngine.MetricsLock.Unlock()

	data, err := json.Marshal(wrappedMetric)
	if err != nil {
		apiError(rw, "Encoding error", err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = rw.Write(data)
}
