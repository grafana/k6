package streams

import (
	"github.com/grafana/sobek"

	"go.k6.io/k6/v2/js/common"
	"go.k6.io/k6/v2/js/modules"
)

// WritableStream is a concrete instance of the general [writable stream] concept.
//
// It is adaptable to any chunk type, and maintains an internal queue to keep track of
// data supplied by the producer but not yet written to the underlying sink.
//
// [writable stream]: https://streams.spec.whatwg.org/#ws-class
type WritableStream struct {
	// backpressure holds the backpressure signal set by the controller.
	backpressure bool

	// closeRequest holds the promise returned by the writer's or the stream's close method.
	closeRequest *promiseWrapper

	// controller holds a [WritableStreamDefaultController] created with the ability to
	// control the state and queue of this stream.
	controller *WritableStreamDefaultController

	// inFlightWriteRequest holds the write request while a write is in flight.
	inFlightWriteRequest *promiseWrapper

	// inFlightCloseRequest holds the close request while a close is in flight.
	inFlightCloseRequest *promiseWrapper

	// pendingAbortRequest holds the pending abort request, or nil if there is none.
	pendingAbortRequest *pendingAbortRequest

	// state holds the current state of the stream.
	state WritableStreamState

	// storedError holds the error that caused the stream to be errored.
	storedError any

	// writer holds the current writer of the stream if the stream is locked to a writer,
	// or nil otherwise.
	writer *WritableStreamDefaultWriter

	// writeRequests holds a list of pending write request promises.
	writeRequests []*promiseWrapper

	// txDepth tracks the nesting depth of the current settlement transaction (see
	// withTransaction).
	txDepth int

	// txQueue holds promise settlements deferred until the current transaction completes.
	txQueue []func()

	runtime *sobek.Runtime
	vu      modules.VU
}

// WritableStreamState represents the current state of a WritableStream.
type WritableStreamState string

const (
	// WritableStreamStateWritable indicates that the stream is writable, and more chunks may be written.
	WritableStreamStateWritable = "writable"

	// WritableStreamStateErroring indicates that the stream is in the process of becoming errored.
	WritableStreamStateErroring = "erroring"

	// WritableStreamStateClosed indicates that the stream is closed and cannot be written to.
	WritableStreamStateClosed = "closed"

	// WritableStreamStateErrored indicates that the stream has been errored.
	WritableStreamStateErrored = "errored"
)

// pendingAbortRequest holds the state of a pending abort request, as described in the [specification].
//
// [specification]: https://streams.spec.whatwg.org/#pending-abort-request
type pendingAbortRequest struct {
	// promise is the promise returned from the stream's abort() method.
	promise *promiseWrapper

	// reason is the value that was passed as an argument to the stream's abort() method.
	reason any

	// wasAlreadyErroring is true if the stream was already erroring when abort() was called.
	wasAlreadyErroring bool
}

// promiseWrapper holds a [sobek.Promise] together with its resolve and reject functions,
// so that it can be settled later from Go code.
type promiseWrapper struct {
	promise          *sobek.Promise
	resolve          func(any) error
	reject           func(any) error
	settlementQueued bool
}

// newPromiseWrapper creates a new pending [promiseWrapper].
func newPromiseWrapper(rt *sobek.Runtime) *promiseWrapper {
	p, resolve, reject := rt.NewPromise()
	return &promiseWrapper{promise: p, resolve: resolve, reject: reject}
}

func (pw *promiseWrapper) isPending() bool {
	return !pw.settlementQueued && pw.promise.State() == sobek.PromiseStatePending
}

func (pw *promiseWrapper) queueSettlement() {
	pw.settlementQueued = true
}

// resolveWith resolves the wrapped promise with the given value.
func (pw *promiseWrapper) resolveWith(value any) {
	pw.queueSettlement()
	if err := pw.resolve(value); err != nil {
		panic(err) // TODO(@mstoykov): propagate as error instead
	}
}

// rejectWith rejects the wrapped promise with the given reason, unwrapping [jsError] values.
func (pw *promiseWrapper) rejectWith(reason any) {
	pw.queueSettlement()
	if jsErr, ok := reason.(*jsError); ok {
		reason = jsErr.Err()
	}
	if err := pw.reject(reason); err != nil {
		panic(err) // TODO(@mstoykov): propagate as error instead
	}
}

// newResolvedPromiseWrapper creates a new [promiseWrapper] resolved with the given value.
func newResolvedPromiseWrapper(rt *sobek.Runtime, value any) *promiseWrapper {
	pw := newPromiseWrapper(rt)
	pw.resolveWith(value)
	return pw
}

// newRejectedPromiseWrapper creates a new [promiseWrapper] rejected with the given reason.
func newRejectedPromiseWrapper(rt *sobek.Runtime, reason any) *promiseWrapper {
	pw := newPromiseWrapper(rt)
	pw.rejectWith(reason)
	return pw
}

// throwableValue converts an internal error value into a value suitable for rejecting a
// promise with or throwing, unwrapping [jsError] instances.
func throwableValue(err any) any {
	if jsErr, ok := err.(*jsError); ok {
		return jsErr.Err()
	}
	return err
}

// markPromiseHandled marks the given promise as handled to prevent unhandled rejection
// tracking. See https://github.com/dop251/goja/issues/565.
func markPromiseHandled(rt *sobek.Runtime, p *sobek.Promise) {
	doNothing := func(sobek.Value) {}
	if _, err := promiseThen(rt, p, doNothing, doNothing); err != nil {
		common.Throw(rt, newError(RuntimeError, err.Error()))
	}
}

// withTransaction runs fn, and then, once the outermost transaction completes, flushes any
// promise settlements that were deferred via settle() during fn.
//
// This is necessary because, in the k6/Sobek event loop, resolving or rejecting a promise
// from Go runs its reactions synchronously. The Streams specification, on the other hand,
// assumes that settling a promise merely schedules its reactions, which run only after all
// synchronous state changes have completed. Deferring the settlements until the end of the
// transaction reproduces that behaviour: reactions observe fully-updated stream state, and
// settle in a deterministic, specification-compliant (FIFO) order.
func (stream *WritableStream) withTransaction(fn func()) {
	stream.txDepth++
	defer func() {
		stream.txDepth--
		if stream.txDepth == 0 {
			stream.drainSettlements()
		}
	}()

	fn()
}

// drainSettlements runs the deferred promise settlements in FIFO order.
func (stream *WritableStream) drainSettlements() {
	// Keep the depth elevated while draining, so that any settlements scheduled by reactions
	// are appended to the queue and processed in order, rather than recursively.
	stream.txDepth++
	for len(stream.txQueue) > 0 {
		settle := stream.txQueue[0]
		stream.txQueue = stream.txQueue[1:]
		settle()
	}
	stream.txDepth--
}

// settle schedules a promise settlement. If a transaction is active, the settlement is
// deferred until the transaction completes; otherwise, it runs immediately.
func (stream *WritableStream) settle(fn func()) {
	if stream.txDepth == 0 {
		fn()
		return
	}
	stream.txQueue = append(stream.txQueue, fn)
}

func (stream *WritableStream) resolvePromise(promise *promiseWrapper, value any) {
	promise.queueSettlement()
	stream.settle(func() { promise.resolveWith(value) })
}

func (stream *WritableStream) rejectPromise(promise *promiseWrapper, reason any) {
	promise.queueSettlement()
	stream.settle(func() { promise.rejectWith(reason) })
}

// Abort aborts the stream, signaling that the producer can no longer successfully write to
// the stream and it is to be immediately moved to an errored state, with any queued-up
// writes discarded.
//
// It implements the WritableStream.abort(reason) [specification] algorithm.
//
// [specification]: https://streams.spec.whatwg.org/#ws-abort
func (stream *WritableStream) Abort(reason sobek.Value) *sobek.Promise {
	if reason == nil {
		reason = sobek.Undefined()
	}

	// 1. If ! IsWritableStreamLocked(this) is true, return a promise rejected with a TypeError exception.
	if stream.isLocked() {
		return newRejectedPromise(stream.vu, newTypeError(stream.runtime, "cannot abort a locked stream").Err())
	}

	// 2. Return ! WritableStreamAbort(this, reason).
	var promise *sobek.Promise
	stream.withTransaction(func() {
		promise = stream.abort(reason)
	})
	return promise
}

// Close closes the stream.
//
// It implements the WritableStream.close() [specification] algorithm.
//
// [specification]: https://streams.spec.whatwg.org/#ws-close
func (stream *WritableStream) Close() *sobek.Promise {
	// 1. If ! IsWritableStreamLocked(this) is true, return a promise rejected with a TypeError exception.
	if stream.isLocked() {
		return newRejectedPromise(stream.vu, newTypeError(stream.runtime, "cannot close a locked stream").Err())
	}

	// 2. If ! WritableStreamCloseQueuedOrInFlight(this) is true, return a promise rejected with a TypeError exception.
	if stream.closeQueuedOrInFlight() {
		return newRejectedPromise(stream.vu, newTypeError(stream.runtime, "stream is already closing").Err())
	}

	// 3. Return ! WritableStreamClose(this).
	var promise *sobek.Promise
	stream.withTransaction(func() {
		promise = stream.close()
	})
	return promise
}

// GetWriter implements the WritableStream.getWriter() [specification] algorithm.
//
// [specification]: https://streams.spec.whatwg.org/#ws-get-writer
func (stream *WritableStream) GetWriter() sobek.Value {
	// 1. Return ? AcquireWritableStreamDefaultWriter(this).
	writer := stream.acquireDefaultWriter()

	obj, err := NewWritableStreamDefaultWriterObject(writer, writableStreamDefaultWriterPrototype(stream.runtime))
	if err != nil {
		common.Throw(stream.runtime, err)
	}

	return obj
}

// streamGoRefKey is the name of a hidden, non-enumerable property that holds a reference to
// the Go [WritableStream] on the stream's JavaScript object. It allows retrieving the Go
// value back from the JavaScript object (e.g. in the WritableStreamDefaultWriter constructor
// and in brand-check getters), which is not otherwise possible on a plain object.
const streamGoRefKey = "__k6WritableStream__"

// writableStreamFromValue retrieves the Go [WritableStream] associated with a JavaScript
// value, or nil if the value is not a WritableStream object.
func writableStreamFromValue(rt *sobek.Runtime, value sobek.Value) *WritableStream {
	if value == nil || common.IsNullish(value) || !isObject(value) {
		return nil
	}

	ref := value.ToObject(rt).Get(streamGoRefKey)
	if ref == nil {
		return nil
	}

	stream, _ := ref.Export().(*WritableStream)
	return stream
}

// toObject builds the stream's JavaScript object.
//
// We build it as a plain object rather than a reflect-wrapped Go value, because the latter is
// not extensible: the Web Platform Tests' recordingWritableStream helper assigns extra
// properties (such as `controller` and `events`) to the stream, which a reflect-wrapped host
// object does not allow. The given proto is used as the object's prototype, and the `locked`
// brand-check getter is installed on it (once).
func (stream *WritableStream) toObject(proto *sobek.Object) *sobek.Object {
	rt := stream.runtime
	obj := rt.NewObject()
	objName := "WritableStream"

	if err := setReadOnlyPropertyOf(obj, objName, "abort", rt.ToValue(stream.Abort)); err != nil {
		common.Throw(rt, newError(RuntimeError, err.Error()))
	}
	if err := setReadOnlyPropertyOf(obj, objName, "close", rt.ToValue(stream.Close)); err != nil {
		common.Throw(rt, newError(RuntimeError, err.Error()))
	}
	if err := setReadOnlyPropertyOf(obj, objName, "getWriter", rt.ToValue(stream.GetWriter)); err != nil {
		common.Throw(rt, newError(RuntimeError, err.Error()))
	}

	// We keep a hidden, non-enumerable reference to the Go stream on the object, so that the
	// WritableStreamDefaultWriter constructor and the prototype's `locked` getter can retrieve
	// it back from the JavaScript object.
	if err := obj.DefineDataProperty(
		streamGoRefKey, rt.ToValue(stream), sobek.FLAG_FALSE, sobek.FLAG_FALSE, sobek.FLAG_FALSE,
	); err != nil {
		common.Throw(rt, newError(RuntimeError, err.Error()))
	}

	if proto.Get("locked") == nil {
		err := proto.DefineAccessorProperty("locked", rt.ToValue(func(fc sobek.FunctionCall) sobek.Value {
			s := writableStreamFromValue(rt, fc.This)
			if s == nil {
				return sobek.Undefined()
			}
			return rt.ToValue(s.isLocked())
		}), nil, sobek.FLAG_FALSE, sobek.FLAG_TRUE)
		if err != nil {
			common.Throw(rt, newError(RuntimeError, err.Error()))
		}
	}

	if err := obj.SetPrototype(proto); err != nil {
		common.Throw(rt, newError(RuntimeError, err.Error()))
	}

	return obj
}

// isLocked implements the [specification]'s IsWritableStreamLocked abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#is-writable-stream-locked
func (stream *WritableStream) isLocked() bool {
	return stream.writer != nil
}

// initialize implements the [specification]'s InitializeWritableStream abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#initialize-writable-stream
func (stream *WritableStream) initialize() {
	stream.state = WritableStreamStateWritable
	stream.storedError = nil
	stream.writer = nil
	stream.controller = nil
	stream.inFlightWriteRequest = nil
	stream.closeRequest = nil
	stream.inFlightCloseRequest = nil
	stream.pendingAbortRequest = nil
	stream.writeRequests = []*promiseWrapper{}
	stream.backpressure = false
}

// acquireDefaultWriter implements the [specification]'s AcquireWritableStreamDefaultWriter abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#acquire-writable-stream-default-writer
func (stream *WritableStream) acquireDefaultWriter() *WritableStreamDefaultWriter {
	// 1. Let writer be a new WritableStreamDefaultWriter.
	writer := &WritableStreamDefaultWriter{}

	// 2. Perform ? SetUpWritableStreamDefaultWriter(writer, stream).
	writer.setup(stream)

	// 3. Return writer.
	return writer
}

// abort implements the [specification]'s WritableStreamAbort abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#writable-stream-abort
func (stream *WritableStream) abort(reason sobek.Value) *sobek.Promise {
	// 1. If stream.[[state]] is "closed" or "errored", return a promise resolved with undefined.
	if stream.state == WritableStreamStateClosed || stream.state == WritableStreamStateErrored {
		return newResolvedPromise(stream.vu, sobek.Undefined())
	}

	// 2. Signal abort on stream.[[controller]].[[signal]] with reason.
	// NOTE: k6 does not support AbortSignal yet, so this step is intentionally omitted.

	// 3. Let state be stream.[[state]].
	state := stream.state

	// 4. If state is "closed" or "errored", return a promise resolved with undefined.
	if state == WritableStreamStateClosed || state == WritableStreamStateErrored {
		return newResolvedPromise(stream.vu, sobek.Undefined())
	}

	// 5. If stream.[[pendingAbortRequest]] is not undefined, return stream.[[pendingAbortRequest]]'s promise.
	if stream.pendingAbortRequest != nil {
		return stream.pendingAbortRequest.promise.promise
	}

	// 6. Assert: state is "writable" or "erroring".
	if state != WritableStreamStateWritable && state != WritableStreamStateErroring {
		common.Throw(stream.runtime, newError(AssertionError, "stream is not writable or erroring"))
	}

	// 7. Let wasAlreadyErroring be false.
	wasAlreadyErroring := false

	// 8. If state is "erroring",
	if state == WritableStreamStateErroring {
		// 8.1. Set wasAlreadyErroring to true.
		wasAlreadyErroring = true
		// 8.2. Set reason to undefined.
		reason = sobek.Undefined()
	}

	// 9. Let promise be a new promise.
	promise := newPromiseWrapper(stream.runtime)

	// 10. Set stream.[[pendingAbortRequest]] to a new pending abort request.
	stream.pendingAbortRequest = &pendingAbortRequest{
		promise:            promise,
		reason:             reason,
		wasAlreadyErroring: wasAlreadyErroring,
	}

	// 11. If wasAlreadyErroring is false, perform ! WritableStreamStartErroring(stream, reason).
	if !wasAlreadyErroring {
		stream.startErroring(reason)
	}

	// 12. Return promise.
	return promise.promise
}

// close implements the [specification]'s WritableStreamClose abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#writable-stream-close
func (stream *WritableStream) close() *sobek.Promise {
	// 1. Let state be stream.[[state]].
	state := stream.state

	// 2. If state is "closed" or "errored", return a promise rejected with a TypeError exception.
	if state == WritableStreamStateClosed || state == WritableStreamStateErrored {
		return newRejectedPromise(stream.vu, newTypeError(stream.runtime, "stream is already closed or errored").Err())
	}

	// 3. Assert: state is "writable" or "erroring".
	if state != WritableStreamStateWritable && state != WritableStreamStateErroring {
		common.Throw(stream.runtime, newError(AssertionError, "stream is not writable or erroring"))
	}

	// 4. Assert: ! WritableStreamCloseQueuedOrInFlight(stream) is false.
	if stream.closeQueuedOrInFlight() {
		common.Throw(stream.runtime, newError(AssertionError, "stream is already closing"))
	}

	// 5. Let promise be a new promise.
	promise := newPromiseWrapper(stream.runtime)

	// 6. Set stream.[[closeRequest]] to promise.
	stream.closeRequest = promise

	// 7. Let writer be stream.[[writer]].
	writer := stream.writer

	// 8. If writer is not undefined, and stream.[[backpressure]] is true, and state is "writable",
	// resolve writer.[[readyPromise]] with undefined.
	if writer != nil && stream.backpressure && state == WritableStreamStateWritable {
		readyPromise := writer.readyPromise
		stream.resolvePromise(readyPromise, sobek.Undefined())
	}

	// 9. Perform ! WritableStreamDefaultControllerClose(stream.[[controller]]).
	stream.controller.close()

	// 10. Return promise.
	return promise.promise
}

// addWriteRequest implements the [specification]'s WritableStreamAddWriteRequest abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#writable-stream-add-write-request
func (stream *WritableStream) addWriteRequest() *sobek.Promise {
	// 1. Assert: ! IsWritableStreamLocked(stream) is true.
	if !stream.isLocked() {
		common.Throw(stream.runtime, newError(AssertionError, "stream is not locked"))
	}

	// 2. Assert: stream.[[state]] is "writable".
	if stream.state != WritableStreamStateWritable {
		common.Throw(stream.runtime, newError(AssertionError, "stream is not writable"))
	}

	// 3. Let promise be a new promise.
	promise := newPromiseWrapper(stream.runtime)

	// 4. Append promise to stream.[[writeRequests]].
	stream.writeRequests = append(stream.writeRequests, promise)

	// 5. Return promise.
	return promise.promise
}

// closeQueuedOrInFlight implements the [specification]'s WritableStreamCloseQueuedOrInFlight
// abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#writable-stream-close-queued-or-in-flight
func (stream *WritableStream) closeQueuedOrInFlight() bool {
	// 1. If stream.[[closeRequest]] is undefined and stream.[[inFlightCloseRequest]] is undefined, return false.
	// 2. Return true.
	return stream.closeRequest != nil || stream.inFlightCloseRequest != nil
}

// dealWithRejection implements the [specification]'s WritableStreamDealWithRejection
// abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#writable-stream-deal-with-rejection
func (stream *WritableStream) dealWithRejection(err any) {
	// 1. Let state be stream.[[state]].
	state := stream.state

	// 2. If state is "writable",
	if state == WritableStreamStateWritable {
		// 2.1. Perform ! WritableStreamStartErroring(stream, error).
		stream.startErroring(err)
		// 2.2. Return.
		return
	}

	// 3. Assert: state is "erroring".
	if state != WritableStreamStateErroring {
		common.Throw(stream.runtime, newError(AssertionError, "stream is not erroring"))
	}

	// 4. Perform ! WritableStreamFinishErroring(stream).
	stream.finishErroring()
}

// startErroring implements the [specification]'s WritableStreamStartErroring abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#writable-stream-start-erroring
func (stream *WritableStream) startErroring(reason any) {
	// 1. Assert: stream.[[storedError]] is undefined.
	if stream.storedError != nil {
		common.Throw(stream.runtime, newError(AssertionError, "stream already has a stored error"))
	}

	// 2. Assert: stream.[[state]] is "writable".
	if stream.state != WritableStreamStateWritable {
		common.Throw(stream.runtime, newError(AssertionError, "stream is not writable"))
	}

	// 3. Let controller be stream.[[controller]].
	controller := stream.controller

	// 4. Assert: controller is not undefined.
	if controller == nil {
		common.Throw(stream.runtime, newError(AssertionError, "stream has no controller"))
	}

	// 5. Set stream.[[state]] to "erroring".
	stream.state = WritableStreamStateErroring

	// 6. Set stream.[[storedError]] to reason.
	stream.storedError = reason

	// 7. Let writer be stream.[[writer]].
	writer := stream.writer

	// 8. If writer is not undefined, perform ! WritableStreamDefaultWriterEnsureReadyPromiseRejected(writer, reason).
	if writer != nil {
		writer.ensureReadyPromiseRejected(reason)
	}

	// 9. If ! WritableStreamHasOperationMarkedInFlight(stream) is false and controller.[[started]] is true,
	// perform ! WritableStreamFinishErroring(stream).
	if !stream.hasOperationMarkedInFlight() && controller.started {
		stream.finishErroring()
	}
}

// finishErroring implements the [specification]'s WritableStreamFinishErroring abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#writable-stream-finish-erroring
func (stream *WritableStream) finishErroring() {
	// 1. Assert: stream.[[state]] is "erroring".
	if stream.state != WritableStreamStateErroring {
		common.Throw(stream.runtime, newError(AssertionError, "stream is not erroring"))
	}

	// 2. Assert: ! WritableStreamHasOperationMarkedInFlight(stream) is false.
	if stream.hasOperationMarkedInFlight() {
		common.Throw(stream.runtime, newError(AssertionError, "stream has an operation marked in-flight"))
	}

	// 3. Set stream.[[state]] to "errored".
	stream.state = WritableStreamStateErrored

	// 4. Perform ! stream.[[controller]].[[ErrorSteps]]().
	stream.controller.errorSteps()

	// 5. Let storedError be stream.[[storedError]].
	storedError := stream.storedError

	// 6. For each writeRequest of stream.[[writeRequests]]: reject writeRequest with storedError.
	for _, writeRequest := range stream.writeRequests {
		wr := writeRequest
		stream.rejectPromise(wr, storedError)
	}

	// 7. Set stream.[[writeRequests]] to an empty list.
	stream.writeRequests = []*promiseWrapper{}

	// 8. If stream.[[pendingAbortRequest]] is undefined,
	if stream.pendingAbortRequest == nil {
		// 8.1. Perform ! WritableStreamRejectCloseAndClosedPromiseIfNeeded(stream).
		stream.rejectCloseAndClosedPromiseIfNeeded()
		// 8.2. Return.
		return
	}

	// 9. Let abortRequest be stream.[[pendingAbortRequest]].
	abortRequest := stream.pendingAbortRequest

	// 10. Set stream.[[pendingAbortRequest]] to undefined.
	stream.pendingAbortRequest = nil

	// 11. If abortRequest's was already erroring is true,
	if abortRequest.wasAlreadyErroring {
		// 11.1. Reject abortRequest's promise with storedError.
		stream.rejectPromise(abortRequest.promise, storedError)
		// 11.2. Perform ! WritableStreamRejectCloseAndClosedPromiseIfNeeded(stream).
		stream.rejectCloseAndClosedPromiseIfNeeded()
		// 11.3. Return.
		return
	}

	// 12. Let promise be ! stream.[[controller]].[[AbortSteps]](abortRequest's reason).
	promise := stream.controller.abortSteps(abortRequest.reason)

	_, err := promiseThen(stream.runtime, promise,
		// 13. Upon fulfillment of promise,
		func(sobek.Value) {
			stream.withTransaction(func() {
				// 13.1. Resolve abortRequest's promise with undefined.
				stream.resolvePromise(abortRequest.promise, sobek.Undefined())
				// 13.2. Perform ! WritableStreamRejectCloseAndClosedPromiseIfNeeded(stream).
				stream.rejectCloseAndClosedPromiseIfNeeded()
			})
		},
		// 14. Upon rejection of promise with reason reason,
		func(reason sobek.Value) {
			stream.withTransaction(func() {
				// 14.1. Reject abortRequest's promise with reason.
				stream.rejectPromise(abortRequest.promise, reason)
				// 14.2. Perform ! WritableStreamRejectCloseAndClosedPromiseIfNeeded(stream).
				stream.rejectCloseAndClosedPromiseIfNeeded()
			})
		},
	)
	if err != nil {
		common.Throw(stream.runtime, err)
	}
}

// finishInFlightWrite implements the [specification]'s WritableStreamFinishInFlightWrite
// abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#writable-stream-finish-in-flight-write
func (stream *WritableStream) finishInFlightWrite() {
	// 1. Assert: stream.[[inFlightWriteRequest]] is not undefined.
	if stream.inFlightWriteRequest == nil {
		common.Throw(stream.runtime, newError(AssertionError, "stream has no in-flight write request"))
	}

	// 2. Resolve stream.[[inFlightWriteRequest]] with undefined.
	inFlightWriteRequest := stream.inFlightWriteRequest
	stream.resolvePromise(inFlightWriteRequest, sobek.Undefined())

	// 3. Set stream.[[inFlightWriteRequest]] to undefined.
	stream.inFlightWriteRequest = nil
}

// finishInFlightWriteWithError implements the [specification]'s
// WritableStreamFinishInFlightWriteWithError abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#writable-stream-finish-in-flight-write-with-error
func (stream *WritableStream) finishInFlightWriteWithError(err any) {
	// 1. Assert: stream.[[inFlightWriteRequest]] is not undefined.
	if stream.inFlightWriteRequest == nil {
		common.Throw(stream.runtime, newError(AssertionError, "stream has no in-flight write request"))
	}

	// 2. Reject stream.[[inFlightWriteRequest]] with error.
	inFlightWriteRequest := stream.inFlightWriteRequest
	stream.rejectPromise(inFlightWriteRequest, err)

	// 3. Set stream.[[inFlightWriteRequest]] to undefined.
	stream.inFlightWriteRequest = nil

	// 4. Assert: stream.[[state]] is "writable" or "erroring".
	if stream.state != WritableStreamStateWritable && stream.state != WritableStreamStateErroring {
		common.Throw(stream.runtime, newError(AssertionError, "stream is not writable or erroring"))
	}

	// 5. Perform ! WritableStreamDealWithRejection(stream, error).
	stream.dealWithRejection(err)
}

// finishInFlightClose implements the [specification]'s WritableStreamFinishInFlightClose
// abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#writable-stream-finish-in-flight-close
func (stream *WritableStream) finishInFlightClose() {
	// 1. Assert: stream.[[inFlightCloseRequest]] is not undefined.
	if stream.inFlightCloseRequest == nil {
		common.Throw(stream.runtime, newError(AssertionError, "stream has no in-flight close request"))
	}

	// 2. Resolve stream.[[inFlightCloseRequest]] with undefined.
	inFlightCloseRequest := stream.inFlightCloseRequest
	stream.resolvePromise(inFlightCloseRequest, sobek.Undefined())

	// 3. Set stream.[[inFlightCloseRequest]] to undefined.
	stream.inFlightCloseRequest = nil

	// 4. Let state be stream.[[state]].
	state := stream.state

	// 5. Assert: stream.[[state]] is "writable" or "erroring".
	if state != WritableStreamStateWritable && state != WritableStreamStateErroring {
		common.Throw(stream.runtime, newError(AssertionError, "stream is not writable or erroring"))
	}

	// 6. If state is "erroring",
	if state == WritableStreamStateErroring {
		// 6.1. Set stream.[[storedError]] to undefined.
		stream.storedError = nil

		// 6.2. If stream.[[pendingAbortRequest]] is not undefined,
		if stream.pendingAbortRequest != nil {
			// 6.2.1. Resolve stream.[[pendingAbortRequest]]'s promise with undefined.
			abortRequest := stream.pendingAbortRequest
			stream.resolvePromise(abortRequest.promise, sobek.Undefined())
			// 6.2.2. Set stream.[[pendingAbortRequest]] to undefined.
			stream.pendingAbortRequest = nil
		}
	}

	// 7. Set stream.[[state]] to "closed".
	stream.state = WritableStreamStateClosed

	// 8. Let writer be stream.[[writer]].
	writer := stream.writer

	// 9. If writer is not undefined, resolve writer.[[closedPromise]] with undefined.
	if writer != nil {
		closedPromise := writer.closedPromise
		stream.resolvePromise(closedPromise, sobek.Undefined())
	}

	// 10. Assert: stream.[[pendingAbortRequest]] is undefined.
	if stream.pendingAbortRequest != nil {
		common.Throw(stream.runtime, newError(AssertionError, "stream has a pending abort request"))
	}

	// 11. Assert: stream.[[storedError]] is undefined.
	if stream.storedError != nil {
		common.Throw(stream.runtime, newError(AssertionError, "stream has a stored error"))
	}
}

// finishInFlightCloseWithError implements the [specification]'s
// WritableStreamFinishInFlightCloseWithError abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#writable-stream-finish-in-flight-close-with-error
func (stream *WritableStream) finishInFlightCloseWithError(err any) {
	// 1. Assert: stream.[[inFlightCloseRequest]] is not undefined.
	if stream.inFlightCloseRequest == nil {
		common.Throw(stream.runtime, newError(AssertionError, "stream has no in-flight close request"))
	}

	// 2. Reject stream.[[inFlightCloseRequest]] with error.
	inFlightCloseRequest := stream.inFlightCloseRequest
	stream.rejectPromise(inFlightCloseRequest, err)

	// 3. Set stream.[[inFlightCloseRequest]] to undefined.
	stream.inFlightCloseRequest = nil

	// 4. Assert: stream.[[state]] is "writable" or "erroring".
	if stream.state != WritableStreamStateWritable && stream.state != WritableStreamStateErroring {
		common.Throw(stream.runtime, newError(AssertionError, "stream is not writable or erroring"))
	}

	// 5. If stream.[[pendingAbortRequest]] is not undefined,
	if stream.pendingAbortRequest != nil {
		// 5.1. Reject stream.[[pendingAbortRequest]]'s promise with error.
		abortRequest := stream.pendingAbortRequest
		stream.rejectPromise(abortRequest.promise, err)
		// 5.2. Set stream.[[pendingAbortRequest]] to undefined.
		stream.pendingAbortRequest = nil
	}

	// 6. Perform ! WritableStreamDealWithRejection(stream, error).
	stream.dealWithRejection(err)
}

// hasOperationMarkedInFlight implements the [specification]'s
// WritableStreamHasOperationMarkedInFlight abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#writable-stream-has-operation-marked-in-flight
func (stream *WritableStream) hasOperationMarkedInFlight() bool {
	// 1. If stream.[[inFlightWriteRequest]] is undefined and stream.[[inFlightCloseRequest]] is undefined, return false.
	// 2. Return true.
	return stream.inFlightWriteRequest != nil || stream.inFlightCloseRequest != nil
}

// markCloseRequestInFlight implements the [specification]'s WritableStreamMarkCloseRequestInFlight
// abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#writable-stream-mark-close-request-in-flight
func (stream *WritableStream) markCloseRequestInFlight() {
	// 1. Assert: stream.[[inFlightCloseRequest]] is undefined.
	if stream.inFlightCloseRequest != nil {
		common.Throw(stream.runtime, newError(AssertionError, "stream has an in-flight close request"))
	}

	// 2. Assert: stream.[[closeRequest]] is not undefined.
	if stream.closeRequest == nil {
		common.Throw(stream.runtime, newError(AssertionError, "stream has no close request"))
	}

	// 3. Set stream.[[inFlightCloseRequest]] to stream.[[closeRequest]].
	stream.inFlightCloseRequest = stream.closeRequest

	// 4. Set stream.[[closeRequest]] to undefined.
	stream.closeRequest = nil
}

// markFirstWriteRequestInFlight implements the [specification]'s
// WritableStreamMarkFirstWriteRequestInFlight abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#writable-stream-mark-first-write-request-in-flight
func (stream *WritableStream) markFirstWriteRequestInFlight() {
	// 1. Assert: stream.[[inFlightWriteRequest]] is undefined.
	if stream.inFlightWriteRequest != nil {
		common.Throw(stream.runtime, newError(AssertionError, "stream has an in-flight write request"))
	}

	// 2. Assert: stream.[[writeRequests]] is not empty.
	if len(stream.writeRequests) == 0 {
		common.Throw(stream.runtime, newError(AssertionError, "stream has no write requests"))
	}

	// 3. Let writeRequest be stream.[[writeRequests]][0].
	writeRequest := stream.writeRequests[0]

	// 4. Remove writeRequest from stream.[[writeRequests]].
	stream.writeRequests = stream.writeRequests[1:]

	// 5. Set stream.[[inFlightWriteRequest]] to writeRequest.
	stream.inFlightWriteRequest = writeRequest
}

// rejectCloseAndClosedPromiseIfNeeded implements the [specification]'s
// WritableStreamRejectCloseAndClosedPromiseIfNeeded abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#writable-stream-reject-close-and-closed-promise-if-needed
func (stream *WritableStream) rejectCloseAndClosedPromiseIfNeeded() {
	// 1. Assert: stream.[[state]] is "errored".
	if stream.state != WritableStreamStateErrored {
		common.Throw(stream.runtime, newError(AssertionError, "stream is not errored"))
	}

	storedError := stream.storedError

	// 2. If stream.[[closeRequest]] is not undefined,
	if stream.closeRequest != nil {
		// 2.1. Assert: stream.[[inFlightCloseRequest]] is undefined.
		if stream.inFlightCloseRequest != nil {
			common.Throw(stream.runtime, newError(AssertionError, "stream has an in-flight close request"))
		}

		// 2.2. Reject stream.[[closeRequest]] with stream.[[storedError]].
		closeRequest := stream.closeRequest
		stream.rejectPromise(closeRequest, storedError)

		// 2.3. Set stream.[[closeRequest]] to undefined.
		stream.closeRequest = nil
	}

	// 3. Let writer be stream.[[writer]].
	writer := stream.writer

	// 4. If writer is not undefined,
	if writer != nil {
		// 4.1. Reject writer.[[closedPromise]] with stream.[[storedError]].
		closedPromise := writer.closedPromise
		closedPromise.queueSettlement()
		stream.settle(func() {
			closedPromise.rejectWith(storedError)
			// 4.2. Set writer.[[closedPromise]].[[PromiseIsHandled]] to true.
			markPromiseHandled(stream.runtime, closedPromise.promise)
		})
	}
}

// updateBackpressure implements the [specification]'s WritableStreamUpdateBackpressure
// abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#writable-stream-update-backpressure
func (stream *WritableStream) updateBackpressure(backpressure bool) {
	// 1. Assert: stream.[[state]] is "writable".
	if stream.state != WritableStreamStateWritable {
		common.Throw(stream.runtime, newError(AssertionError, "stream is not writable"))
	}

	// 2. Assert: ! WritableStreamCloseQueuedOrInFlight(stream) is false.
	if stream.closeQueuedOrInFlight() {
		common.Throw(stream.runtime, newError(AssertionError, "stream is already closing"))
	}

	// 3. Let writer be stream.[[writer]].
	writer := stream.writer

	// 4. If writer is not undefined and backpressure is not stream.[[backpressure]],
	if writer != nil && backpressure != stream.backpressure {
		if backpressure {
			// 4.1. If backpressure is true, set writer.[[readyPromise]] to a new promise.
			writer.readyPromise = newPromiseWrapper(stream.runtime)
		} else {
			// 4.2. Otherwise,
			// 4.2.1. Assert: backpressure is false.
			// 4.2.2. Resolve writer.[[readyPromise]] with undefined.
			readyPromise := writer.readyPromise
			stream.resolvePromise(readyPromise, sobek.Undefined())
		}
	}

	// 5. Set stream.[[backpressure]] to backpressure.
	stream.backpressure = backpressure
}

// setupWritableStreamDefaultControllerFromUnderlyingSink implements the [specification]'s
// SetUpWritableStreamDefaultControllerFromUnderlyingSink abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#set-up-writable-stream-default-controller-from-underlying-sink
func (stream *WritableStream) setupWritableStreamDefaultControllerFromUnderlyingSink(
	underlyingSink *sobek.Object,
	underlyingSinkDict UnderlyingSink,
	highWaterMark float64,
	sizeAlgorithm SizeAlgorithm,
) {
	// 1. Let controller be a new WritableStreamDefaultController.
	controller := &WritableStreamDefaultController{}

	// 2. Let startAlgorithm be an algorithm that returns undefined.
	var startAlgorithm UnderlyingSinkStartCallback = func(*sobek.Object) sobek.Value {
		return sobek.Undefined()
	}

	// 3. Let writeAlgorithm be an algorithm that returns a promise resolved with undefined.
	var writeAlgorithm UnderlyingSinkWriteCallback = func(sobek.Value, *sobek.Object) *sobek.Promise {
		return newResolvedPromise(stream.vu, sobek.Undefined())
	}

	// 4. Let closeAlgorithm be an algorithm that returns a promise resolved with undefined.
	var closeAlgorithm UnderlyingSinkCloseCallback = func() *sobek.Promise {
		return newResolvedPromise(stream.vu, sobek.Undefined())
	}

	// 5. Let abortAlgorithm be an algorithm that returns a promise resolved with undefined.
	var abortAlgorithm UnderlyingSinkAbortCallback = func(any) *sobek.Promise {
		return newResolvedPromise(stream.vu, sobek.Undefined())
	}

	// 6. If underlyingSinkDict["start"] exists, then set startAlgorithm to an algorithm which
	// returns the result of invoking underlyingSinkDict["start"] with argument list « controller »
	// and callback this value underlyingSink.
	if isDictionaryMemberPresent(underlyingSinkDict.Start) {
		startAlgorithm = stream.startAlgorithm(underlyingSink, underlyingSinkDict)
	}

	// 7. If underlyingSinkDict["write"] exists, then set writeAlgorithm to an algorithm which
	// returns the result of invoking underlyingSinkDict["write"] with argument list « chunk, controller »
	// and callback this value underlyingSink.
	if isDictionaryMemberPresent(underlyingSinkDict.Write) {
		writeAlgorithm = stream.writeAlgorithm(underlyingSink, underlyingSinkDict)
	}

	// 8. If underlyingSinkDict["close"] exists, then set closeAlgorithm to an algorithm which
	// returns the result of invoking underlyingSinkDict["close"] with argument list « »
	// and callback this value underlyingSink.
	if isDictionaryMemberPresent(underlyingSinkDict.Close) {
		closeAlgorithm = stream.closeAlgorithm(underlyingSink, underlyingSinkDict)
	}

	// 9. If underlyingSinkDict["abort"] exists, then set abortAlgorithm to an algorithm which
	// returns the result of invoking underlyingSinkDict["abort"] with argument list « reason »
	// and callback this value underlyingSink.
	if isDictionaryMemberPresent(underlyingSinkDict.Abort) {
		abortAlgorithm = stream.abortAlgorithm(underlyingSink, underlyingSinkDict)
	}

	// 10. Perform ? SetUpWritableStreamDefaultController(...).
	stream.setupWritableStreamDefaultController(
		controller,
		startAlgorithm,
		writeAlgorithm,
		closeAlgorithm,
		abortAlgorithm,
		highWaterMark,
		sizeAlgorithm,
	)
}

func (stream *WritableStream) startAlgorithm(
	underlyingSink *sobek.Object,
	underlyingSinkDict UnderlyingSink,
) UnderlyingSinkStartCallback {
	call, ok := sobek.AssertFunction(underlyingSinkDict.Start)
	if !ok {
		throw(stream.runtime, newTypeError(stream.runtime, "underlyingSink.[[start]] must be a function"))
	}

	return func(controller *sobek.Object) sobek.Value {
		v, err := call(underlyingSink, controller)
		if err != nil {
			panic(err)
		}
		return v
	}
}

func (stream *WritableStream) writeAlgorithm(
	underlyingSink *sobek.Object,
	underlyingSinkDict UnderlyingSink,
) UnderlyingSinkWriteCallback {
	call, ok := sobek.AssertFunction(underlyingSinkDict.Write)
	if !ok {
		throw(stream.runtime, newTypeError(stream.runtime, "underlyingSink.[[write]] must be a function"))
	}

	return func(chunk sobek.Value, controller *sobek.Object) *sobek.Promise {
		v, err := call(underlyingSink, chunk, controller)
		if err != nil {
			return stream.rejectedPromiseFromErr(err)
		}
		if p, ok := v.Export().(*sobek.Promise); ok {
			return p
		}
		return newResolvedPromise(stream.vu, v)
	}
}

func (stream *WritableStream) closeAlgorithm(
	underlyingSink *sobek.Object,
	underlyingSinkDict UnderlyingSink,
) UnderlyingSinkCloseCallback {
	call, ok := sobek.AssertFunction(underlyingSinkDict.Close)
	if !ok {
		throw(stream.runtime, newTypeError(stream.runtime, "underlyingSink.[[close]] must be a function"))
	}

	return func() *sobek.Promise {
		v, err := call(underlyingSink)
		if err != nil {
			return stream.rejectedPromiseFromErr(err)
		}
		if p, ok := v.Export().(*sobek.Promise); ok {
			return p
		}
		return newResolvedPromise(stream.vu, v)
	}
}

func (stream *WritableStream) abortAlgorithm(
	underlyingSink *sobek.Object,
	underlyingSinkDict UnderlyingSink,
) UnderlyingSinkAbortCallback {
	call, ok := sobek.AssertFunction(underlyingSinkDict.Abort)
	if !ok {
		throw(stream.runtime, newTypeError(stream.runtime, "underlyingSink.[[abort]] must be a function"))
	}

	return func(reason any) *sobek.Promise {
		v, err := call(underlyingSink, stream.runtime.ToValue(reason))
		if err != nil {
			return stream.rejectedPromiseFromErr(err)
		}
		if p, ok := v.Export().(*sobek.Promise); ok {
			return p
		}
		return newResolvedPromise(stream.vu, v)
	}
}

// rejectedPromiseFromErr returns a rejected promise from a Go error, unwrapping any
// [sobek.Exception] so that the original thrown value is preserved.
func (stream *WritableStream) rejectedPromiseFromErr(err error) *sobek.Promise {
	return newRejectedPromise(stream.vu, exceptionValue(err))
}

// setupWritableStreamDefaultController implements the [specification]'s
// SetUpWritableStreamDefaultController abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#set-up-writable-stream-default-controller
func (stream *WritableStream) setupWritableStreamDefaultController(
	controller *WritableStreamDefaultController,
	startAlgorithm UnderlyingSinkStartCallback,
	writeAlgorithm UnderlyingSinkWriteCallback,
	closeAlgorithm UnderlyingSinkCloseCallback,
	abortAlgorithm UnderlyingSinkAbortCallback,
	highWaterMark float64,
	sizeAlgorithm SizeAlgorithm,
) {
	rt := stream.runtime

	// 1. Assert: stream implements WritableStream.
	// 2. Assert: stream.[[controller]] is undefined.
	if stream.controller != nil {
		common.Throw(rt, newError(AssertionError, "stream already has a controller"))
	}

	// 3. Set controller.[[stream]] to stream.
	controller.stream = stream

	// 4. Set stream.[[controller]] to controller.
	stream.controller = controller

	// 5. Perform ! ResetQueue(controller).
	controller.resetQueue()

	// 6. Set controller.[[signal]] to a new AbortSignal.
	// NOTE: k6 does not support AbortSignal yet, so this step is intentionally omitted.

	// 7. Set controller.[[started]] to false.
	controller.started = false

	// 8. Set controller.[[strategySizeAlgorithm]] to sizeAlgorithm.
	controller.strategySizeAlgorithm = sizeAlgorithm

	// 9. Set controller.[[strategyHWM]] to highWaterMark.
	controller.strategyHWM = highWaterMark

	// 10. Set controller.[[writeAlgorithm]] to writeAlgorithm.
	controller.writeAlgorithm = writeAlgorithm

	// 11. Set controller.[[closeAlgorithm]] to closeAlgorithm.
	controller.closeAlgorithm = closeAlgorithm

	// 12. Set controller.[[abortAlgorithm]] to abortAlgorithm.
	controller.abortAlgorithm = abortAlgorithm

	// 13. Let backpressure be ! WritableStreamDefaultControllerGetBackpressure(controller).
	backpressure := controller.getBackpressure()

	// 14. Perform ! WritableStreamUpdateBackpressure(stream, backpressure).
	stream.updateBackpressure(backpressure)

	// 15. Let startResult be the result of performing startAlgorithm. (This may throw an exception.)
	controllerObj, err := controller.toObject()
	if err != nil {
		common.Throw(rt, newError(RuntimeError, err.Error()))
	}
	startResult := startAlgorithm(controllerObj)

	// 16. Let startPromise be a promise resolved with startResult.
	var startPromise *sobek.Promise
	if common.IsNullish(startResult) {
		startPromise = newResolvedPromise(stream.vu, sobek.Undefined())
	} else if p, ok := startResult.Export().(*sobek.Promise); ok {
		startPromise = p
	} else {
		startPromise = newResolvedPromise(stream.vu, startResult)
	}

	_, err = promiseThen(rt, startPromise,
		// 17. Upon fulfillment of startPromise,
		func(sobek.Value) {
			stream.withTransaction(func() {
				// 17.1. Assert: stream.[[state]] is "writable" or "erroring".
				if stream.state != WritableStreamStateWritable && stream.state != WritableStreamStateErroring {
					common.Throw(rt, newError(AssertionError, "stream is not writable or erroring"))
				}
				// 17.2. Set controller.[[started]] to true.
				controller.started = true
				// 17.3. Perform ! WritableStreamDefaultControllerAdvanceQueueIfNeeded(controller).
				controller.advanceQueueIfNeeded()
			})
		},
		// 18. Upon rejection of startPromise with reason r,
		func(r sobek.Value) {
			stream.withTransaction(func() {
				// 18.1. Assert: stream.[[state]] is "writable" or "erroring".
				if stream.state != WritableStreamStateWritable && stream.state != WritableStreamStateErroring {
					common.Throw(rt, newError(AssertionError, "stream is not writable or erroring"))
				}
				// 18.2. Set controller.[[started]] to true.
				controller.started = true
				// 18.3. Perform ! WritableStreamDealWithRejection(stream, r).
				stream.dealWithRejection(r)
			})
		},
	)
	if err != nil {
		common.Throw(rt, err)
	}
}
