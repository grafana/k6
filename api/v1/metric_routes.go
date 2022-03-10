/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package v1

import (
	"encoding/json"
	"net/http"
	"time"

	"go.k6.io/k6/api/common"
)

func handleGetMetrics(cs *common.ControlSurface, rw http.ResponseWriter, r *http.Request) {
	var t time.Duration
	if cs.ExecutionScheduler != nil {
		t = cs.ExecutionScheduler.GetState().GetCurrentTestRunDuration()
	}

	cs.MetricsEngine.MetricsLock.Lock()
	metrics := newMetricsJSONAPI(cs.MetricsEngine.ObservedMetrics, t)
	cs.MetricsEngine.MetricsLock.Unlock()

	data, err := json.Marshal(metrics)
	if err != nil {
		apiError(rw, "Encoding error", err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = rw.Write(data)
}

func handleGetMetric(cs *common.ControlSurface, rw http.ResponseWriter, r *http.Request, id string) {
	var t time.Duration
	if cs.ExecutionScheduler != nil {
		t = cs.ExecutionScheduler.GetState().GetCurrentTestRunDuration()
	}

	cs.MetricsEngine.MetricsLock.Lock()
	metric, ok := cs.MetricsEngine.ObservedMetrics[id]
	if !ok {
		cs.MetricsEngine.MetricsLock.Unlock()
		apiError(rw, "Not Found", "No metric with that ID was found", http.StatusNotFound)
		return
	}
	wrappedMetric := newMetricEnvelope(metric, t)
	cs.MetricsEngine.MetricsLock.Unlock()

	data, err := json.Marshal(wrappedMetric)
	if err != nil {
		apiError(rw, "Encoding error", err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = rw.Write(data)
}
