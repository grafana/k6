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
	"github.com/loadimpact/k6/lib"
	"github.com/manyminds/api2go/jsonapi"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

type groupDummyRunner struct {
	Group *lib.Group
}

func (r groupDummyRunner) NewVU() (lib.VU, error) { return nil, nil }

func (r groupDummyRunner) GetDefaultGroup() *lib.Group { return r.Group }

func (r groupDummyRunner) GetOptions() lib.Options { return lib.Options{} }

func (r groupDummyRunner) ApplyOptions(opts lib.Options) {}

func TestGetGroups(t *testing.T) {
	g := &lib.Group{}
	g1 := &lib.Group{ID: 1, Name: "group 1", Parent: g}
	g2 := &lib.Group{ID: 2, Name: "group 2", Parent: g1}
	g.Groups = map[string]*lib.Group{g1.Name: g1}
	g1.Groups = map[string]*lib.Group{g2.Name: g2}

	engine, err := lib.NewEngine(groupDummyRunner{g}, lib.Options{})
	assert.NoError(t, err)

	t.Run("list", func(t *testing.T) {
		rw := httptest.NewRecorder()
		NewHandler().ServeHTTP(rw, newRequestWithEngine(engine, "GET", "/v1/groups", nil))
		res := rw.Result()
		assert.Equal(t, http.StatusOK, res.StatusCode)

		t.Run("document", func(t *testing.T) {
			var doc jsonapi.Document
			assert.NoError(t, json.Unmarshal(rw.Body.Bytes(), &doc))
			assert.Nil(t, doc.Data.DataObject)
			if assert.NotEmpty(t, doc.Data.DataArray) {
				assert.Equal(t, "groups", doc.Data.DataArray[0].Type)
			}
		})

		t.Run("groups", func(t *testing.T) {
			var groups []Group
			assert.NoError(t, jsonapi.Unmarshal(rw.Body.Bytes(), &groups))
			if assert.Len(t, groups, 3) {
				for _, g := range groups {
					switch g.ID {
					case 0:
						assert.Equal(t, "", g.Name)
						assert.Nil(t, g.Parent)
						assert.Equal(t, int64(0), g.ParentID)
						assert.Len(t, g.GroupIDs, 1)
						assert.Contains(t, g.GroupIDs, int64(1))
					case 1:
						assert.Equal(t, "group 1", g.Name)
						assert.Nil(t, g.Parent)
						assert.Equal(t, int64(0), g.ParentID)
						assert.EqualValues(t, []int64{2}, g.GroupIDs)
					case 2:
						assert.Equal(t, "group 2", g.Name)
						assert.Nil(t, g.Parent)
						assert.Equal(t, int64(1), g.ParentID)
						assert.EqualValues(t, []int64{}, g.GroupIDs)
					default:
						assert.Fail(t, "Unknown ID: %d", g.ID)
					}
				}
			}
		})
	})
	for _, gp := range []*lib.Group{g, g1, g2} {
		t.Run(gp.Name, func(t *testing.T) {
			rw := httptest.NewRecorder()
			NewHandler().ServeHTTP(rw, newRequestWithEngine(engine, "GET", "/v1/groups/"+strconv.FormatInt(gp.ID, 10), nil))
			res := rw.Result()
			assert.Equal(t, http.StatusOK, res.StatusCode)
		})
	}
}
