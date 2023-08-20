package v1

import (
	"encoding/json"
	"io"
	"net/http"
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

func handleSetupDataOutput(rw http.ResponseWriter, setupData json.RawMessage) {
	rw.Header().Set("Content-Type", "application/json")
	var err error
	var data []byte

	if setupData == nil {
		data, err = json.Marshal(newSetUpJSONAPI(NullSetupData{Data: nil}))
	} else {
		data, err = json.Marshal(newSetUpJSONAPI(SetupData{setupData}))
	}
	if err != nil {
		apiError(rw, "Encoding error", err.Error(), http.StatusInternalServerError)
		return
	}

	_, _ = rw.Write(data)
}

// handleGetSetupData just returns the current JSON-encoded setup data
func handleGetSetupData(cs *ControlSurface, rw http.ResponseWriter, _ *http.Request) {
	handleSetupDataOutput(rw, cs.RunState.Runner.GetSetupData())
}

// handleSetSetupData just parses the JSON request body and sets the result as setup data for the runner
func handleSetSetupData(cs *ControlSurface, rw http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
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

	runner := cs.RunState.Runner
	if len(body) == 0 {
		runner.SetSetupData(nil)
	} else {
		runner.SetSetupData(body)
	}

	handleSetupDataOutput(rw, runner.GetSetupData())
}

// handleRunSetup executes the runner's Setup() method and returns the result
func handleRunSetup(cs *ControlSurface, rw http.ResponseWriter, r *http.Request) {
	runner := cs.RunState.Runner

	if err := cs.RunState.Runner.Setup(r.Context(), cs.Samples); err != nil {
		cs.RunState.Logger.WithError(err).Error("Error executing setup")
		apiError(rw, "Error executing setup", err.Error(), http.StatusInternalServerError)
		return
	}

	handleSetupDataOutput(rw, runner.GetSetupData())
}

// handleRunTeardown executes the runner's Teardown() method
func handleRunTeardown(cs *ControlSurface, rw http.ResponseWriter, r *http.Request) {
	if err := cs.RunState.Runner.Teardown(r.Context(), cs.Samples); err != nil {
		cs.RunState.Logger.WithError(err).Error("Error executing teardown")
		apiError(rw, "Error executing teardown", err.Error(), http.StatusInternalServerError)
	}
}
