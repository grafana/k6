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
	"fmt"

	"github.com/dop251/goja"

	"github.com/k6io/k6/lib/netext/httpext"
)

// ToURL tries to convert anything passed to it to a k6 URL struct
func ToURL(u interface{}) (httpext.URL, error) {
	switch tu := u.(type) {
	case httpext.URL:
		// Handling of http.url`http://example.com/{$id}`
		return tu, nil
	case string:
		// Handling of "http://example.com/"
		return httpext.NewURL(tu, tu)
	case goja.Value:
		// Unwrap goja values
		return ToURL(tu.Export())
	default:
		return httpext.URL{}, fmt.Errorf("invalid URL value '%#v'", u)
	}
}

// URL creates new URL from the provided parts
func (http *HTTP) URL(parts []string, pieces ...string) (httpext.URL, error) {
	var name, urlstr string
	for i, part := range parts {
		name += part
		urlstr += part
		if i < len(pieces) {
			name += "${}"
			urlstr += pieces[i]
		}
	}
	return httpext.NewURL(urlstr, name)
}
