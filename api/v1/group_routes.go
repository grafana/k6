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
	"github.com/julienschmidt/httprouter"
	"github.com/loadimpact/k6/api/common"
	"github.com/manyminds/api2go/jsonapi"
	"net/http"
	"strconv"
)

func HandleGetGroups(rw http.ResponseWriter, r *http.Request, p httprouter.Params) {
	engine := common.GetEngine(r.Context())

	root := NewGroup(engine.Runner.GetDefaultGroup(), nil)
	groups := FlattenGroup(root)

	data, err := jsonapi.Marshal(groups)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = rw.Write(data)
}

func HandleGetGroup(rw http.ResponseWriter, r *http.Request, p httprouter.Params) {
	id, err := strconv.ParseInt(p.ByName("id"), 10, 64)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}

	engine := common.GetEngine(r.Context())

	root := NewGroup(engine.Runner.GetDefaultGroup(), nil)
	groups := FlattenGroup(root)

	var group *Group
	for _, g := range groups {
		if g.ID == id {
			group = g
			break
		}
	}
	if group == nil {
		http.Error(rw, "No such group", http.StatusNotFound)
		return
	}

	data, err := jsonapi.Marshal(group)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = rw.Write(data)
}
