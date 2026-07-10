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

func runStreamPromiseScript(t testing.TB, script string) sobek.Value {
	t.Helper()

	rt := newStreamsRuntime(t)
	callback := rt.VU.RegisterCallback()
	var result sobek.Value

	require.NoError(t, rt.VU.Runtime().Set("__streamTestDone", func(value sobek.Value) {
		result = value
		callback(func() error { return nil })
	}))

	_, err := rt.RunOnEventLoop(`
Promise.resolve((async () => {
` + script + `
})()).then(
  value => __streamTestDone(value),
  error => __streamTestDone("ERROR:" + (error && error.message || error))
);
`)
	require.NoError(t, err)
	require.NotNil(t, result)

	return result
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

func TestWritableStreamReusesControllerObject(t *testing.T) {
	t.Parallel()

	result := runStreamPromiseScript(t, `
let startController;
let writeController;
let marker;

const stream = new WritableStream({
  start(controller) {
    startController = controller;
    controller.marker = "kept";
  },
  write(_chunk, controller) {
    writeController = controller;
    marker = controller.marker;
  },
});

const writer = stream.getWriter();
await writer.write("chunk");

return JSON.stringify([writeController === startController, marker]);
`)

	require.Equal(t, `[true,"kept"]`, result.String())
}

func TestWritableStreamSizeConversionAbruptCompletionRejectsWrite(t *testing.T) {
	t.Parallel()

	result := runStreamPromiseScript(t, `
const boom = new Error("sz");
const stream = new WritableStream({
  write() {},
}, {
  size() {
    return {
      valueOf() {
        throw boom;
      },
    };
  },
});
const writer = stream.getWriter();

let writePromise;
try {
  writePromise = writer.write("chunk");
} catch (error) {
  return "threw:" + error.message;
}

const writeResult = await writePromise.then(
  () => "write fulfilled",
  error => error === boom ? "write rejected with boom" : "write rejected with " + error.message,
);
const closeResult = await writer.close().then(
  () => "close fulfilled",
  error => error ? "close rejected" : "close rejected without error",
);

return JSON.stringify([writeResult, closeResult]);
`)

	require.Equal(t, `["write rejected with boom","close rejected"]`, result.String())
}

func TestWritableStreamQueuedReadyRejectionWinsOverQueuedFulfillment(t *testing.T) {
	t.Parallel()

	result := runStreamPromiseScript(t, `
const boom = new Error("boom");
const stream = new WritableStream({
  write(_chunk, controller) {
    controller.error(boom);
  },
}, { highWaterMark: 1 });
const writer = stream.getWriter();

await writer.write("chunk").catch(() => {});

return writer.ready.then(
  () => "ready fulfilled",
  error => error === boom ? "ready rejected with boom" : "ready rejected with " + error.message,
);
`)

	require.Equal(t, "ready rejected with boom", result.String())
}

func TestWritableStreamDefaultWriterUsesConstructorPrototype(t *testing.T) {
	t.Parallel()

	result := runStreamPromiseScript(t, `
const stream = new WritableStream();
const writer = stream.getWriter();
const originalWrite = WritableStreamDefaultWriter.prototype.write;
let patchedCalled = false;

WritableStreamDefaultWriter.prototype.write = function(chunk) {
  patchedCalled = true;
  return originalWrite.call(this, chunk);
};

await writer.write("chunk");

return JSON.stringify([
  writer instanceof WritableStreamDefaultWriter,
  Object.prototype.hasOwnProperty.call(writer, "write"),
  patchedCalled,
]);
`)

	require.Equal(t, `[true,false,true]`, result.String())
}
