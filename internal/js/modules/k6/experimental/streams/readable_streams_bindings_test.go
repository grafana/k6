package streams

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadableStreamConstructorsRequireNew(t *testing.T) {
	t.Parallel()

	result := runStreamScript(t, `
JSON.stringify([
  (() => { try { ReadableStream(); return false; } catch (e) { return e instanceof TypeError; } })(),
  (() => { try { ReadableStreamDefaultReader({}); return false; } catch (e) { return e instanceof TypeError; } })(),
]);
`)

	require.Equal(t, `[true,true]`, result.String())
}

func TestReadableStreamDefaultControllerEnqueuesUndefinedWhenChunkIsOmitted(t *testing.T) {
	t.Parallel()

	result := runStreamPromiseScript(t, `
const stream = new ReadableStream({
  start(controller) {
    controller.enqueue();
    controller.close();
  },
});
const reader = stream.getReader();
const first = await reader.read();
const second = await reader.read();
return JSON.stringify([first.value === undefined, first.done, second.done]);
`)

	require.Equal(t, `[true,false,true]`, result.String())
}

func TestReadableStreamBindingsUseCanonicalPrototypes(t *testing.T) {
	t.Parallel()

	result := runStreamScript(t, `
const stream = new ReadableStream();
const reader = stream.getReader();
const transform = new TransformStream();
const methodNames = ["cancel", "getReader", "tee"];
const readerMethodNames = ["cancel", "read", "releaseLock"];

JSON.stringify({
  streamOwnMethods: methodNames.some(name => Object.hasOwn(stream, name)),
  readerOwnMethods: readerMethodNames.some(name => Object.hasOwn(reader, name)),
  streamExtensible: Object.isExtensible(stream),
  readerExtensible: Object.isExtensible(reader),
  streamPrototype: Object.getPrototypeOf(stream) === ReadableStream.prototype,
  readerPrototype: Object.getPrototypeOf(reader) === ReadableStreamDefaultReader.prototype,
  transformReadablePrototype:
    Object.getPrototypeOf(transform.readable) === ReadableStream.prototype,
  transformWritablePrototype:
    Object.getPrototypeOf(transform.writable) === WritableStream.prototype,
  methodDescriptor: Object.getOwnPropertyDescriptor(ReadableStream.prototype, "getReader"),
  getterDescriptor: Object.getOwnPropertyDescriptor(ReadableStream.prototype, "locked"),
});
`)

	require.JSONEq(t, `{
  "streamOwnMethods": false,
  "readerOwnMethods": false,
  "streamExtensible": true,
  "readerExtensible": true,
  "streamPrototype": true,
  "readerPrototype": true,
  "transformReadablePrototype": true,
  "transformWritablePrototype": true,
  "methodDescriptor": {"writable": true, "enumerable": true, "configurable": true},
  "getterDescriptor": {"enumerable": true, "configurable": true}
}`, result.String())
}

func TestReadableStreamPrototypeBrandChecks(t *testing.T) {
	t.Parallel()

	result := runStreamPromiseScript(t, `
const streamPrototype = ReadableStream.prototype;
const readerPrototype = ReadableStreamDefaultReader.prototype;
const results = [];

for (const getter of [
  () => Reflect.get(streamPrototype, "locked", {}),
  () => Reflect.get(readerPrototype, "closed", {}),
]) {
  try {
    getter();
    results.push(false);
  } catch (error) {
    results.push(error instanceof TypeError);
  }
}

for (const operation of [
  streamPrototype.cancel.call({}),
  readerPrototype.cancel.call({}),
  readerPrototype.read.call({}),
]) {
  results.push(await operation.then(() => false, error => error instanceof TypeError));
}

for (const operation of [
  () => streamPrototype.getReader.call({}),
  () => streamPrototype.tee.call({}),
  () => readerPrototype.releaseLock.call({}),
]) {
  try {
    operation();
    results.push(false);
  } catch (error) {
    results.push(error instanceof TypeError);
  }
}

return JSON.stringify(results);
`)

	require.Equal(t, `[true,true,true,true,true,true,true,true]`, result.String())
}

func TestReadableStreamDerivedNewTargetPrototype(t *testing.T) {
	t.Parallel()

	result := runStreamScript(t, `
function Derived() {}
Derived.prototype = Object.create(ReadableStream.prototype);
const stream = Reflect.construct(ReadableStream, [], Derived);
const reader = stream.getReader();

JSON.stringify([
  Object.getPrototypeOf(stream) === Derived.prototype,
  Object.getPrototypeOf(reader) === ReadableStreamDefaultReader.prototype,
  stream instanceof ReadableStream,
]);
`)

	require.Equal(t, `[true,true,true]`, result.String())
}
