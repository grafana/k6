package mqtt

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/grafana/sobek"
	"github.com/grafana/xk6-mqtt/internal/broker"
	"github.com/stretchr/testify/require"
	extensionapi "go.k6.io/k6-extension-api"
	extensionapitest "go.k6.io/k6-extension-api/test"
)

func runScriptTest(t *testing.T, filename string) {
	t.Helper()
	runtime := extensionapitest.NewScriptRuntime(nil)
	host := newTestRuntime(t).VU
	runtime.VU.DialContextFunc = host.DialContextFunc
	runtime.VU.LookupHostFunc = host.LookupHostFunc
	runtime.VU.TLSClientFunc = host.TLSClientFunc
	runtime.VU.LookupEnvFunc = host.LookupEnvFunc
	runtime.VU.RegisterBuiltinMetric(extensionapi.BuiltinDataSent, "data_sent")
	runtime.VU.RegisterBuiltinMetric(extensionapi.BuiltinDataReceived, "data_received")
	runtime.SetExtension(ImportPath, New())
	runtime.SetModule("k6/x/assert", map[string]any{
		"true": func(call sobek.FunctionCall) sobek.Value {
			if !call.Argument(0).ToBoolean() {
				panic(runtime.VU.Runtime().NewGoError(errors.New(assertMessage(call))))
			}
			return sobek.Undefined()
		},
		"false": func(call sobek.FunctionCall) sobek.Value {
			if call.Argument(0).ToBoolean() {
				panic(runtime.VU.Runtime().NewGoError(errors.New(assertMessage(call))))
			}
			return sobek.Undefined()
		},
		"equal": func(call sobek.FunctionCall) sobek.Value {
			if !reflect.DeepEqual(call.Argument(0).Export(), call.Argument(1).Export()) {
				panic(runtime.VU.Runtime().NewGoError(errors.New(assertMessage(call))))
			}
			return sobek.Undefined()
		},
	})
	brokerAddress := os.Getenv(broker.EnvBrokerAddress) //nolint:forbidigo // embedded broker address
	require.NotEmpty(t, brokerAddress)
	require.NoError(t, runtime.VU.Runtime().Set("__ENV", map[string]string{broker.EnvBrokerAddress: brokerAddress}))
	exports, err := runtime.RunFile(filename)
	require.NoError(t, err)
	obj := exports.ToObject(runtime.VU.Runtime())
	if setup, ok := sobek.AssertFunction(obj.Get("setup")); ok {
		_, err = runtime.Call(setup)
		require.NoError(t, err)
	}
	fn, ok := sobek.AssertFunction(exports)
	require.True(t, ok, "module.exports should be a function")
	_, err = runtime.Call(fn)
	require.NoError(t, err)
	if teardown, ok := sobek.AssertFunction(obj.Get("teardown")); ok {
		_, err = runtime.Call(teardown)
		require.NoError(t, err)
	}
}

func assertMessage(call sobek.FunctionCall) string {
	if len(call.Arguments) > 1 {
		return call.Argument(1).String()
	}
	return "assertion failed"
}

func Test_script(t *testing.T) {
	files, err := filepath.Glob("testdata/*_test.cjs")
	require.NoError(t, err)
	require.NotEmpty(t, files)
	for _, file := range files {
		t.Run(filepath.ToSlash(file), func(t *testing.T) { runScriptTest(t, file) })
	}
}
