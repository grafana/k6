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

package v1

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// Error is an api error
type Error struct {
	Status string `json:"status,omitempty"`
	Title  string `json:"title,omitempty"`
	Detail string `json:"detail,omitempty"`
}

func (e Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Title, e.Detail)
}

// ErrorResponse is a struct wrapper around multiple errors
type ErrorResponse struct {
	Errors []Error `json:"errors"`
}

func apiError(rw http.ResponseWriter, title, detail string, status int) {
	doc := ErrorResponse{
		Errors: []Error{
			{
				Status: strconv.Itoa(status),
				Title:  title,
				Detail: detail,
			},
		},
	}
	data, err := json.Marshal(doc)
	if err != nil {
		panic(err)
	}
	rw.WriteHeader(status)
	_, _ = rw.Write(data)
}
