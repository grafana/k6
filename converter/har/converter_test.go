/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2017 Load Impact
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

package har

import (
	"fmt"
	"testing"

	"github.com/loadimpact/k6/js"
	"github.com/loadimpact/k6/lib"
	"github.com/spf13/afero"
)

func TestBuildK6Headers(t *testing.T) {
	var headers = []struct {
		values   []Header
		expected []string
	}{
		{[]Header{{"name", "1"}, {"name", "2"}}, []string{`"name": "1"`}},
		{[]Header{{"name", "1"}, {"name2", "2"}}, []string{`"name": "1"`, `"name2": "2"`}},
		{[]Header{{":host", "localhost"}}, []string{}},
	}

	for _, pair := range headers {
		v := buildK6Headers(pair.values)
		if len(v) != len(pair.expected) {
			t.Errorf("buildK6Headers(%v): expected %v, actual %v", pair.values, pair.expected, v)
		}
	}
}

func TestBuildK6RequestObject(t *testing.T) {
	req := &Request{
		Method:  "get",
		URL:     "http://www.google.es",
		Headers: []Header{{"accept-language", "es-ES,es;q=0.8"}},
		Cookies: []Cookie{{Name: "a", Value: "b"}},
	}
	v, err := buildK6RequestObject(req)
	if err != nil {
		t.Error(err)
	}
	_, err = js.New(&lib.SourceData{
		Filename: "/script.js",
		Data:     []byte(fmt.Sprintf("export default function() { res = http.batch([%v]); }", v)),
	}, afero.NewMemMapFs())

	if err != nil {
		t.Error(err)
	}
}
