package streams

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadableStreamPipeToTransfersAndCloses(t *testing.T) {
	t.Parallel()

	result := runStreamPromiseScript(t, `
const chunks = [];
let closed = false;
const source = new ReadableStream({
  start(controller) {
    controller.enqueue("a");
    controller.enqueue("b");
    controller.close();
  },
});
const destination = new WritableStream({
  write(chunk) { chunks.push(chunk); },
  close() { closed = true; },
});

const piping = source.pipeTo(destination);
const lockedDuringPipe = source.locked && destination.locked;
await piping;
return JSON.stringify([chunks, closed, lockedDuringPipe, source.locked, destination.locked]);
`)

	require.Equal(t, `[["a","b"],true,true,false,false]`, result.String())
}

func TestReadableStreamPipeToPropagation(t *testing.T) {
	t.Parallel()

	result := runStreamPromiseScript(t, `
const sourceError = new Error("source");
let abortReason;
const source = new ReadableStream({ start(controller) { controller.error(sourceError); } });
const destination = new WritableStream({ abort(reason) { abortReason = reason; } });
const forward = await source.pipeTo(destination).then(
  () => false,
  reason => reason === sourceError && abortReason === sourceError,
);

const destinationError = new Error("destination");
let cancelReason;
const backwardSource = new ReadableStream({
  start(controller) { controller.enqueue("x"); },
  cancel(reason) { cancelReason = reason; },
});
const backwardDestination = new WritableStream({ write() { throw destinationError; } });
const backward = await backwardSource.pipeTo(backwardDestination).then(
  () => false,
  reason => reason === destinationError && cancelReason === destinationError,
);

return JSON.stringify([forward, backward]);
`)

	require.Equal(t, `[true,true]`, result.String())
}

func TestReadableStreamPipeToWaitsForFinalWriteBeforeShutdown(t *testing.T) {
	t.Parallel()

	result := runStreamPromiseScript(t, `
let controller;
let resolveWriteStarted;
const writeStarted = new Promise(resolve => { resolveWriteStarted = resolve; });
let resolveWrite;
const writePending = new Promise(resolve => { resolveWrite = resolve; });
const sourceError = new Error("source");
const source = new ReadableStream({ start(c) { controller = c; } });
const destination = new WritableStream({
  write() {
    resolveWriteStarted();
    return writePending;
  },
});

let settled = false;
const piping = source.pipeTo(destination, { preventAbort: true }).then(
  () => { settled = true; },
  () => { settled = true; },
);
controller.enqueue("a");
await writeStarted;
controller.error(sourceError);
await Promise.resolve();
await Promise.resolve();
const settledBeforeWrite = settled;
resolveWrite();
await piping;

return JSON.stringify([settledBeforeWrite, source.locked, destination.locked]);
`)

	require.Equal(t, `[false,false,false]`, result.String())
}

func TestReadableStreamPipeToPreventionOptions(t *testing.T) {
	t.Parallel()

	result := runStreamPromiseScript(t, `
let closed = false;
const source = new ReadableStream({ start(controller) { controller.close(); } });
const destination = new WritableStream({ close() { closed = true; } });
await source.pipeTo(destination, { preventClose: true });
const writer = destination.getWriter();
const remainedWritable = writer.desiredSize !== null;
writer.releaseLock();

let canceled = false;
const source2 = new ReadableStream({
  start(controller) { controller.enqueue("x"); },
  cancel() { canceled = true; },
});
const destination2 = new WritableStream({ write() { throw new Error("boom"); } });
await source2.pipeTo(destination2, { preventCancel: true }).catch(() => {});

return JSON.stringify([closed, remainedWritable, canceled]);
`)

	require.Equal(t, `[false,true,false]`, result.String())
}

func TestReadableStreamPipeToOptionsAndPreflight(t *testing.T) {
	t.Parallel()

	result := runStreamPromiseScript(t, `
const order = [];
const options = {};
for (const name of ["preventAbort", "preventCancel", "preventClose", "signal"]) {
  Object.defineProperty(options, name, {
    get() { order.push(name); return undefined; },
  });
}
const source = new ReadableStream({ start(controller) { controller.close(); } });
await source.pipeTo(new WritableStream(), options);

const lockedSource = new ReadableStream();
const reader = lockedSource.getReader();
const unlockedDestination = new WritableStream();
const rejectedLocked = await lockedSource.pipeTo(unlockedDestination).then(
  () => false,
  error => error instanceof TypeError && !unlockedDestination.locked,
);
reader.releaseLock();

const signalRejected = await new ReadableStream().pipeTo(new WritableStream(), { signal: {} }).then(
  () => false,
  error => error && error.name === "NotSupportedError",
);

return JSON.stringify([order, rejectedLocked, signalRejected]);
`)

	require.Equal(t,
		`[["preventAbort","preventCancel","preventClose","signal"],true,true]`, result.String())
}

func TestReadableStreamPipeThrough(t *testing.T) {
	t.Parallel()

	result := runStreamPromiseScript(t, `
const source = new ReadableStream({
  start(controller) {
    controller.enqueue("a");
    controller.close();
  },
});
const transform = new TransformStream({ transform(chunk, controller) { controller.enqueue(chunk + "!"); } });
const readable = source.pipeThrough(transform);
const reader = readable.getReader();
const first = await reader.read();
const second = await reader.read();

const pairReadable = new ReadableStream();
const pairWritable = new WritableStream();
const returned = new ReadableStream({ start(controller) { controller.close(); } })
  .pipeThrough({ readable: pairReadable, writable: pairWritable }, { preventClose: true });

return JSON.stringify([first.value, first.done, second.done, returned === pairReadable]);
`)

	require.Equal(t, `["a!",false,true,true]`, result.String())
}

func TestReadableStreamPipeThroughThrowsBeforeLocking(t *testing.T) {
	t.Parallel()

	result := runStreamScript(t, `
const source = new ReadableStream();
const destination = new WritableStream();
const order = [];
const pair = {
  get readable() { order.push("readable"); return new ReadableStream(); },
  get writable() { order.push("writable"); return destination; },
};
const options = {
  get preventAbort() { order.push("preventAbort"); throw new Error("boom"); },
};
let threw = false;
try {
  source.pipeThrough(pair, options);
} catch (error) {
  threw = error.message === "boom";
}
JSON.stringify([order, threw, source.locked, destination.locked]);
`)

	require.Equal(t, `[["readable","writable","preventAbort"],true,false,false]`, result.String())
}
