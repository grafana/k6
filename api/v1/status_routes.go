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
	"io/ioutil"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/loadimpact/k6/api/common"
	"github.com/manyminds/api2go/jsonapi"
)

func HandleGetStatus(rw http.ResponseWriter, r *http.Request, p httprouter.Params) {
	engine := common.GetEngine(r.Context())

	status := NewStatus(engine)
	data, err := jsonapi.Marshal(status)
	if err != nil {
		apiError(rw, "Encoding error", err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = rw.Write(data)
}

func HandlePatchStatus(rw http.ResponseWriter, r *http.Request, p httprouter.Params) {
	engine := common.GetEngine(r.Context())

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		apiError(rw, "Couldn't read request", err.Error(), http.StatusBadRequest)
		return
	}

	var status Status
	if err := jsonapi.Unmarshal(body, &status); err != nil {
		apiError(rw, "Invalid data", err.Error(), http.StatusBadRequest)
		return
	}

	if status.VUsMax.Valid {
		if err := engine.SetVUsMax(status.VUsMax.Int64); err != nil {
			apiError(rw, "Couldn't change cap", err.Error(), http.StatusBadRequest)
			return
		}
	}
	if status.VUs.Valid {
		if err := engine.SetVUs(status.VUs.Int64); err != nil {
			apiError(rw, "Couldn't scale", err.Error(), http.StatusBadRequest)
			return
		}
	}
	if status.Paused.Valid {
		engine.SetPaused(status.Paused.Bool)
	}

	data, err := jsonapi.Marshal(NewStatus(engine))
	if err != nil {
		apiError(rw, "Encoding error", err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = rw.Write(data)
}
