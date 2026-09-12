package streams

import (
	"github.com/grafana/sobek"

	"go.k6.io/k6/v2/js/common"
	"go.k6.io/k6/v2/js/modules"
)

// WritableStreamDefaultWriter is the object returned by a WritableStream's getWriter() method,
// and represents a writer designed to be vended by a [WritableStream].
//
// [specification]: https://streams.spec.whatwg.org/#writablestreamdefaultwriter
type WritableStreamDefaultWriter struct {
	// closedPromise is fulfilled when the stream becomes closed, or rejected if the stream
	// errors or the writer's lock is released.
	closedPromise *promiseWrapper

	// readyPromise is fulfilled when the desired size of the stream's internal queue
	// transitions from non-positive to positive, signaling that it is no longer applying
	// backpressure.
	readyPromise *promiseWrapper

	// stream is the [WritableStream] instance that owns this writer, or nil once the writer's
	// lock has been released.
	stream *WritableStream

	runtime *sobek.Runtime
	vu      modules.VU
}

const writerGoRefKey = "__k6WritableStreamDefaultWriter__"

// NewWritableStreamDefaultWriterObject creates a new [sobek.Object] from a
// [WritableStreamDefaultWriter] instance.
func NewWritableStreamDefaultWriterObject(
	writer *WritableStreamDefaultWriter,
	proto *sobek.Object,
) (*sobek.Object, error) {
	rt := writer.runtime
	obj := rt.NewObject()

	if err := obj.DefineDataProperty(
		writerGoRefKey, rt.ToValue(writer), sobek.FLAG_FALSE, sobek.FLAG_FALSE, sobek.FLAG_FALSE,
	); err != nil {
		return nil, err
	}

	if proto == nil {
		return nil, newError(RuntimeError, "WritableStreamDefaultWriter prototype is not initialized")
	}

	if err := obj.SetPrototype(proto); err != nil {
		return nil, err
	}

	return obj, nil
}

func installWritableStreamDefaultWriterPrototype(rt *sobek.Runtime, proto *sobek.Object) error {
	if err := defineWriterGetter(rt, proto, "closed", func(this sobek.Value) *sobek.Promise {
		writer := writableStreamDefaultWriterFromThis(rt, this)
		return writer.closedPromise.promise
	}); err != nil {
		return err
	}

	if err := defineWriterGetter(rt, proto, "desiredSize", func(this sobek.Value) sobek.Value {
		writer := writableStreamDefaultWriterFromThis(rt, this)
		return writer.DesiredSize()
	}); err != nil {
		return err
	}

	if err := defineWriterGetter(rt, proto, "ready", func(this sobek.Value) *sobek.Promise {
		writer := writableStreamDefaultWriterFromThis(rt, this)
		return writer.readyPromise.promise
	}); err != nil {
		return err
	}

	if err := defineWriterMethod(rt, proto, "abort", func(this, reason sobek.Value) *sobek.Promise {
		writer := writableStreamDefaultWriterFromThis(rt, this)
		return writer.Abort(reason)
	}); err != nil {
		return err
	}

	if err := defineWriterMethod(rt, proto, "close", func(this, _ sobek.Value) *sobek.Promise {
		writer := writableStreamDefaultWriterFromThis(rt, this)
		return writer.Close()
	}); err != nil {
		return err
	}

	if err := defineWriterMethod(rt, proto, "releaseLock", func(this, _ sobek.Value) sobek.Value {
		writer := writableStreamDefaultWriterFromThis(rt, this)
		writer.ReleaseLock()
		return sobek.Undefined()
	}); err != nil {
		return err
	}

	return defineWriterMethod(rt, proto, "write", func(this, chunk sobek.Value) *sobek.Promise {
		writer := writableStreamDefaultWriterFromThis(rt, this)
		return writer.Write(chunk)
	})
}

func defineWriterGetter(rt *sobek.Runtime, proto *sobek.Object, name string, getter any) error {
	if hasOwnProperty(proto, name) {
		return nil
	}

	wrapped, err := wrapWriterGetter(rt, getter)
	if err != nil {
		return err
	}

	return proto.DefineAccessorProperty(name, wrapped, nil, sobek.FLAG_TRUE, sobek.FLAG_TRUE)
}

func defineWriterMethod(rt *sobek.Runtime, proto *sobek.Object, name string, method any) error {
	if hasOwnProperty(proto, name) {
		return nil
	}

	wrapped, err := wrapWriterMethod(rt, method)
	if err != nil {
		return err
	}

	return setDefaultPrototypePropertyOf(proto, name, wrapped)
}

func wrapWriterGetter(rt *sobek.Runtime, getter any) (sobek.Value, error) {
	return wrapWriterPrototypeFunction(rt, getter, `
(function(getter) {
  return function() {
    return getter(this);
  };
})`)
}

func wrapWriterMethod(rt *sobek.Runtime, method any) (sobek.Value, error) {
	return wrapWriterPrototypeFunction(rt, method, `
(function(method) {
  return function(arg) {
    return method(this, arg);
  };
})`)
}

func wrapWriterPrototypeFunction(rt *sobek.Runtime, callback any, source string) (sobek.Value, error) {
	wrapper, err := rt.RunString(source)
	if err != nil {
		return nil, err
	}

	call, ok := sobek.AssertFunction(wrapper)
	if !ok {
		return nil, newError(RuntimeError, "writer prototype wrapper is not a function")
	}

	return call(sobek.Undefined(), rt.ToValue(callback))
}

func writableStreamDefaultWriterFromValue(rt *sobek.Runtime, value sobek.Value) *WritableStreamDefaultWriter {
	if value == nil || common.IsNullish(value) {
		return nil
	}

	ref := value.ToObject(rt).Get(writerGoRefKey)
	if ref == nil {
		return nil
	}

	writer, _ := ref.Export().(*WritableStreamDefaultWriter)
	return writer
}

func writableStreamDefaultWriterFromThis(
	rt *sobek.Runtime,
	this sobek.Value,
) *WritableStreamDefaultWriter {
	writer := writableStreamDefaultWriterFromValue(rt, this)
	if writer == nil {
		throw(rt, newTypeError(rt, "value is not a WritableStreamDefaultWriter"))
	}

	return writer
}

// Abort aborts the stream, signaling that the producer can no longer successfully write to
// the stream.
//
// It implements the WritableStreamDefaultWriter.abort(reason) [specification] algorithm.
//
// [specification]: https://streams.spec.whatwg.org/#default-writer-abort
func (writer *WritableStreamDefaultWriter) Abort(reason sobek.Value) *sobek.Promise {
	if reason == nil {
		reason = sobek.Undefined()
	}

	// 1. If this.[[stream]] is undefined, return a promise rejected with a TypeError exception.
	stream := writer.stream
	if stream == nil {
		return newRejectedPromise(writer.vu, newTypeError(writer.runtime, "stream is undefined").Err())
	}

	// 2. Return ! WritableStreamDefaultWriterAbort(this, reason).
	var promise *sobek.Promise
	stream.withTransaction(func() {
		promise = writer.abort(reason)
	})
	return promise
}

// Close closes the stream.
//
// It implements the WritableStreamDefaultWriter.close() [specification] algorithm.
//
// [specification]: https://streams.spec.whatwg.org/#default-writer-close
func (writer *WritableStreamDefaultWriter) Close() *sobek.Promise {
	// 1. Let stream be this.[[stream]].
	stream := writer.stream

	// 2. If stream is undefined, return a promise rejected with a TypeError exception.
	if stream == nil {
		return newRejectedPromise(writer.vu, newTypeError(writer.runtime, "stream is undefined").Err())
	}

	// 3. If ! WritableStreamCloseQueuedOrInFlight(stream) is true, return a promise rejected with a TypeError.
	if stream.closeQueuedOrInFlight() {
		return newRejectedPromise(writer.vu, newTypeError(writer.runtime, "stream is already closing").Err())
	}

	// 4. Return ! WritableStreamDefaultWriterClose(this).
	var promise *sobek.Promise
	stream.withTransaction(func() {
		promise = writer.close()
	})
	return promise
}

// Write writes the given chunk to the writable stream, by waiting until any previous writes
// have finished successfully, and then sending the chunk to the underlying sink's write()
// method.
//
// It implements the WritableStreamDefaultWriter.write(chunk) [specification] algorithm.
//
// [specification]: https://streams.spec.whatwg.org/#default-writer-write
func (writer *WritableStreamDefaultWriter) Write(chunk sobek.Value) *sobek.Promise {
	if chunk == nil {
		chunk = sobek.Undefined()
	}

	// 1. If this.[[stream]] is undefined, return a promise rejected with a TypeError exception.
	stream := writer.stream
	if stream == nil {
		return newRejectedPromise(writer.vu, newTypeError(writer.runtime, "stream is undefined").Err())
	}

	// 2. Return ! WritableStreamDefaultWriterWrite(this, chunk).
	var promise *sobek.Promise
	stream.withTransaction(func() {
		promise = writer.write(chunk)
	})
	return promise
}

// ReleaseLock releases the writer's lock on the corresponding stream. After the lock is
// released, the writer is no longer active.
//
// It implements the WritableStreamDefaultWriter.releaseLock() [specification] algorithm.
//
// [specification]: https://streams.spec.whatwg.org/#default-writer-release-lock
func (writer *WritableStreamDefaultWriter) ReleaseLock() {
	// 1. Let stream be this.[[stream]].
	stream := writer.stream

	// 2. If stream is undefined, return.
	if stream == nil {
		return
	}

	// 3. Assert: stream.[[writer]] is not undefined.
	if stream.writer == nil {
		common.Throw(writer.runtime, newError(AssertionError, "stream has no writer"))
	}

	// 4. Perform ! WritableStreamDefaultWriterRelease(this).
	stream.withTransaction(func() {
		writer.release()
	})
}

// DesiredSize returns the desired size to fill the stream's internal queue.
//
// It implements the WritableStreamDefaultWriter.desiredSize [specification] getter.
//
// [specification]: https://streams.spec.whatwg.org/#default-writer-desired-size
func (writer *WritableStreamDefaultWriter) DesiredSize() sobek.Value {
	// 1. If this.[[stream]] is undefined, throw a TypeError exception.
	if writer.stream == nil {
		throw(writer.runtime, newTypeError(writer.runtime, "stream is undefined"))
	}

	// 2. Return ! WritableStreamDefaultWriterGetDesiredSize(this).
	return writer.getDesiredSize()
}

// setup implements the [specification]'s SetUpWritableStreamDefaultWriter abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#set-up-writable-stream-default-writer
func (writer *WritableStreamDefaultWriter) setup(stream *WritableStream) {
	rt := stream.runtime

	// 1. If ! IsWritableStreamLocked(stream) is true, throw a TypeError exception.
	if stream.isLocked() {
		throw(rt, newTypeError(rt, "stream is locked"))
	}

	// 2. Set writer.[[stream]] to stream.
	writer.stream = stream
	writer.runtime = rt
	writer.vu = stream.vu

	// 3. Set stream.[[writer]] to writer.
	stream.writer = writer

	// 4. Let state be stream.[[state]].
	state := stream.state

	switch state {
	// 5. If state is "writable",
	case WritableStreamStateWritable:
		// 5.1. If ! WritableStreamCloseQueuedOrInFlight(stream) is false and stream.[[backpressure]] is true,
		// set writer.[[readyPromise]] to a new promise.
		if !stream.closeQueuedOrInFlight() && stream.backpressure {
			writer.readyPromise = newPromiseWrapper(rt)
		} else {
			// 5.2. Otherwise, set writer.[[readyPromise]] to a promise resolved with undefined.
			writer.readyPromise = newResolvedPromiseWrapper(rt, sobek.Undefined())
		}
		// 5.3. Set writer.[[closedPromise]] to a new promise.
		writer.closedPromise = newPromiseWrapper(rt)
	// 6. Otherwise, if state is "erroring",
	case WritableStreamStateErroring:
		// 6.1. Set writer.[[readyPromise]] to a promise rejected with stream.[[storedError]].
		writer.readyPromise = newRejectedPromiseWrapper(rt, stream.storedError)
		// 6.2. Set writer.[[readyPromise]].[[PromiseIsHandled]] to true.
		markPromiseHandled(rt, writer.readyPromise.promise)
		// 6.3. Set writer.[[closedPromise]] to a new promise.
		writer.closedPromise = newPromiseWrapper(rt)
	// 7. Otherwise, if state is "closed",
	case WritableStreamStateClosed:
		// 7.1. Set writer.[[readyPromise]] to a promise resolved with undefined.
		writer.readyPromise = newResolvedPromiseWrapper(rt, sobek.Undefined())
		// 7.2. Set writer.[[closedPromise]] to a promise resolved with undefined.
		writer.closedPromise = newResolvedPromiseWrapper(rt, sobek.Undefined())
	// 8. Otherwise,
	default:
		// 8.1. Assert: state is "errored".
		if state != WritableStreamStateErrored {
			common.Throw(rt, newError(AssertionError, "stream is not errored"))
		}
		// 8.2. Let storedError be stream.[[storedError]].
		storedError := stream.storedError
		// 8.3. Set writer.[[readyPromise]] to a promise rejected with storedError.
		writer.readyPromise = newRejectedPromiseWrapper(rt, storedError)
		// 8.4. Set writer.[[readyPromise]].[[PromiseIsHandled]] to true.
		markPromiseHandled(rt, writer.readyPromise.promise)
		// 8.5. Set writer.[[closedPromise]] to a promise rejected with storedError.
		writer.closedPromise = newRejectedPromiseWrapper(rt, storedError)
		// 8.6. Set writer.[[closedPromise]].[[PromiseIsHandled]] to true.
		markPromiseHandled(rt, writer.closedPromise.promise)
	}
}

// settle schedules a promise settlement on the writer's stream, deferring it until the end
// of the current transaction. If the writer is no longer attached to a stream, the
// settlement runs immediately.
func (writer *WritableStreamDefaultWriter) settle(fn func()) {
	if writer.stream != nil {
		writer.stream.settle(fn)
		return
	}
	fn()
}

// abort implements the [specification]'s WritableStreamDefaultWriterAbort abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#writable-stream-default-writer-abort
func (writer *WritableStreamDefaultWriter) abort(reason sobek.Value) *sobek.Promise {
	// 1. Let stream be writer.[[stream]].
	stream := writer.stream

	// 2. Assert: stream is not undefined.
	if stream == nil {
		common.Throw(writer.runtime, newError(AssertionError, "stream is undefined"))
	}

	// 3. Return ! WritableStreamAbort(stream, reason).
	return stream.abort(reason)
}

// close implements the [specification]'s WritableStreamDefaultWriterClose abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#writable-stream-default-writer-close
func (writer *WritableStreamDefaultWriter) close() *sobek.Promise {
	// 1. Let stream be writer.[[stream]].
	stream := writer.stream

	// 2. Assert: stream is not undefined.
	if stream == nil {
		common.Throw(writer.runtime, newError(AssertionError, "stream is undefined"))
	}

	// 3. Return ! WritableStreamClose(stream).
	return stream.close()
}

func (writer *WritableStreamDefaultWriter) closeWithErrorPropagation() *sobek.Promise {
	stream := writer.stream
	if stream == nil {
		return newRejectedPromise(writer.vu, newTypeError(writer.runtime, "stream is undefined").Err())
	}
	if stream.closeQueuedOrInFlight() || stream.state == WritableStreamStateClosed {
		return writer.closedPromise.promise
	}
	if stream.state == WritableStreamStateErrored {
		return newRejectedPromise(writer.vu, throwableValue(stream.storedError))
	}
	return writer.close()
}

// getDesiredSize implements the [specification]'s WritableStreamDefaultWriterGetDesiredSize
// abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#writable-stream-default-writer-get-desired-size
func (writer *WritableStreamDefaultWriter) getDesiredSize() sobek.Value {
	// 1. Let stream be writer.[[stream]].
	stream := writer.stream

	// 2. Let state be stream.[[state]].
	state := stream.state

	// 3. If state is "errored" or "erroring", return null.
	if state == WritableStreamStateErrored || state == WritableStreamStateErroring {
		return sobek.Null()
	}

	// 4. If state is "closed", return 0.
	if state == WritableStreamStateClosed {
		return writer.runtime.ToValue(0)
	}

	// 5. Return ! WritableStreamDefaultControllerGetDesiredSize(stream.[[controller]]).
	return writer.runtime.ToValue(stream.controller.getDesiredSize())
}

// ensureClosedPromiseRejected implements the [specification]'s
// WritableStreamDefaultWriterEnsureClosedPromiseRejected abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#writable-stream-default-writer-ensure-closed-promise-rejected
func (writer *WritableStreamDefaultWriter) ensureClosedPromiseRejected(err any) {
	// 1. If writer.[[closedPromise]].[[PromiseState]] is "pending", reject writer.[[closedPromise]] with error.
	// 2. Otherwise, set writer.[[closedPromise]] to a promise rejected with error.
	//
	// The promise identity is updated synchronously (so that the `closed` getter reflects it
	// immediately), but the actual rejection is deferred so that reactions observe up-to-date
	// state. See [WritableStream.withTransaction].
	if !writer.closedPromise.isPending() {
		writer.closedPromise = newPromiseWrapper(writer.runtime)
	}
	closedPromise := writer.closedPromise
	closedPromise.queueSettlement()
	writer.settle(func() {
		closedPromise.rejectWith(err)
		// 3. Set writer.[[closedPromise]].[[PromiseIsHandled]] to true.
		markPromiseHandled(writer.runtime, closedPromise.promise)
	})
}

// ensureReadyPromiseRejected implements the [specification]'s
// WritableStreamDefaultWriterEnsureReadyPromiseRejected abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#writable-stream-default-writer-ensure-ready-promise-rejected
func (writer *WritableStreamDefaultWriter) ensureReadyPromiseRejected(err any) {
	// 1. If writer.[[readyPromise]].[[PromiseState]] is "pending", reject writer.[[readyPromise]] with error.
	// 2. Otherwise, set writer.[[readyPromise]] to a promise rejected with error.
	//
	// The promise identity is updated synchronously (so that the `ready` getter reflects it
	// immediately), but the actual rejection is deferred so that reactions observe up-to-date
	// state. See [WritableStream.withTransaction].
	if !writer.readyPromise.isPending() {
		writer.readyPromise = newPromiseWrapper(writer.runtime)
	}
	readyPromise := writer.readyPromise
	readyPromise.queueSettlement()
	writer.settle(func() {
		readyPromise.rejectWith(err)
		// 3. Set writer.[[readyPromise]].[[PromiseIsHandled]] to true.
		markPromiseHandled(writer.runtime, readyPromise.promise)
	})
}

// release implements the [specification]'s WritableStreamDefaultWriterRelease abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#writable-stream-default-writer-release
func (writer *WritableStreamDefaultWriter) release() {
	// 1. Let stream be writer.[[stream]].
	stream := writer.stream

	// 2. Assert: stream is not undefined.
	if stream == nil {
		common.Throw(writer.runtime, newError(AssertionError, "stream is undefined"))
	}

	// 3. Assert: stream.[[writer]] is writer.
	if stream.writer != writer {
		common.Throw(writer.runtime, newError(AssertionError, "stream writer is not writer"))
	}

	// 4. Let releasedError be a new TypeError.
	releasedError := newTypeError(writer.runtime, "writer released")

	// 5. Perform ! WritableStreamDefaultWriterEnsureReadyPromiseRejected(writer, releasedError).
	writer.ensureReadyPromiseRejected(releasedError)

	// 6. Perform ! WritableStreamDefaultWriterEnsureClosedPromiseRejected(writer, releasedError).
	writer.ensureClosedPromiseRejected(releasedError)

	// 7. Set stream.[[writer]] to undefined.
	stream.writer = nil

	// 8. Set writer.[[stream]] to undefined.
	writer.stream = nil
}

// write implements the [specification]'s WritableStreamDefaultWriterWrite abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#writable-stream-default-writer-write
func (writer *WritableStreamDefaultWriter) write(chunk sobek.Value) *sobek.Promise {
	// 1. Let stream be writer.[[stream]].
	stream := writer.stream

	// 2. Assert: stream is not undefined.
	if stream == nil {
		common.Throw(writer.runtime, newError(AssertionError, "stream is undefined"))
	}

	// 3. Let controller be stream.[[controller]].
	controller := stream.controller

	// 4. Let chunkSize be ! WritableStreamDefaultControllerGetChunkSize(controller, chunk).
	chunkSize := controller.getChunkSize(chunk)

	// 5. If stream is not equal to writer.[[stream]], return a promise rejected with a TypeError exception.
	if writer.stream != stream {
		return newRejectedPromise(writer.vu, newTypeError(writer.runtime, "writer was released").Err())
	}

	// 6. Let state be stream.[[state]].
	state := stream.state

	// 7. If state is "errored", return a promise rejected with stream.[[storedError]].
	if state == WritableStreamStateErrored {
		return newRejectedPromise(writer.vu, throwableValue(stream.storedError))
	}

	// 8. If ! WritableStreamCloseQueuedOrInFlight(stream) is true or state is "closed",
	// return a promise rejected with a TypeError exception indicating that the stream is closing or closed.
	if stream.closeQueuedOrInFlight() || state == WritableStreamStateClosed {
		return newRejectedPromise(writer.vu, newTypeError(writer.runtime, "stream is closing or closed").Err())
	}

	// 9. If state is "erroring", return a promise rejected with stream.[[storedError]].
	if state == WritableStreamStateErroring {
		return newRejectedPromise(writer.vu, throwableValue(stream.storedError))
	}

	// 10. Assert: state is "writable".
	if state != WritableStreamStateWritable {
		common.Throw(writer.runtime, newError(AssertionError, "stream is not writable"))
	}

	// 11. Let promise be ! WritableStreamAddWriteRequest(stream).
	promise := stream.addWriteRequest()

	// 12. Perform ! WritableStreamDefaultControllerWrite(controller, chunk, chunkSize).
	controller.write(chunk, chunkSize)

	// 13. Return promise.
	return promise
}
