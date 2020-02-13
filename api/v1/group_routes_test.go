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

func TestGetGroups(t *testing.T) {
	g0, err := lib.NewGroup("", nil)
	assert.NoError(t, err)
	g1, err := g0.Group("group 1")
	assert.NoError(t, err)
	g2, err := g1.Group("group 2")
	assert.NoError(t, err)

	execScheduler, err := local.NewExecutionScheduler(&minirunner.MiniRunner{Group: g0}, logrus.StandardLogger())
	require.NoError(t, err)
	engine, err := core.NewEngine(execScheduler, lib.Options{}, logrus.StandardLogger())
	require.NoError(t, err)

	t.Run("list", func(t *testing.T) {
		rw := httptest.NewRecorder()
		NewHandler().ServeHTTP(rw, newRequestWithEngine(engine, "GET", "/v1/groups", nil))
		res := rw.Result()
		body := rw.Body.Bytes()
		assert.Equal(t, http.StatusOK, res.StatusCode)
		assert.NotEmpty(t, body)

		t.Run("document", func(t *testing.T) {
			var doc jsonapi.Document
			assert.NoError(t, json.Unmarshal(body, &doc))
			assert.Nil(t, doc.Data.DataObject)
			if assert.NotEmpty(t, doc.Data.DataArray) {
				assert.Equal(t, "groups", doc.Data.DataArray[0].Type)
			}
		})

		t.Run("groups", func(t *testing.T) {
			var groups []Group
			require.NoError(t, jsonapi.Unmarshal(body, &groups))
			require.Len(t, groups, 3)
			for _, g := range groups {
				switch g.ID {
				case g0.ID:
					assert.Equal(t, "", g.Name)
					assert.Nil(t, g.Parent)
					assert.Equal(t, "", g.ParentID)
					assert.Len(t, g.GroupIDs, 1)
					assert.EqualValues(t, []string{g1.ID}, g.GroupIDs)
				case g1.ID:
					assert.Equal(t, "group 1", g.Name)
					assert.Nil(t, g.Parent)
					assert.Equal(t, g0.ID, g.ParentID)
					assert.EqualValues(t, []string{g2.ID}, g.GroupIDs)
				case g2.ID:
					assert.Equal(t, "group 2", g.Name)
					assert.Nil(t, g.Parent)
					assert.Equal(t, g1.ID, g.ParentID)
					assert.EqualValues(t, []string{}, g.GroupIDs)
				default:
					assert.Fail(t, "Unknown ID: "+g.ID)
				}
			}
		})
	})
	for _, gp := range []*lib.Group{g0, g1, g2} {
		t.Run(gp.Name, func(t *testing.T) {
			rw := httptest.NewRecorder()
			NewHandler().ServeHTTP(rw, newRequestWithEngine(engine, "GET", "/v1/groups/"+gp.ID, nil))
			res := rw.Result()
			assert.Equal(t, http.StatusOK, res.StatusCode)
		})
	}
}
