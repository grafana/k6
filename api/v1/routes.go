// Package v1 implements the v1 of the k6's REST API
package v1

import (
	"net/http"
)

func NewHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/status", func(rw http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleGetStatus(rw, r)
		case http.MethodPatch:
			handlePatchStatus(rw, r)
		default:
			rw.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/v1/metrics", func(rw http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			rw.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		handleGetMetrics(rw, r)
	})

	mux.HandleFunc("/v1/metrics/", func(rw http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			rw.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		id := r.URL.Path[len("/v1/metrics/"):]
		handleGetMetric(rw, r, id)
	})

	mux.HandleFunc("/v1/groups", func(rw http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			rw.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		handleGetGroups(rw, r)
	})

	mux.HandleFunc("/v1/groups/", func(rw http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			rw.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		id := r.URL.Path[len("/v1/groups/"):]
		handleGetGroup(rw, r, id)
	})

	mux.HandleFunc("/v1/setup", func(rw http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			handleRunSetup(rw, r)
		case http.MethodPut:
			handleSetSetupData(rw, r)
		case http.MethodGet:
			handleGetSetupData(rw, r)
		default:
			rw.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/v1/teardown", func(rw http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			rw.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		handleRunTeardown(rw, r)
	})

	return mux
}
