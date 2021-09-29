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

package cloudapi

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

var (
	ErrNotAuthorized    = errors.New("Not allowed to upload result to Load Impact cloud")
	ErrNotAuthenticated = errors.New("Failed to authenticate with Load Impact cloud")
	ErrUnknown          = errors.New("An error occurred talking to Load Impact cloud")
)

// ErrorResponse represents an error cause by talking to the API
type ErrorResponse struct {
	Response *http.Response `json:"-"`

	Code        int                 `json:"code"`
	Message     string              `json:"message"`
	Details     map[string][]string `json:"details"`
	FieldErrors map[string][]string `json:"field_errors"`
	Errors      []string            `json:"errors"`
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func (e ErrorResponse) Error() string {
	msg := e.Message

	for _, v := range e.Errors {
		// atm: `errors` and `message` could be duplicated
		// TODO: remove condition when the API changes
		if v != msg {
			msg += "\n " + v
		}
	}

	// `e.Details` is the old API version
	// TODO: do not handle `details` when the old API becomes obsolete
	var details []string
	var detail string
	for k, v := range e.Details {
		detail = k + ": " + strings.Join(v, ", ")
		details = append(details, detail)
	}

	for k, v := range e.FieldErrors {
		detail = k + ": " + strings.Join(v, ", ")
		// atm: `details` and `field_errors` could be duplicated
		if !contains(details, detail) {
			details = append(details, detail)
		}
	}

	if len(details) > 0 {
		msg += "\n " + strings.Join(details, "\n")
	}

	var code string
	if e.Code > 0 && e.Response != nil {
		code = fmt.Sprintf("%d/E%d", e.Response.StatusCode, e.Code)
	} else if e.Response != nil {
		code = fmt.Sprintf("%d", e.Response.StatusCode)
	} else if e.Code > 0 {
		code = fmt.Sprintf("E%d", e.Code)
	}

	if len(code) > 0 {
		msg = fmt.Sprintf("(%s) %s", code, msg)
	}

	return msg
}
