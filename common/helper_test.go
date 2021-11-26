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
	"context"
	"encoding/json"
	"fmt"
	"math"
	"testing"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/runtime"
	"github.com/dop251/goja"
	"github.com/grafana/xk6-browser/testutils"
	"github.com/mailru/easyjson"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	k6common "go.k6.io/k6/js/common"
)

func newExecCtx() (*ExecutionContext, context.Context, *goja.Runtime) {
	ctx := context.Background()
	logger := NewLogger(ctx, logrus.New(), false, nil)
	execCtx := NewExecutionContext(ctx, nil, nil, runtime.ExecutionContextID(123456789), logger)

	return execCtx, ctx, goja.New()
}

func TestHelpersConvertArgument(t *testing.T) {
	t.Parallel()

	t.Run("int64", func(t *testing.T) {
		execCtx, ctx, rt := newExecCtx()
		var value int64 = 777
		arg, _ := convertArgument(ctx, execCtx, rt.ToValue(value))

		require.NotNil(t, arg)
		result, _ := json.Marshal(value)
		require.Equal(t, result, []byte(arg.Value))
		require.Empty(t, arg.UnserializableValue)
		require.Empty(t, arg.ObjectID)
	})

	t.Run("int64 maxint", func(t *testing.T) {
		execCtx, ctx, rt := newExecCtx()

		var value int64 = math.MaxInt32 + 1
		arg, _ := convertArgument(ctx, execCtx, rt.ToValue(value))

		require.NotNil(t, arg)
		require.Equal(t, fmt.Sprintf("%dn", value), string(arg.UnserializableValue))
		require.Empty(t, arg.Value)
		require.Empty(t, arg.ObjectID)
	})

	t.Run("float64", func(t *testing.T) {
		execCtx, ctx, rt := newExecCtx()

		var value float64 = 777.0
		arg, _ := convertArgument(ctx, execCtx, rt.ToValue(value))

		require.NotNil(t, arg)
		result, _ := json.Marshal(value)
		require.Equal(t, result, []byte(arg.Value))
		require.Empty(t, arg.UnserializableValue)
		require.Empty(t, arg.ObjectID)
	})

	t.Run("float64 unserializable values", func(t *testing.T) {
		execCtx, ctx, rt := newExecCtx()

		unserializableValues := []struct {
			value    float64
			expected string
		}{
			{
				value:    math.Float64frombits(0 | (1 << 63)),
				expected: "-0",
			},
			{
				value:    math.Inf(0),
				expected: "Infinity",
			},
			{
				value:    math.Inf(-1),
				expected: "-Infinity",
			},
			{
				value:    math.NaN(),
				expected: "NaN",
			},
		}

		for _, v := range unserializableValues {
			arg, _ := convertArgument(ctx, execCtx, rt.ToValue(v.value))
			require.NotNil(t, arg)
			require.Equal(t, v.expected, string(arg.UnserializableValue))
			require.Empty(t, arg.Value)
			require.Empty(t, arg.ObjectID)
		}
	})

	t.Run("bool", func(t *testing.T) {
		execCtx, ctx, rt := newExecCtx()

		var value bool = true
		arg, _ := convertArgument(ctx, execCtx, rt.ToValue(value))

		require.NotNil(t, arg)
		result, _ := json.Marshal(value)
		require.Equal(t, result, []byte(arg.Value))
		require.Empty(t, arg.UnserializableValue)
		require.Empty(t, arg.ObjectID)
	})

	t.Run("string", func(t *testing.T) {
		execCtx, ctx, rt := newExecCtx()
		var value string = "hello world"
		arg, _ := convertArgument(ctx, execCtx, rt.ToValue(value))

		require.NotNil(t, arg)
		result, _ := json.Marshal(value)
		require.Equal(t, result, []byte(arg.Value))
		require.Empty(t, arg.UnserializableValue)
		require.Empty(t, arg.ObjectID)
	})

	t.Run("*BaseJSHandle", func(t *testing.T) {
		execCtx, ctx, rt := newExecCtx()
		timeoutSetings := NewTimeoutSettings(nil)
		frameManager := NewFrameManager(ctx, nil, nil, timeoutSetings, NewLogger(ctx, NullLogger(), false, nil))
		frame := NewFrame(ctx, frameManager, nil, cdp.FrameID("frame_id_0123456789"))
		remoteObjValue := "hellow world"
		result, _ := json.Marshal(remoteObjValue)
		remoteObject := &runtime.RemoteObject{
			Type:  "string",
			Value: result,
		}

		value := NewJSHandle(ctx, nil, execCtx, frame, remoteObject, execCtx.logger)
		arg, _ := convertArgument(ctx, execCtx, rt.ToValue(value))

		require.NotNil(t, arg)
		require.Equal(t, result, []byte(arg.Value))
		require.Empty(t, arg.UnserializableValue)
		require.Empty(t, arg.ObjectID)
	})

	t.Run("*BaseJSHandle wrong context", func(t *testing.T) {
		execCtx, ctx, rt := newExecCtx()
		timeoutSetings := NewTimeoutSettings(nil)
		frameManager := NewFrameManager(ctx, nil, nil, timeoutSetings, NewLogger(ctx, NullLogger(), false, nil))
		frame := NewFrame(ctx, frameManager, nil, cdp.FrameID("frame_id_0123456789"))
		remoteObjectID := runtime.RemoteObjectID("object_id_0123456789")
		remoteObject := &runtime.RemoteObject{
			Type:     "object",
			Subtype:  "node",
			ObjectID: remoteObjectID,
		}
		execCtx2 := NewExecutionContext(ctx, nil, nil, runtime.ExecutionContextID(123456789), execCtx.logger)

		value := NewJSHandle(ctx, nil, execCtx2, frame, remoteObject, execCtx.logger)
		arg, err := convertArgument(ctx, execCtx, rt.ToValue(value))

		require.Nil(t, arg)
		require.ErrorIs(t, ErrWrongExecutionContext, err)
	})

	t.Run("*BaseJSHandle is disposed", func(t *testing.T) {
		execCtx, ctx, rt := newExecCtx()
		timeoutSetings := NewTimeoutSettings(nil)
		frameManager := NewFrameManager(ctx, nil, nil, timeoutSetings, NewLogger(ctx, NullLogger(), false, nil))
		frame := NewFrame(ctx, frameManager, nil, cdp.FrameID("frame_id_0123456789"))
		remoteObjectID := runtime.RemoteObjectID("object_id_0123456789")
		remoteObject := &runtime.RemoteObject{
			Type:     "object",
			Subtype:  "node",
			ObjectID: remoteObjectID,
		}

		value := NewJSHandle(ctx, nil, execCtx, frame, remoteObject, execCtx.logger)
		value.(*BaseJSHandle).disposed = true
		arg, err := convertArgument(ctx, execCtx, rt.ToValue(value))

		require.Nil(t, arg)
		require.ErrorIs(t, ErrJSHandleDisposed, err)
	})

	t.Run("*BaseJSHandle as *ElementHandle", func(t *testing.T) {
		execCtx, ctx, rt := newExecCtx()
		timeoutSetings := NewTimeoutSettings(nil)
		frameManager := NewFrameManager(ctx, nil, nil, timeoutSetings, NewLogger(ctx, NullLogger(), false, nil))
		frame := NewFrame(ctx, frameManager, nil, cdp.FrameID("frame_id_0123456789"))
		remoteObjectID := runtime.RemoteObjectID("object_id_0123456789")
		remoteObject := &runtime.RemoteObject{
			Type:     "object",
			Subtype:  "node",
			ObjectID: remoteObjectID,
		}

		value := NewJSHandle(ctx, nil, execCtx, frame, remoteObject, execCtx.logger)
		arg, _ := convertArgument(ctx, execCtx, rt.ToValue(value))

		require.NotNil(t, arg)
		require.Equal(t, remoteObjectID, arg.ObjectID)
		require.Empty(t, arg.Value)
		require.Empty(t, arg.UnserializableValue)
	})
}

func TestHelpersValueFromRemoteObject(t *testing.T) {
	t.Parallel()

	t.Run("unserializable value error", func(t *testing.T) {
		rt := goja.New()
		ctx := k6common.WithRuntime(context.Background(), rt)
		unserializableValue := runtime.UnserializableValue("a string instead")
		remoteObject := &runtime.RemoteObject{
			Type:                "number",
			UnserializableValue: unserializableValue,
		}

		logger := NewLogger(ctx, logrus.New(), false, nil)
		arg, err := valueFromRemoteObject(ctx, remoteObject, logger.logger.WithContext(ctx))
		require.True(t, goja.IsNull(arg))
		require.ErrorIs(t, UnserializableValueError{unserializableValue}, err)
	})

	t.Run("bigint parsing error", func(t *testing.T) {
		rt := goja.New()
		ctx := k6common.WithRuntime(context.Background(), rt)
		unserializableValue := runtime.UnserializableValue("a string instead")
		remoteObject := &runtime.RemoteObject{
			Type:                "bigint",
			UnserializableValue: unserializableValue,
		}

		logger := NewLogger(ctx, logrus.New(), false, nil)
		arg, err := valueFromRemoteObject(ctx, remoteObject, logger.logger.WithContext(ctx))

		require.True(t, goja.IsNull(arg))
		assert.ErrorIs(t, UnserializableValueError{unserializableValue}, err)
	})

	t.Run("float64 unserializable values", func(t *testing.T) {
		rt := goja.New()
		ctx := k6common.WithRuntime(context.Background(), rt)
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
			logger := NewLogger(ctx, logrus.New(), false, nil)
			arg, err := valueFromRemoteObject(ctx, remoteObject, logger.logger.WithContext(ctx))
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
		rt := goja.New()
		ctx := k6common.WithRuntime(context.Background(), rt)

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

		for _, p := range primitiveTypes {
			marshalled, _ := json.Marshal(p.value)
			remoteObject := &runtime.RemoteObject{
				Type:  p.typ,
				Value: marshalled,
			}

			logger := NewLogger(ctx, logrus.New(), false, nil)
			arg, err := valueFromRemoteObject(ctx, remoteObject, logger.logger.WithContext(ctx))

			require.Nil(t, err)
			require.Equal(t, p.value, p.toFn(arg))
		}
	})

	t.Run("remote object with ID", func(t *testing.T) {
		rt := goja.New()
		ctx := k6common.WithRuntime(context.Background(), rt)
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

		logger := NewLogger(ctx, logrus.New(), false, nil)
		val, err := valueFromRemoteObject(ctx, remoteObject, logger.logger.WithContext(ctx))
		require.NoError(t, err)
		assert.Equal(t, rt.ToValue(map[string]interface{}{"num": float64(1)}), val)
	})

}

func TestHelpersParseRemoteObject(t *testing.T) {
	t.Parallel()

	rt := goja.New()
	ctx := k6common.WithRuntime(context.Background(), rt)

	testCases := []struct {
		name     string
		preview  *runtime.ObjectPreview
		value    string
		expected map[string]interface{}
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
			name:     "overflow",
			preview:  &runtime.ObjectPreview{Overflow: true},
			expected: map[string]interface{}{},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			remoteObject := &runtime.RemoteObject{
				Type:     "object",
				ObjectID: runtime.RemoteObjectID("object_id_0123456789"),
				Preview:  tc.preview,
				Value:    easyjson.RawMessage(tc.value),
			}
			lg := logrus.New()
			logHook := testutils.LogHook(lg)
			logger := NewLogger(ctx, lg, false, nil)
			val, err := parseRemoteObject(remoteObject, logger.logger.WithContext(ctx))
			require.NoError(t, err)
			assert.Equal(t, tc.expected, val)

			if tc.name == "overflow" {
				var gotMsg bool
				for _, evt := range logHook.Drain() {
					if evt.Message == "object will be parsed partially" {
						gotMsg = true
					}
				}
				assert.True(t, gotMsg)
			}
		})
	}
}
