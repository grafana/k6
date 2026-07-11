package v1

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"go.k6.io/k6/v2/errext"
	"go.k6.io/k6/v2/errext/exitcodes"
	"go.k6.io/k6/v2/internal/execution"
)

func handleGetStatus(cs *ControlSurface, rw http.ResponseWriter, _ *http.Request) {
	status := newStatusJSONAPIFromEngine(cs)
	data, err := json.Marshal(status)
	if err != nil {
		apiError(rw, "Encoding error", err.Error(), http.StatusInternalServerError)
		return
	}
	rw.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = rw.Write(data)
}

func handlePatchStatus(cs *ControlSurface, rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json; charset=utf-8")

	body, err := io.ReadAll(r.Body)
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
		execution.AbortTestRun(
			cs.RunCtx,
			errext.WithAbortReasonIfNone(
				errext.WithExitCodeIfNone(fmt.Errorf("test run stopped from REST API"), exitcodes.ScriptStoppedFromRESTAPI),
				errext.AbortedByUser,
			),
		)
	} else {
		if status.Paused.Valid {
			if err = cs.Scheduler.SetPaused(status.Paused.Bool); err != nil {
				apiError(rw, "Pause error", err.Error(), http.StatusInternalServerError)
				return
			}
		}

		if status.VUsMax.Valid || status.VUs.Valid {
			apiError(rw, "Execution config error",
				"live VU configuration updates are not supported", http.StatusInternalServerError)
			return
		}
	}

	data, err := json.Marshal(newStatusJSONAPIFromEngine(cs))
	if err != nil {
		apiError(rw, "Encoding error", err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = rw.Write(data)
}
