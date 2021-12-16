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
	"errors"
	"io/ioutil"
	"net/http"

	"go.k6.io/k6/api/common"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/executor"
)

func handleGetStatus(rw http.ResponseWriter, r *http.Request) {
	engine := common.GetEngine(r.Context())

	status := newStatusEnvelopFromEngine(engine)
	data, err := json.Marshal(status)
	if err != nil {
		apiError(rw, "Encoding error", err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = rw.Write(data)
}

func getFirstExternallyControlledExecutor(
	execScheduler lib.ExecutionScheduler,
) (*executor.ExternallyControlled, error) {
	executors := execScheduler.GetExecutors()
	for _, s := range executors {
		if mex, ok := s.(*executor.ExternallyControlled); ok {
			return mex, nil
		}
	}
	return nil, errors.New("an externally-controlled executor needs to be configured for live configuration updates")
}

func handlePatchStatus(rw http.ResponseWriter, r *http.Request) {
	engine := common.GetEngine(r.Context())

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		apiError(rw, "Couldn't read request", err.Error(), http.StatusBadRequest)
		return
	}

	status, err := unmarshalStatusEnvelopJSON(body)
	if err != nil {
		apiError(rw, "Invalid data", err.Error(), http.StatusBadRequest)
		return
	}

	if status.Stopped { //nolint:nestif
		engine.Stop()
	} else {
		if status.Paused.Valid {
			if err = engine.ExecutionScheduler.SetPaused(status.Paused.Bool); err != nil {
				apiError(rw, "Pause error", err.Error(), http.StatusInternalServerError)
				return
			}
		}

		if status.VUsMax.Valid || status.VUs.Valid {
			// TODO: add ability to specify the actual executor id? Though this should
			// likely be in the v2 REST API, where we could implement it in a way that
			// may allow us to eventually support other executor types.
			executor, updateErr := getFirstExternallyControlledExecutor(engine.ExecutionScheduler)
			if updateErr != nil {
				apiError(rw, "Execution config error", updateErr.Error(), http.StatusInternalServerError)
				return
			}
			newConfig := executor.GetCurrentConfig().ExternallyControlledConfigParams
			if status.VUsMax.Valid {
				newConfig.MaxVUs = status.VUsMax
			}
			if status.VUs.Valid {
				newConfig.VUs = status.VUs
			}
			if updateErr := executor.UpdateConfig(r.Context(), newConfig); updateErr != nil {
				apiError(rw, "Config update error", updateErr.Error(), http.StatusBadRequest)
				return
			}
		}
	}

	data, err := json.Marshal(newStatusEnvelopFromEngine(engine))
	if err != nil {
		apiError(rw, "Encoding error", err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = rw.Write(data)
}
