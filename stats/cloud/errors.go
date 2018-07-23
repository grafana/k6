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

package cloud

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/pkg/errors"
)

var (
	ErrNotAuthorized    = errors.New("Not allowed to upload result to Load Impact cloud")
	ErrNotAuthenticated = errors.New("Failed to authenticate with Load Impact cloud")
	ErrUnknown          = errors.New("An error occurred talking to Load Impact cloud")
)

// ErrorResponse represents an error cause by talking to the API
type ErrorResponse struct {
	Response *http.Response `json:"-"`

	Code    int               `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details"`
}

func (e ErrorResponse) Error() string {
	msg := e.Message
	if e.Response != nil {
		msg = fmt.Sprintf("%d %s", e.Response.StatusCode, msg)
	}
	if e.Code != 0 {
		msg = fmt.Sprintf("%s [err code %d]", msg, e.Code)
	}

	var details []string
	for k, v := range e.Details {
		details = append(details, k+" ("+v+")")
	}
	if len(details) > 0 {
		msg += ": " + strings.Join(details, ", ")
	}

	return msg
}
