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

package v2

import (
	"encoding/json"
	"github.com/loadimpact/k6/lib"
	"github.com/manyminds/api2go/jsonapi"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetStatus(t *testing.T) {
	engine, err := lib.NewEngine(nil)
	assert.NoError(t, err)

	rw := httptest.NewRecorder()
	NewHandler().ServeHTTP(rw, newRequestWithEngine(engine, "GET", "/v2/status", nil))
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
		assert.True(t, status.Running.Valid)
		assert.True(t, status.VUs.Valid)
		assert.True(t, status.VUsMax.Valid)
		assert.False(t, status.Tainted)
	})
}
