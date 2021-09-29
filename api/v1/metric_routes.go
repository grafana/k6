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
	"net/http"
	"time"

	"github.com/manyminds/api2go/jsonapi"

	"go.k6.io/k6/api/common"
)

func handleGetMetrics(rw http.ResponseWriter, r *http.Request) {
	engine := common.GetEngine(r.Context())

	var t time.Duration
	if engine.ExecutionScheduler != nil {
		t = engine.ExecutionScheduler.GetState().GetCurrentTestRunDuration()
	}

	metrics := make([]Metric, 0)
	for _, m := range engine.Metrics {
		metrics = append(metrics, NewMetric(m, t))
	}

	data, err := jsonapi.Marshal(metrics)
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

	var metric Metric
	var found bool
	for _, m := range engine.Metrics {
		if m.Name == id {
			metric = NewMetric(m, t)
			found = true
			break
		}
	}

	if !found {
		apiError(rw, "Not Found", "No metric with that ID was found", http.StatusNotFound)
		return
	}

	data, err := jsonapi.Marshal(metric)
	if err != nil {
		apiError(rw, "Encoding error", err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = rw.Write(data)
}
