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
)

// A URL wraps net.URL, and preserves the template (if any) the URL was constructed from.
type URL struct {
	URL       *url.URL `js:"-"`
	Name      string   `js:"name"` // http://example.com/thing/${}/
	URLString string   `js:"url"`  // http://example.com/thing/1234/
}

func (http *HTTP) Url(parts []string, pieces ...string) (URL, error) {
	var name, urlstr string
	for i, part := range parts {
		name += part
		urlstr += part
		if i < len(pieces) {
			name += "${}"
			urlstr += pieces[i]
		}
	}
	u, err := url.Parse(urlstr)
	return URL{u, name, urlstr}, err
}
