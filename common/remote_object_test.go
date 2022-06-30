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
	"encoding/json"
	"math"
	"testing"

	"github.com/grafana/xk6-browser/k6ext/k6test"

	"github.com/chromedp/cdproto/runtime"
	"github.com/dop251/goja"
	"github.com/mailru/easyjson"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValueFromRemoteObject(t *testing.T) {
	t.Parallel()

	t.Run("unserializable value error", func(t *testing.T) {
		vu := k6test.NewVU(t)
		unserializableValue := runtime.UnserializableValue("a string instead")
		remoteObject := &runtime.RemoteObject{
			Type:                "number",
			UnserializableValue: unserializableValue,
		}

		arg, err := valueFromRemoteObject(vu.Context(), remoteObject)
		require.True(t, goja.IsNull(arg))
		require.ErrorIs(t, UnserializableValueError{unserializableValue}, err)
	})

	t.Run("bigint parsing error", func(t *testing.T) {
		vu := k6test.NewVU(t)
		unserializableValue := runtime.UnserializableValue("a string instead")
		remoteObject := &runtime.RemoteObject{
			Type:                "bigint",
			UnserializableValue: unserializableValue,
		}

		arg, err := valueFromRemoteObject(vu.Context(), remoteObject)

		require.True(t, goja.IsNull(arg))
		assert.ErrorIs(t, UnserializableValueError{unserializableValue}, err)
	})

	t.Run("float64 unserializable values", func(t *testing.T) {
		vu := k6test.NewVU(t)
		unserializableValues := []struct {
			value    string
			expected float64
		}{
			{
				value:    "-0",
				expected: math.Float64frombits(0 | (1 << 63)),
			},
			{
				value:    "Infinity",
				expected: math.Inf(0),
			},
			{
				value:    "-Infinity",
				expected: math.Inf(-1),
			},
			{
				value:    "NaN",
				expected: math.NaN(),
			},
		}

		for _, v := range unserializableValues {
			remoteObject := &runtime.RemoteObject{
				Type:                "number",
				UnserializableValue: runtime.UnserializableValue(v.value),
			}
			arg, err := valueFromRemoteObject(vu.Context(), remoteObject)
			require.NoError(t, err)
			require.NotNil(t, arg)
			if v.value == "NaN" {
				require.True(t, math.IsNaN(arg.ToFloat()))
			} else {
				require.Equal(t, v.expected, arg.ToFloat())
			}
		}
	})

	t.Run("primitive types", func(t *testing.T) {
		primitiveTypes := []struct {
			typ   runtime.Type
			value interface{}
			toFn  func(goja.Value) interface{}
		}{
			{
				typ:   "number",
				value: int64(777),
				toFn:  func(v goja.Value) interface{} { return v.ToInteger() },
			},
			{
				typ:   "number",
				value: float64(777.0),
				toFn:  func(v goja.Value) interface{} { return v.ToFloat() },
			},
			{
				typ:   "string",
				value: "hello world",
				toFn:  func(v goja.Value) interface{} { return v.String() },
			},
			{
				typ:   "boolean",
				value: true,
				toFn:  func(v goja.Value) interface{} { return v.ToBoolean() },
			},
		}

		vu := k6test.NewVU(t)
		for _, p := range primitiveTypes {
			marshalled, _ := json.Marshal(p.value)
			remoteObject := &runtime.RemoteObject{
				Type:  p.typ,
				Value: marshalled,
			}

			arg, err := valueFromRemoteObject(vu.Context(), remoteObject)

			require.Nil(t, err)
			require.Equal(t, p.value, p.toFn(arg))
		}
	})

	t.Run("remote object with ID", func(t *testing.T) {
		remoteObjectID := runtime.RemoteObjectID("object_id_0123456789")
		remoteObject := &runtime.RemoteObject{
			Type:     "object",
			Subtype:  "node",
			ObjectID: remoteObjectID,
			Preview: &runtime.ObjectPreview{
				Properties: []*runtime.PropertyPreview{
					{Name: "num", Type: runtime.TypeNumber, Value: "1"},
				},
			},
		}

		vu := k6test.NewVU(t)
		val, err := valueFromRemoteObject(vu.Context(), remoteObject)
		require.NoError(t, err)
		assert.Equal(t, vu.ToGojaValue(map[string]interface{}{"num": float64(1)}), val)
	})
}

func TestParseRemoteObject(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		preview  *runtime.ObjectPreview
		value    string
		expected map[string]interface{}
		expErr   string
	}{
		{
			name: "most_types",
			preview: &runtime.ObjectPreview{
				Properties: []*runtime.PropertyPreview{
					{Name: "accessor", Type: runtime.TypeAccessor, Value: ""},
					{Name: "bigint", Type: runtime.TypeBigint, Value: "100n"},
					{Name: "bool", Type: runtime.TypeBoolean, Value: "true"},
					{Name: "fn", Type: runtime.TypeFunction, Value: ""},
					{Name: "num", Type: runtime.TypeNumber, Value: "1"},
					{Name: "str", Type: runtime.TypeString, Value: "string"},
					{Name: "strquot", Type: runtime.TypeString, Value: `"quoted string"`},
					{Name: "sym", Type: runtime.TypeSymbol, Value: "Symbol()"},
				},
			},
			expected: map[string]interface{}{
				"accessor": "accessor",
				"bigint":   int64(100),
				"bool":     true,
				"fn":       "function()",
				"num":      float64(1),
				"str":      "string",
				"strquot":  "quoted string",
				"sym":      "Symbol()",
			},
		},
		{
			name: "nested",
			preview: &runtime.ObjectPreview{
				Properties: []*runtime.PropertyPreview{
					// We don't actually get nested ObjectPreviews from CDP.
					// I.e. the object `{nested: {one: 1}}` returns value "Object"
					// for the "nested" property, with a nil *ValuePreview. :-/
					{Name: "nested", Type: runtime.TypeObject, Value: "Object"},
				},
			},
			expected: map[string]interface{}{
				"nested": "Object",
			},
		},
		{
			name:     "err_overflow",
			preview:  &runtime.ObjectPreview{Overflow: true},
			expected: map[string]interface{}{},
			expErr:   "object is too large and will be parsed partially",
		},
		{
			name: "err_parsing_property",
			preview: &runtime.ObjectPreview{
				Properties: []*runtime.PropertyPreview{
					{Name: "failprop", Type: runtime.TypeObject, Value: "some"},
				},
			},
			expected: map[string]interface{}{},
			expErr:   "parsing object property",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			remoteObject := &runtime.RemoteObject{
				Type:     "object",
				ObjectID: runtime.RemoteObjectID("object_id_0123456789"),
				Preview:  tc.preview,
				Value:    easyjson.RawMessage(tc.value),
			}
			val, err := parseRemoteObject(remoteObject)
			assert.Equal(t, tc.expected, val)
			if tc.expErr != "" {
				assert.Contains(t, err.Error(), tc.expErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
