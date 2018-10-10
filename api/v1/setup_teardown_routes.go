/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2018 Load Impact
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
	"io/ioutil"
	"net/http"

	"github.com/julienschmidt/httprouter"
	"github.com/loadimpact/k6/api/common"
	"github.com/manyminds/api2go/jsonapi"
)

// NullSetupData is wrapper around null to satisfy jsonapi
type NullSetupData struct {
	SetupData
	Data interface{} `json:"data,omitempty" yaml:"data"`
}

// SetupData is just a simple wrapper to satisfy jsonapi
type SetupData struct {
	Data interface{} `json:"data" yaml:"data"`
}

// GetName is a dummy method so we can satisfy the jsonapi.EntityNamer interface
func (sd SetupData) GetName() string {
	return "setupData"
}

// GetID is a dummy method so we can satisfy the jsonapi.MarshalIdentifier interface
func (sd SetupData) GetID() string {
	return "default"
}

func handleSetupDataOutput(rw http.ResponseWriter, setupData json.RawMessage) {
	rw.Header().Set("Content-Type", "application/json")
	var err error
	var data []byte

	if setupData == nil {
		data, err = jsonapi.Marshal(NullSetupData{Data: nil})
	} else {
		data, err = jsonapi.Marshal(SetupData{setupData})
	}
	if err != nil {
		apiError(rw, "Encoding error", err.Error(), http.StatusInternalServerError)
		return
	}

	_, _ = rw.Write(data)
}

// HandleGetSetupData just returns the current JSON-encoded setup data
func HandleGetSetupData(rw http.ResponseWriter, r *http.Request, p httprouter.Params) {
	runner := common.GetEngine(r.Context()).Executor.GetRunner()
	handleSetupDataOutput(rw, runner.GetSetupData())
}

// HandleSetSetupData just parses the JSON request body and sets the result as setup data for the runner
func HandleSetSetupData(rw http.ResponseWriter, r *http.Request, p httprouter.Params) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		apiError(rw, "Error reading request body", err.Error(), http.StatusBadRequest)
		return
	}

	var data interface{}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &data); err != nil {
			apiError(rw, "Error parsing request body", err.Error(), http.StatusBadRequest)
			return
		}
	}

	runner := common.GetEngine(r.Context()).Executor.GetRunner()

	if len(body) == 0 {
		runner.SetSetupData(nil)
	} else {
		runner.SetSetupData(body)
	}

	handleSetupDataOutput(rw, runner.GetSetupData())
}

// HandleRunSetup executes the runner's Setup() method and returns the result
func HandleRunSetup(rw http.ResponseWriter, r *http.Request, p httprouter.Params) {
	engine := common.GetEngine(r.Context())
	runner := engine.Executor.GetRunner()

	if err := runner.Setup(r.Context(), engine.Samples); err != nil {
		apiError(rw, "Error executing setup", err.Error(), http.StatusInternalServerError)
		return
	}

	handleSetupDataOutput(rw, runner.GetSetupData())
}

// HandleRunTeardown executes the runner's Teardown() method
func HandleRunTeardown(rw http.ResponseWriter, r *http.Request, p httprouter.Params) {
	engine := common.GetEngine(r.Context())
	runner := common.GetEngine(r.Context()).Executor.GetRunner()

	if err := runner.Teardown(r.Context(), engine.Samples); err != nil {
		apiError(rw, "Error executing teardown", err.Error(), http.StatusInternalServerError)
	}
}
