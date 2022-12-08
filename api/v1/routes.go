// Package v1 implements the v1 of the k6's REST API
package v1

import (
	"net/http"
)

// NewHandler returns the top handler for the v1 REST APIs
func NewHandler(cs *ControlSurface) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/status", func(rw http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleGetStatus(cs, rw, r)
		case http.MethodPatch:
			handlePatchStatus(cs, rw, r)
		default:
			rw.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/v1/metrics", func(rw http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			rw.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		handleGetMetrics(cs, rw, r)
	})

	mux.HandleFunc("/v1/metrics/", func(rw http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			rw.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		id := r.URL.Path[len("/v1/metrics/"):]
		handleGetMetric(cs, rw, r, id)
	})

	mux.HandleFunc("/v1/groups", func(rw http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			rw.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		handleGetGroups(cs, rw, r)
	})

	mux.HandleFunc("/v1/groups/", func(rw http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			rw.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		id := r.URL.Path[len("/v1/groups/"):]
		handleGetGroup(cs, rw, r, id)
	})

	mux.HandleFunc("/v1/setup", func(rw http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			handleRunSetup(cs, rw, r)
		case http.MethodPut:
			handleSetSetupData(cs, rw, r)
		case http.MethodGet:
			handleGetSetupData(cs, rw, r)
		default:
			rw.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/v1/teardown", func(rw http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			rw.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		handleRunTeardown(cs, rw, r)
	})

	return mux
}
