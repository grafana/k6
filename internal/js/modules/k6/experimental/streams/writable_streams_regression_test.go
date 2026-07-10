package streams

import (
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/js/modulestest"
)

func newStreamsRuntime(t testing.TB) *modulestest.Runtime {
	t.Helper()

	rt := modulestest.NewRuntime(t)
	m := new(RootModule).NewModuleInstance(rt.VU)
	for k, v := range m.Exports().Named {
		require.NoError(t, rt.VU.RuntimeField.Set(k, v))
	}

	return rt
}

func runStreamScript(t testing.TB, script string) sobek.Value {
	t.Helper()

	rt := newStreamsRuntime(t)
	value, err := rt.RunOnEventLoop(script)
	require.NoError(t, err)

	return value
}

func TestStreamControllerConstructorsThrowWhenCalled(t *testing.T) {
	t.Parallel()

	result := runStreamScript(t, `
const results = [];

new WritableStream({
  start(controller) {
    try {
      controller.constructor().error(new Error("boom"));
      results.push("writable did not throw");
    } catch (error) {
      results.push(error instanceof TypeError);
    }
  },
});

new ReadableStream({
  start(controller) {
    try {
      controller.constructor().error(new Error("boom"));
      results.push("readable did not throw");
    } catch (error) {
      results.push(error instanceof TypeError);
    }
  },
});

JSON.stringify(results);
`)

	require.Equal(t, `[true,true]`, result.String())
}

func TestWritableStreamUnderlyingSinkDictionaryPresence(t *testing.T) {
	t.Parallel()

	result := runStreamScript(t, `
const results = [];

try {
  new WritableStream({
    start: undefined,
    write: undefined,
    close: undefined,
    abort: undefined,
  });
  results.push("undefined callbacks ignored");
} catch (error) {
  results.push(error && error.message);
}

try {
  new WritableStream({ type: null });
  results.push("type null accepted");
} catch (error) {
  results.push(error instanceof RangeError);
}

JSON.stringify(results);
`)

	require.Equal(t, `["undefined callbacks ignored",true]`, result.String())
}
