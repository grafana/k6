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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/loadimpact/k6/core"
	"github.com/loadimpact/k6/core/local"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/testutils/minirunner"
	"github.com/manyminds/api2go/jsonapi"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetStatus(t *testing.T) {
	execScheduler, err := local.NewExecutionScheduler(&minirunner.MiniRunner{}, logrus.StandardLogger())
	require.NoError(t, err)
	engine, err := core.NewEngine(execScheduler, lib.Options{}, logrus.StandardLogger())
	require.NoError(t, err)

	rw := httptest.NewRecorder()
	NewHandler().ServeHTTP(rw, newRequestWithEngine(engine, "GET", "/v1/status", nil))
	res := rw.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode)

	t.Run("document", func(t *testing.T) {
		var doc jsonapi.Document
		assert.NoError(t, json.Unmarshal(rw.Body.Bytes(), &doc))
		if !assert.NotNil(t, doc.Data.DataObject) {
			return
		}
		assert.Equal(t, "status", doc.Data.DataObject.Type)
	})

	t.Run("status", func(t *testing.T) {
		var status Status
		assert.NoError(t, jsonapi.Unmarshal(rw.Body.Bytes(), &status))
		assert.True(t, status.Paused.Valid)
		assert.True(t, status.VUs.Valid)
		assert.True(t, status.VUsMax.Valid)
		assert.False(t, status.Tainted)
	})
}

// TODO: fix after the externally-controlled executor
/*
func TestPatchStatus(t *testing.T) {
	testdata := map[string]struct {
		StatusCode int
		Status     Status
	}{
		"nothing":      {200, Status{}},
		"paused":       {200, Status{Paused: null.BoolFrom(true)}},
		"max vus":      {200, Status{VUsMax: null.IntFrom(10)}},
		"too many vus": {400, Status{VUs: null.IntFrom(10), VUsMax: null.IntFrom(0)}},
		"vus":          {200, Status{VUs: null.IntFrom(10), VUsMax: null.IntFrom(10)}},
	}

	for name, indata := range testdata {
		t.Run(name, func(t *testing.T) {
			engine, err := core.NewEngine(nil, lib.Options{})
			assert.NoError(t, err)

			body, err := jsonapi.Marshal(indata.Status)
			if !assert.NoError(t, err) {
				return
			}

			rw := httptest.NewRecorder()
			NewHandler().ServeHTTP(rw, newRequestWithEngine(engine, "PATCH", "/v1/status", bytes.NewReader(body)))
			res := rw.Result()

			if !assert.Equal(t, indata.StatusCode, res.StatusCode) {
				return
			}
			if indata.StatusCode != 200 {
				return
			}

			status := NewStatus(engine)
			if indata.Status.Paused.Valid {
				assert.Equal(t, indata.Status.Paused, status.Paused)
			}
			if indata.Status.VUs.Valid {
				assert.Equal(t, indata.Status.VUs, status.VUs)
			}
			if indata.Status.VUsMax.Valid {
				assert.Equal(t, indata.Status.VUsMax, status.VUsMax)
			}
		})
	}
}
*/
