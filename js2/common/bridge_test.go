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

package common

import (
	"reflect"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
)

type bridgeTestType struct {
	Exported      string
	ExportedTag   string `js:"renamed"`
	unexported    string
	unexportedTag string `js:"unexported"`
}

func (bridgeTestType) ExportedFn()   {}
func (bridgeTestType) unexportedFn() {}

func (*bridgeTestType) ExportedPtrFn()   {}
func (*bridgeTestType) unexportedPtrFn() {}

func TestFieldNameMapper(t *testing.T) {
	typ := reflect.TypeOf(bridgeTestType{})
	t.Run("Fields", func(t *testing.T) {
		names := map[string]string{
			"Exported":      "exported",
			"ExportedTag":   "renamed",
			"unexported":    "",
			"unexportedTag": "",
		}
		for name, result := range names {
			t.Run(name, func(t *testing.T) {
				f, ok := typ.FieldByName(name)
				if assert.True(t, ok) {
					assert.Equal(t, result, (FieldNameMapper{}).FieldName(typ, f))
				}
			})
		}
	})
	t.Run("Exported", func(t *testing.T) {
		t.Run("ExportedFn", func(t *testing.T) {
			m, ok := typ.MethodByName("ExportedFn")
			if assert.True(t, ok) {
				assert.Equal(t, "exportedFn", (FieldNameMapper{}).MethodName(typ, m))
			}
		})
		t.Run("unexportedFn", func(t *testing.T) {
			_, ok := typ.MethodByName("unexportedFn")
			assert.False(t, ok)
		})
	})
}

func TestBindToGlobal(t *testing.T) {
	testdata := map[string]struct {
		Obj  interface{}
		Keys []string
		Not  []string
	}{
		"Value": {
			bridgeTestType{},
			[]string{"exported", "renamed", "exportedFn"},
			[]string{"exportedPtrFn"},
		},
		"Pointer": {
			&bridgeTestType{},
			[]string{"exported", "renamed", "exportedFn", "exportedPtrFn"},
			[]string{},
		},
	}
	for name, data := range testdata {
		t.Run(name, func(t *testing.T) {
			rt := goja.New()
			unbind := BindToGlobal(rt, data.Obj)
			for _, k := range data.Keys {
				t.Run(k, func(t *testing.T) {
					v := rt.Get(k)
					if assert.NotNil(t, v) {
						assert.False(t, goja.IsUndefined(v), "value is undefined")
					}
				})
			}
			for _, k := range data.Not {
				t.Run(k, func(t *testing.T) {
					assert.Nil(t, rt.Get(k), "unexpected member bridged")
				})
			}

			t.Run("Unbind", func(t *testing.T) {
				unbind()
				for _, k := range data.Keys {
					t.Run(k, func(t *testing.T) {
						v := rt.Get(k)
						assert.True(t, goja.IsUndefined(v), "value is not undefined")
					})
				}
			})
		})
	}
}
