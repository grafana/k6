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

package http

import (
	"net/url"
	"testing"

	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js/common"
	"github.com/stretchr/testify/assert"
)

func TestTagURL(t *testing.T) {
	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})
	rt.Set("http", common.Bind(rt, New(), nil))

	testdata := map[string]struct{ u, n string }{
		`http://localhost/anything/`:               {"http://localhost/anything/", "http://localhost/anything/"},
		`http://localhost/anything/${1+1}`:         {"http://localhost/anything/2", "http://localhost/anything/${}"},
		`http://localhost/anything/${1+1}/`:        {"http://localhost/anything/2/", "http://localhost/anything/${}/"},
		`http://localhost/anything/${1+1}/${1+2}`:  {"http://localhost/anything/2/3", "http://localhost/anything/${}/${}"},
		`http://localhost/anything/${1+1}/${1+2}/`: {"http://localhost/anything/2/3/", "http://localhost/anything/${}/${}/"},
	}
	for expr, data := range testdata {
		t.Run("expr="+expr, func(t *testing.T) {
			u, err := url.Parse(data.u)
			assert.NoError(t, err)
			tag := URL{u, data.n, data.u}
			v, err := common.RunString(rt, "http.url`"+expr+"`")
			if assert.NoError(t, err) {
				assert.Equal(t, tag, v.Export())
			}
		})
	}
}
