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

import (
	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/dop251/goja"
)

// JSHandle is the interface of an in-page JS object.
type JSHandle interface {
	AsElement() ElementHandle
	Dispose()
	Evaluate(pageFunc goja.Value, args ...goja.Value) interface{}
	EvaluateHandle(pageFunc goja.Value, args ...goja.Value) JSHandle
	GetProperties() map[string]JSHandle
	GetProperty(propertyName string) JSHandle
	JSONValue() goja.Value
	ObjectID() cdpruntime.RemoteObjectID
}
