package streams

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadableStreamTeeDuplicatesChunksAndCloses(t *testing.T) {
	t.Parallel()

	result := runStreamPromiseScript(t, `
const shared = { value: "shared" };
const source = new ReadableStream({
  start(controller) {
    controller.enqueue(shared);
    controller.enqueue("second");
    controller.close();
  },
});

const branches = source.tee();
const lockedImmediately = source.locked;
const canonicalBranches = branches.every(branch =>
  branch instanceof ReadableStream && Object.getPrototypeOf(branch) === ReadableStream.prototype);

async function collect(stream) {
  const chunks = [];
  const reader = stream.getReader();
  for (;;) {
    const result = await reader.read();
    if (result.done) return chunks;
    chunks.push(result.value);
  }
}

const [left, right] = await Promise.all(branches.map(collect));
return JSON.stringify([
  lockedImmediately,
  canonicalBranches,
  left.length,
  right.length,
  left[0] === shared,
  right[0] === shared,
  left[0] === right[0],
  left[1],
  right[1],
]);
`)

	require.Equal(t, `[true,true,2,2,true,true,true,"second","second"]`, result.String())
}

func TestReadableStreamTeeBuffersForSlowerBranch(t *testing.T) {
	t.Parallel()

	result := runStreamPromiseScript(t, `
let next = 0;
const source = new ReadableStream({
  pull(controller) {
    controller.enqueue(++next);
    if (next === 4) controller.close();
  },
}, { highWaterMark: 0 });

const [fastBranch, slowBranch] = source.tee();

async function collect(stream) {
  const chunks = [];
  const reader = stream.getReader();
  for (;;) {
    const result = await reader.read();
    if (result.done) return chunks;
    chunks.push(result.value);
  }
}

const fast = await collect(fastBranch);
const slow = await collect(slowBranch);
return JSON.stringify([fast, slow, next]);
`)

	require.Equal(t, `[[1,2,3,4],[1,2,3,4],4]`, result.String())
}

func TestReadableStreamTeeDuplicatesAsynchronousChunks(t *testing.T) {
	t.Parallel()

	result := runStreamPromiseScript(t, `
let next = 0;
const source = new ReadableStream({
  pull(controller) {
    return new Promise(resolve => setTimeout(() => {
      controller.enqueue(++next);
      if (next === 3) controller.close();
      resolve();
    }, 0));
  },
}, { highWaterMark: 0 });
const branches = source.tee();

async function collect(stream) {
  const chunks = [];
  const reader = stream.getReader();
  for (;;) {
    const result = await reader.read();
    if (result.done) return chunks;
    chunks.push(result.value);
  }
}

return JSON.stringify(await Promise.all(branches.map(collect)));
`)

	require.Equal(t, `[[1,2,3],[1,2,3]]`, result.String())
}

func TestReadableStreamTeePullsToRefillEmptyBranchQueues(t *testing.T) {
	t.Parallel()

	result := runStreamPromiseScript(t, `
function recordingReadableStream(extras, strategy) {
  const source = new ReadableStream({
    pull(controller) {
      source.events.push("pull");
      return extras.pull(controller);
    },
  }, strategy);
  source.events = [];
  return source;
}
const idleSource = recordingReadableStream({ pull() {} }, { highWaterMark: 0 });
idleSource.tee();
await new Promise(resolve => setTimeout(resolve, 0));

let directPulls = 0;
const source = recordingReadableStream({
  pull(controller) { directPulls++; controller.enqueue("chunk"); },
}, { highWaterMark: 0 });
const beforeTee = source.events.length;
const [reader1, reader2] = source.tee().map(branch => branch.getReader());
const afterTee = source.events.length;
return Promise.all([reader1.read(), reader2.read()]).then(() =>
  JSON.stringify([beforeTee, afterTee, source.events.length, directPulls]));
`)

	require.Equal(t, `[0,0,2,2]`, result.String())
}

func TestReadableStreamTeeAggregatesCancellationReasonsInBranchOrder(t *testing.T) {
	t.Parallel()

	result := runStreamPromiseScript(t, `
async function cancelInOrder(reverse) {
  let sourceReason;
  let finishCancel;
  let cancelCalls = 0;
  const source = new ReadableStream({
    cancel(reason) {
      cancelCalls++;
      sourceReason = reason;
      return new Promise(resolve => { finishCancel = resolve; });
    },
  });
  const branches = source.tee();
  const reason1 = { branch: 1 };
  const reason2 = { branch: 2 };
  const firstIndex = reverse ? 1 : 0;
  const secondIndex = reverse ? 0 : 1;
  const reasons = [reason1, reason2];

  let firstSettled = false;
  const first = branches[firstIndex].cancel(reasons[firstIndex]);
  first.then(() => { firstSettled = true; });
  await Promise.resolve();
  await Promise.resolve();
  const waitedForOtherBranch = cancelCalls === 0 && !firstSettled;

  const second = branches[secondIndex].cancel(reasons[secondIndex]);
  const reasonsPreserved = Array.isArray(sourceReason) &&
    sourceReason[0] === reason1 && sourceReason[1] === reason2;
  finishCancel();
  await Promise.all([first, second]);

  return [waitedForOtherBranch, reasonsPreserved, cancelCalls, source.locked];
}

return JSON.stringify([
  await cancelInOrder(false),
  await cancelInOrder(true),
]);
`)

	require.Equal(t, `[[true,true,1,true],[true,true,1,true]]`, result.String())
}

func TestReadableStreamTeeCancellationFailureReachesBothBranches(t *testing.T) {
	t.Parallel()

	result := runStreamPromiseScript(t, `
const error = { name: "cancel failure" };
const source = new ReadableStream({ cancel() { throw error; } });
const [branch1, branch2] = source.tee();

const results = await Promise.all([
  branch1.cancel().then(() => false, reason => reason === error),
  branch2.cancel().then(() => false, reason => reason === error),
]);
return JSON.stringify(results);
`)

	require.Equal(t, `[true,true]`, result.String())
}

func TestReadableStreamTeeCanceledBranchSettlesWhenSourceTerminates(t *testing.T) {
	t.Parallel()

	result := runStreamPromiseScript(t, `
let controller;
const source = new ReadableStream({ start(c) { controller = c; } });
const [canceledBranch, activeBranch] = source.tee();
let cancellationSettled = false;
const cancellation = canceledBranch.cancel("unused").then(() => { cancellationSettled = true; });

controller.enqueue("kept");
const reader = activeBranch.getReader();
const first = await reader.read();
const pendingBeforeClose = !cancellationSettled;
controller.close();
const second = await reader.read();
await cancellation;

return JSON.stringify([first.value, first.done, second.done, pendingBeforeClose, cancellationSettled]);
`)

	require.Equal(t, `["kept",false,true,true,true]`, result.String())
}

func TestReadableStreamTeePropagatesSourceErrors(t *testing.T) {
	t.Parallel()

	result := runStreamPromiseScript(t, `
const error = { name: "source failure" };
let controller;
const source = new ReadableStream({ start(c) { controller = c; } });
const [branch1, branch2] = source.tee();
const reader1 = branch1.getReader();
const reader2 = branch2.getReader();

controller.enqueue("discarded");
controller.enqueue("also discarded");
await Promise.resolve();
controller.error(error);

const results = await Promise.all([
  reader1.closed.then(() => false, reason => reason === error),
  reader2.closed.then(() => false, reason => reason === error),
]);
return JSON.stringify(results);
`)

	require.Equal(t, `[true,true]`, result.String())
}

func TestReadableStreamTeePropagatesPreexistingSourceError(t *testing.T) {
	t.Parallel()

	result := runStreamPromiseScript(t, `
const error = { name: "preexisting source failure" };
let controller;
const source = new ReadableStream({ start(c) { controller = c; } });
controller.error(error);

const results = await Promise.all(source.tee().map(async branch => {
  const reader = branch.getReader();
  const readRejected = await reader.read().then(() => false, reason => reason === error);
  const closedRejected = await reader.closed.then(() => false, reason => reason === error);
  return readRejected && closedRejected;
}));
return JSON.stringify(results);
`)

	require.Equal(t, `[true,true]`, result.String())
}

func TestReadableStreamTeePullsOnceAndStopsAfterCancellation(t *testing.T) {
	t.Parallel()

	result := runStreamPromiseScript(t, `
let pulls = 0;
let cancelReason;
const source = new ReadableStream({
  pull() { pulls++; },
  cancel(reason) { cancelReason = reason; },
}, { highWaterMark: 0 });
const [branch1, branch2] = source.tee();

await Promise.resolve();
await Promise.resolve();
const pullsBeforeCancel = pulls;
await Promise.all([branch1.cancel("one"), branch2.cancel("two")]);
await Promise.resolve();
await Promise.resolve();

return JSON.stringify([
  pullsBeforeCancel,
  pulls,
  Array.isArray(cancelReason) && cancelReason[0] === "one" && cancelReason[1] === "two",
]);
`)

	require.Equal(t, `[1,1,true]`, result.String())
}

func TestReadableStreamTeeRejectsLockedSourceSynchronously(t *testing.T) {
	t.Parallel()

	result := runStreamScript(t, `
const source = new ReadableStream();
const reader = source.getReader();
let typeError = false;
try {
  source.tee();
} catch (error) {
  typeError = error instanceof TypeError;
}
const remainedLocked = source.locked;
reader.releaseLock();
JSON.stringify([typeError, remainedLocked, source.locked]);
`)

	require.Equal(t, `[true,true,false]`, result.String())
}
