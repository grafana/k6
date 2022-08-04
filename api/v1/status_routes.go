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

	status := newStatusJSONAPIFromEngine(engine)
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

	var statusEnvelop StatusJSONAPI
	if err = json.Unmarshal(body, &statusEnvelop); err != nil {
		apiError(rw, "Invalid data", err.Error(), http.StatusBadRequest)
		return
	}

	status := statusEnvelop.Status()

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

	data, err := json.Marshal(newStatusJSONAPIFromEngine(engine))
	if err != nil {
		apiError(rw, "Encoding error", err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = rw.Write(data)
}
