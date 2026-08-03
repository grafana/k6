package streams

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTransformStreamUsesCachedPrototypes(t *testing.T) {
	t.Parallel()

	result := runStreamScript(t, `
const originalWritableStream = WritableStream;
const originalWritableStreamDefaultWriter = WritableStreamDefaultWriter;
const stream = new TransformStream();
const readableDescriptor = Object.getOwnPropertyDescriptor(TransformStream.prototype, "readable");
const writableDescriptor = Object.getOwnPropertyDescriptor(TransformStream.prototype, "writable");

WritableStream = function PatchedWritableStream() {};
const streamAfterPatch = new TransformStream();

JSON.stringify([
  Object.getPrototypeOf(stream) === TransformStream.prototype,
  !Object.hasOwn(stream, "readable"),
  !Object.hasOwn(stream, "writable"),
  typeof readableDescriptor.get === "function",
  readableDescriptor.enumerable,
  readableDescriptor.configurable,
  typeof writableDescriptor.get === "function",
  writableDescriptor.enumerable,
  writableDescriptor.configurable,
  Object.getPrototypeOf(stream.writable) === originalWritableStream.prototype,
  Object.getPrototypeOf(stream.writable.getWriter()) ===
    originalWritableStreamDefaultWriter.prototype,
  Object.getPrototypeOf(streamAfterPatch.writable) === originalWritableStream.prototype,
]);
`)

	require.Equal(t, `[true,true,true,true,true,true,true,true,true,true,true,true]`, result.String())
}

func TestTransformStreamControllerConstructorThrows(t *testing.T) {
	t.Parallel()

	result := runStreamScript(t, `
let controller;
new TransformStream({
  start(value) {
    controller = value;
  },
});

const results = [];
try {
  controller.constructor();
  results.push("call did not throw");
} catch (error) {
  results.push(error instanceof TypeError);
}

try {
  new controller.constructor();
  results.push("construction did not throw");
} catch (error) {
  results.push(error instanceof TypeError);
}

JSON.stringify(results);
`)

	require.Equal(t, `[true,true]`, result.String())
}

func TestTransformStreamIgnoresUndefinedTransformerMembers(t *testing.T) {
	t.Parallel()

	result := runStreamScript(t, `
const stream = new TransformStream({
  start: undefined,
  transform: undefined,
  flush: undefined,
  cancel: undefined,
  readableType: undefined,
  writableType: undefined,
});

stream instanceof TransformStream;
`)

	require.True(t, result.ToBoolean())
}

func TestTransformStreamTransactionRestoresDepthAfterPanic(t *testing.T) {
	t.Parallel()

	stream := &TransformStream{}
	require.Panics(t, func() {
		stream.withTransaction(func() { panic("boom") })
	})
	require.Zero(t, stream.txDepth)
}
