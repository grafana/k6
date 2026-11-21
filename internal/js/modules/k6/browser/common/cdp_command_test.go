package common

import (
	"context"
	"testing"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"
	jsonv2 "github.com/go-json-experiment/json"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/log"
)

type protocolExecutorFunc func(context.Context, string, any, any) error

func (f protocolExecutorFunc) Execute(ctx context.Context, method string, params, result any) error {
	return f(ctx, method, params, result)
}

func TestProtocolCommandBinaryData(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	payload := []byte{0, 1, 0xff}
	command := newCommand(fetch.ContinueRequest, fetch.ContinueRequestParams{RequestID: "request", PostData: payload})
	err := command.Do(ctx, protocolExecutorFunc(func(_ context.Context, method string, params, result any) error {
		require.Equal(t, fetch.CommandContinueRequest, method)
		require.Nil(t, result)
		encoded, err := jsonv2.Marshal(params)
		require.NoError(t, err)
		require.JSONEq(t, `{"requestId":"request","postData":"AAH/"}`, string(encoded))
		return nil
	}))
	require.NoError(t, err)
	result, err := newCommand(network.GetResponseBody, network.GetResponseBodyParams{RequestID: "request"}).Result(ctx,
		protocolExecutorFunc(func(_ context.Context, _ string, _, result any) error {
			return jsonv2.Unmarshal([]byte(`{"body":"AAH/","base64Encoded":true}`), result)
		}))
	require.NoError(t, err)
	require.Equal(t, payload, result.Body)
}

func TestProtocolTime(t *testing.T) {
	t.Parallel()
	require.Equal(t, time.Unix(1700000000, 250000000), protocolTime(1700000000.25))
	require.Equal(t, time.Unix(12, 500000000), protocolTime(float64(cdp.MonotonicTime(12.5))))
}

func TestExecutionContextDestroyedUniqueID(t *testing.T) {
	t.Parallel()
	destroyed := &ExecutionContext{id: 1, uniqueID: "destroyed"}
	retained := &ExecutionContext{id: 2, uniqueID: "retained"}
	fs := &FrameSession{
		logger: log.NewNullLogger(), session: &Session{},
		contextIDToContext: map[runtime.ExecutionContextID]*ExecutionContext{1: destroyed, 2: retained},
	}
	fs.onExecutionContextDestroyed("unknown")
	require.Len(t, fs.contextIDToContext, 2)
	fs.onExecutionContextDestroyed("destroyed")
	require.Equal(t, map[runtime.ExecutionContextID]*ExecutionContext{2: retained}, fs.contextIDToContext)
}
