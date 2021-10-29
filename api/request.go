/*
 *
 * xk6-browser - a browser automation extension for k6
 * Copyright (C) 2021 Load Impact
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

package api

import "github.com/dop251/goja"

// Request is the interface of an HTTP request
type Request interface {
	AllHeaders() map[string]string
	Failure() goja.Value
	Frame() Frame
	HeaderValue(string) goja.Value
	Headers() map[string]string
	HeadersArray() []HTTPHeader
	IsNavigationRequest() bool
	Method() string
	PostData() string
	PostDataBuffer() goja.ArrayBuffer
	PostDataJSON() string
	RedirectedFrom() Request
	RedirectedTo() Request
	ResourceType() string
	Response() Response
	Size() HTTPMessageSize
	Timing() goja.Value
	URL() string
}
