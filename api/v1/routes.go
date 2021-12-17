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

// Package v1 implements the v1 of the k6's API
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
