package common

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"testing"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/runtime"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/js/modules/k6/browser/log"
)

func newExecCtx() (*ExecutionContext, context.Context) {
	ctx := context.Background()
	logger := log.NewNullLogger()
	execCtx := NewExecutionContext(ctx, nil, nil, runtime.ExecutionContextID(123456789), logger)

	return execCtx, ctx
}

func TestConvertArgument(t *testing.T) {
	t.Parallel()

	t.Run("int64", func(t *testing.T) {
		t.Parallel()

		execCtx, ctx := newExecCtx()
		var value int64 = 777
		arg, _ := convertArgument(ctx, execCtx, value)

		require.NotNil(t, arg)
		result, _ := json.Marshal(value)
		require.Equal(t, result, []byte(arg.Value))
		require.Empty(t, arg.UnserializableValue)
		require.Empty(t, arg.ObjectID)
	})

	t.Run("int64 maxint", func(t *testing.T) {
		t.Parallel()

		execCtx, ctx := newExecCtx()

		var value int64 = math.MaxInt32 + 1
		arg, _ := convertArgument(ctx, execCtx, value)

		require.NotNil(t, arg)
		require.Equal(t, fmt.Sprintf("%dn", value), string(arg.UnserializableValue))
		require.Empty(t, arg.Value)
		require.Empty(t, arg.ObjectID)
	})

	t.Run("float64", func(t *testing.T) {
		t.Parallel()

		execCtx, ctx := newExecCtx()

		var value float64 = 777.0
		arg, _ := convertArgument(ctx, execCtx, value)

		require.NotNil(t, arg)
		result, _ := json.Marshal(value)
		require.Equal(t, result, []byte(arg.Value))
		require.Empty(t, arg.UnserializableValue)
		require.Empty(t, arg.ObjectID)
	})

	t.Run("float64 unserializable values", func(t *testing.T) {
		t.Parallel()

		execCtx, ctx := newExecCtx()

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
			arg, _ := convertArgument(ctx, execCtx, v.value)
			require.NotNil(t, arg)
			require.Equal(t, v.expected, string(arg.UnserializableValue))
			require.Empty(t, arg.Value)
			require.Empty(t, arg.ObjectID)
		}
	})

	t.Run("bool", func(t *testing.T) {
		t.Parallel()

		execCtx, ctx := newExecCtx()

		value := true
		arg, _ := convertArgument(ctx, execCtx, value)

		require.NotNil(t, arg)
		result, _ := json.Marshal(value)
		require.Equal(t, result, []byte(arg.Value))
		require.Empty(t, arg.UnserializableValue)
		require.Empty(t, arg.ObjectID)
	})

	t.Run("string", func(t *testing.T) {
		t.Parallel()

		execCtx, ctx := newExecCtx()
		value := "hello world"
		arg, _ := convertArgument(ctx, execCtx, value)

		require.NotNil(t, arg)
		result, _ := json.Marshal(value)
		require.Equal(t, result, []byte(arg.Value))
		require.Empty(t, arg.UnserializableValue)
		require.Empty(t, arg.ObjectID)
	})

	t.Run("*BaseJSHandle", func(t *testing.T) {
		t.Parallel()

		execCtx, ctx := newExecCtx()
		log := log.NewNullLogger()

		timeoutSettings := NewTimeoutSettings(nil)
		frameManager := NewFrameManager(ctx, nil, nil, timeoutSettings, log)
		frame := NewFrame(ctx, frameManager, nil, cdp.FrameID("frame_id_0123456789"), log)
		remoteObjValue := "hellow world"
		result, _ := json.Marshal(remoteObjValue)
		remoteObject := &runtime.RemoteObject{
			Type:  "string",
			Value: result,
		}

		value := NewJSHandle(ctx, nil, execCtx, frame, remoteObject, execCtx.logger)
		arg, _ := convertArgument(ctx, execCtx, value)

		require.NotNil(t, arg)
		require.Equal(t, result, []byte(arg.Value))
		require.Empty(t, arg.UnserializableValue)
		require.Empty(t, arg.ObjectID)
	})

	t.Run("*BaseJSHandle wrong context", func(t *testing.T) {
		t.Parallel()

		execCtx, ctx := newExecCtx()
		log := log.NewNullLogger()

		timeoutSettings := NewTimeoutSettings(nil)
		frameManager := NewFrameManager(ctx, nil, nil, timeoutSettings, log)
		frame := NewFrame(ctx, frameManager, nil, cdp.FrameID("frame_id_0123456789"), log)
		remoteObjectID := runtime.RemoteObjectID("object_id_0123456789")
		remoteObject := &runtime.RemoteObject{
			Type:     "object",
			Subtype:  "node",
			ObjectID: remoteObjectID,
		}
		execCtx2 := NewExecutionContext(ctx, nil, nil, runtime.ExecutionContextID(123456789), execCtx.logger)

		value := NewJSHandle(ctx, nil, execCtx2, frame, remoteObject, execCtx.logger)
		arg, err := convertArgument(ctx, execCtx, value)

		require.Nil(t, arg)
		require.ErrorIs(t, ErrWrongExecutionContext, err)
	})

	t.Run("*BaseJSHandle is disposed", func(t *testing.T) {
		t.Parallel()

		execCtx, ctx := newExecCtx()
		log := log.NewNullLogger()

		timeoutSettings := NewTimeoutSettings(nil)
		frameManager := NewFrameManager(ctx, nil, nil, timeoutSettings, log)
		frame := NewFrame(ctx, frameManager, nil, cdp.FrameID("frame_id_0123456789"), log)
		remoteObjectID := runtime.RemoteObjectID("object_id_0123456789")
		remoteObject := &runtime.RemoteObject{
			Type:     "object",
			Subtype:  "node",
			ObjectID: remoteObjectID,
		}

		value := NewJSHandle(ctx, nil, execCtx, frame, remoteObject, execCtx.logger)
		value.(*BaseJSHandle).disposed = true
		arg, err := convertArgument(ctx, execCtx, value)

		require.Nil(t, arg)
		require.ErrorIs(t, ErrJSHandleDisposed, err)
	})

	t.Run("*BaseJSHandle as *ElementHandle", func(t *testing.T) {
		t.Parallel()

		execCtx, ctx := newExecCtx()
		log := log.NewNullLogger()

		timeoutSettings := NewTimeoutSettings(nil)
		frameManager := NewFrameManager(ctx, nil, nil, timeoutSettings, log)
		frame := NewFrame(ctx, frameManager, nil, cdp.FrameID("frame_id_0123456789"), log)
		remoteObjectID := runtime.RemoteObjectID("object_id_0123456789")
		remoteObject := &runtime.RemoteObject{
			Type:     "object",
			Subtype:  "node",
			ObjectID: remoteObjectID,
		}

		value := NewJSHandle(ctx, nil, execCtx, frame, remoteObject, execCtx.logger)
		arg, _ := convertArgument(ctx, execCtx, value)

		require.NotNil(t, arg)
		require.Equal(t, remoteObjectID, arg.ObjectID)
		require.Empty(t, arg.Value)
		require.Empty(t, arg.UnserializableValue)
	})
}
