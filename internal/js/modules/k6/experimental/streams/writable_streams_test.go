package streams

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/compiler"
	"go.k6.io/k6/v2/js/modulestest"
)

func TestWritableStreamUsesCanonicalPrototypes(t *testing.T) {
	t.Parallel()

	runtime := modulestest.NewRuntime(t)
	err := runtime.SetupModuleSystem(
		map[string]any{"k6/experimental/streams": New()},
		nil,
		compiler.New(runtime.VU.InitEnv().Logger),
	)
	require.NoError(t, err)

	_, err = runtime.VU.Runtime().RunString(`
const {
  WritableStream,
  WritableStreamDefaultWriter,
} = require("k6/experimental/streams");

function assert(condition, message) {
  if (!condition) {
    throw new Error(message);
  }
}

function assertThrowsTypeError(fn, message) {
  try {
    fn();
  } catch (error) {
    assert(error instanceof TypeError, message + ": expected TypeError, got " + error);
    return;
  }
  throw new Error(message + ": expected function to throw");
}

assert(
  Object.prototype.hasOwnProperty.call(WritableStream.prototype, "locked"),
  "WritableStream.prototype must be initialized when the module is instantiated",
);
assert(
  Object.prototype.hasOwnProperty.call(WritableStreamDefaultWriter.prototype, "closed"),
  "WritableStreamDefaultWriter.prototype must be initialized when the module is instantiated",
);

const stream = new WritableStream({ write() {} });
const writer = stream.getWriter();

assert(stream instanceof WritableStream, "stream must use the exported constructor prototype");
assert(writer instanceof WritableStreamDefaultWriter, "writer must use the exported constructor prototype");
assert(
  Object.getPrototypeOf(writer) === WritableStreamDefaultWriter.prototype,
  "vended writer must use WritableStreamDefaultWriter.prototype",
);
assert(
  writer.constructor === WritableStreamDefaultWriter,
  "vended writer must retain the exported constructor",
);

const locked = Object.getOwnPropertyDescriptor(WritableStream.prototype, "locked");
assert(locked.configurable === true, "locked must be configurable");
assert(locked.enumerable === false, "locked must not be enumerable");

const closed = Object.getOwnPropertyDescriptor(WritableStreamDefaultWriter.prototype, "closed");
assert(closed.configurable === true, "closed must be configurable");
assert(closed.enumerable === false, "closed must not be enumerable");

const write = Object.getOwnPropertyDescriptor(WritableStreamDefaultWriter.prototype, "write");
assert(write.writable === true, "write must be writable");
assert(write.configurable === true, "write must be configurable");
assert(write.enumerable === false, "write must not be enumerable");

writer.releaseLock();

assertThrowsTypeError(
  () => WritableStreamDefaultWriter.call({}, stream),
  "writer constructor must reject a foreign receiver",
);
assertThrowsTypeError(
  () => WritableStream.call({}, {}),
  "stream constructor must reject a foreign receiver",
);

function ForeignStream() {}
Reflect.construct(WritableStream, [{}], ForeignStream);
assert(
  !Object.prototype.hasOwnProperty.call(ForeignStream.prototype, "locked"),
  "derived construction must not modify the new target prototype",
);

function ForeignWriter() {}
Reflect.construct(WritableStreamDefaultWriter, [stream], ForeignWriter);
for (const name of ["closed", "desiredSize", "ready", "abort", "close", "releaseLock", "write"]) {
  assert(
    !Object.prototype.hasOwnProperty.call(ForeignWriter.prototype, name),
    "derived construction must not install " + name + " on the new target prototype",
  );
}

for (const name of ["locked", "closed", "desiredSize", "ready", "abort", "close", "releaseLock", "write"]) {
  assert(
    !Object.prototype.hasOwnProperty.call(Object.prototype, name),
    "constructor calls must not install " + name + " on Object.prototype",
  );
}
`)
	require.NoError(t, err)
}

func TestWritableStream(t *testing.T) {
	t.Parallel()
	if _, err := os.Stat(webPlatformTestSuite); err != nil { //nolint:forbidigo
		t.Skipf("If you want to run Streams tests, you need to run the 'checkout.sh` script in the directory to get "+
			"https://github.com/web-platform-tests/wpt at the correct last tested commit (%v)", err)
	}

	suites := []string{
		"aborting.any.js",
		"bad-strategies.any.js",
		"bad-underlying-sinks.any.js",
		"close.any.js",
		"constructor.any.js",
		"count-queuing-strategy.any.js",
		"error.any.js",
		"floating-point-total-queue-size.any.js",
		"general.any.js",
		"properties.any.js",
		"reentrant-strategy.any.js",
		"start.any.js",
		"write.any.js",
	}

	for _, suite := range suites {
		t.Run(suite, func(t *testing.T) {
			t.Parallel()
			ts := newConfiguredRuntime(t)
			gotErr := ts.EventLoop.Start(func() error {
				return executeTestScript(ts.VU, webPlatformTestSuite+"/streams/writable-streams", suite)
			})
			assert.NoError(t, gotErr)
		})
	}
}
