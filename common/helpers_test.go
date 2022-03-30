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
	"github.com/oxtoacart/bpool"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	k6common "go.k6.io/k6/js/common"
	k6modulestest "go.k6.io/k6/js/modulestest"
	k6lib "go.k6.io/k6/lib"
	k6metrics "go.k6.io/k6/lib/metrics"
	k6stats "go.k6.io/k6/stats"
	"gopkg.in/guregu/null.v3"
)

func newExecCtx() (*ExecutionContext, context.Context, *goja.Runtime) {
	ctx := context.Background()
	logger := NewLogger(ctx, logrus.New(), false, nil)
	execCtx := NewExecutionContext(ctx, nil, nil, runtime.ExecutionContextID(123456789), logger)

	return execCtx, ctx, goja.New()
}

func TestConvertArgument(t *testing.T) {
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
		log := NewNullLogger()

		timeoutSetings := NewTimeoutSettings(nil)
		frameManager := NewFrameManager(ctx, nil, nil, timeoutSetings, log)
		frame := NewFrame(ctx, frameManager, nil, cdp.FrameID("frame_id_0123456789"), log)
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
		log := NewNullLogger()

		timeoutSetings := NewTimeoutSettings(nil)
		frameManager := NewFrameManager(ctx, nil, nil, timeoutSetings, log)
		frame := NewFrame(ctx, frameManager, nil, cdp.FrameID("frame_id_0123456789"), log)
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
		log := NewNullLogger()

		timeoutSetings := NewTimeoutSettings(nil)
		frameManager := NewFrameManager(ctx, nil, nil, timeoutSetings, log)
		frame := NewFrame(ctx, frameManager, nil, cdp.FrameID("frame_id_0123456789"), log)
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
		log := NewNullLogger()

		timeoutSetings := NewTimeoutSettings(nil)
		frameManager := NewFrameManager(ctx, nil, nil, timeoutSetings, log)
		frame := NewFrame(ctx, frameManager, nil, cdp.FrameID("frame_id_0123456789"), log)
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

func newMockVU(tb testing.TB) *k6modulestest.VU {
	tb.Helper()

	rt := goja.New()
	rt.SetFieldNameMapper(k6common.FieldNameMapper{})

	samples := make(chan k6stats.SampleContainer, 1000)

	root, err := k6lib.NewGroup("", nil)
	require.NoError(tb, err)

	state := &k6lib.State{
		Options: k6lib.Options{
			MaxRedirects: null.IntFrom(10),
			UserAgent:    null.StringFrom("TestUserAgent"),
			Throw:        null.BoolFrom(true),
			SystemTags:   &k6stats.DefaultSystemTagSet,
			Batch:        null.IntFrom(20),
			BatchPerHost: null.IntFrom(20),
			// HTTPDebug:    null.StringFrom("full"),
		},
		Logger:         logrus.StandardLogger(),
		Group:          root,
		BPool:          bpool.NewBufferPool(1),
		Samples:        samples,
		Tags:           k6lib.NewTagMap(map[string]string{"group": root.Path}),
		BuiltinMetrics: k6metrics.RegisterBuiltinMetrics(k6metrics.NewRegistry()),
	}
	mockVU := &k6modulestest.VU{
		RuntimeField: rt,
		InitEnvField: &k6common.InitEnvironment{
			Registry: k6metrics.NewRegistry(),
		},
		StateField: state,
	}
	ctx := WithVU(context.Background(), mockVU)
	mockVU.CtxField = ctx

	return mockVU
}
