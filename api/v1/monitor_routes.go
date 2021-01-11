/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2021 Load Impact
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
	"strconv"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/loadimpact/k6/api/common"
	dto "github.com/prometheus/client_model/go"
)

func handleGetMonitor(rw http.ResponseWriter, r *http.Request, p httprouter.Params) {
	engine := common.GetEngine(r.Context())

	var t time.Duration
	if engine.ExecutionScheduler != nil {
		t = engine.ExecutionScheduler.GetState().GetCurrentTestRunDuration()
	}

	metrics := make([]dto.MetricFamily, 0)
	for _, m := range engine.Metrics {
		metrics = append(metrics, newMetricFamily(m, t)...)
	}

	data, err := marshallMetricFamily(metrics)
	if err != nil {
		apiError(rw, "Encoding error", err.Error(), http.StatusInternalServerError)
		return
	}
	rw.Header().Add("Content-Length", strconv.Itoa(len(data)))
	_, _ = rw.Write(data)
}
