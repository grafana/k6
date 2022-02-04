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

package common

import (
	"reflect"

	k6common "go.k6.io/k6/js/common"
)

// FieldNameMapper for goja.Runtime.SetFieldNameMapper().
type FieldNameMapper struct {
	parent k6common.FieldNameMapper
}

var methodNameExceptions = map[string]string{
	"Query":    "$",
	"QueryAll": "$$",
}

// NewFieldNameMapper creates a new field name mapper to add some method name
// exceptions needed for the xk6-browser extension.
func NewFieldNameMapper() *FieldNameMapper {
	return &FieldNameMapper{
		parent: k6common.FieldNameMapper{},
	}
}

// FieldName is part of the goja.FieldNameMapper interface
// https://godoc.org/github.com/dop251/goja#FieldNameMapper
func (fm *FieldNameMapper) FieldName(t reflect.Type, f reflect.StructField) string {
	return fm.parent.FieldName(t, f)
}

// MethodName is part of the goja.FieldNameMapper interface
// https://godoc.org/github.com/dop251/goja#FieldNameMapper
func (fm *FieldNameMapper) MethodName(t reflect.Type, m reflect.Method) string {
	if exception, ok := methodNameExceptions[m.Name]; ok {
		return exception
	}
	return fm.parent.MethodName(t, m)
}
