package streams

import (
	"errors"

	"github.com/grafana/sobek"
	"go.k6.io/k6/v2/js/common"
)

// WritableStreamDefaultController is the default controller for a WritableStream. It has
// methods to control the stream's state and internal queue.
//
// For more details, see the [specification].
//
// [specification]: https://streams.spec.whatwg.org/#ws-default-controller-class
type WritableStreamDefaultController struct {
	// abortAlgorithm is a promise-returning algorithm, taking one argument (the abort
	// reason), which communicates a requested abort to the underlying sink.
	abortAlgorithm UnderlyingSinkAbortCallback

	// closeAlgorithm is a promise-returning algorithm which communicates a requested close
	// to the underlying sink.
	closeAlgorithm UnderlyingSinkCloseCallback

	// queue is a list representing the stream's internal queue of chunks.
	queue *QueueWithSizes

	// started is a boolean flag indicating whether the underlying sink has finished starting.
	started bool

	// strategyHWM is a number supplied to the constructor as part of the stream's queuing
	// strategy, indicating the point at which the stream will apply backpressure to its
	// underlying sink.
	strategyHWM float64

	// strategySizeAlgorithm is an algorithm to calculate the size of enqueued chunks, as part
	// of the stream's queuing strategy.
	strategySizeAlgorithm SizeAlgorithm

	// stream is the writable stream that this controller controls.
	stream *WritableStream

	// writeAlgorithm is a promise-returning algorithm, taking one argument (the chunk to
	// write), which writes data to the underlying sink.
	writeAlgorithm UnderlyingSinkWriteCallback

	// object is the JavaScript wrapper exposed to the underlying sink.
	object *sobek.Object
}

// closeSentinelType is the type of the close sentinel enqueued in the controller's queue to
// signal that the stream should be closed once all preceding writes have been processed.
type closeSentinelType struct{}

// closeSentinel is the unique, immutable value enqueued to represent a requested close.
//
// See the [close sentinel] concept in the specification.
//
// [close sentinel]: https://streams.spec.whatwg.org/#writable-stream-default-controller-close
//
//nolint:gochecknoglobals // an immutable identity sentinel, shared across controllers
var closeSentinel = &closeSentinelType{}

// NewWritableStreamDefaultControllerObject creates a new [sobek.Object] from a
// [WritableStreamDefaultController] instance.
func NewWritableStreamDefaultControllerObject(
	controller *WritableStreamDefaultController,
) (*sobek.Object, error) {
	rt := controller.stream.runtime
	obj := rt.NewObject()
	objName := "WritableStreamDefaultController"

	// The controller is not constructable: invoking its constructor must throw a TypeError.
	// Exposing a plain Go function (which is not a constructor) achieves this, as calling it
	// with `new` throws a TypeError.
	if err := setReadOnlyPropertyOf(obj, objName, "constructor", rt.ToValue(func() sobek.Value {
		throw(rt, newTypeError(rt, "WritableStreamDefaultController is not constructable"))
		return sobek.Undefined()
	})); err != nil {
		return nil, err
	}

	// NOTE: the [[signal]] slot and its `signal` getter (an AbortSignal) are intentionally
	// omitted, as k6 does not support AbortSignal yet.

	if err := setReadOnlyPropertyOf(obj, objName, "error", rt.ToValue(controller.Error)); err != nil {
		return nil, err
	}

	return rt.CreateObject(obj), nil
}

// Error signals that the producer can no longer successfully write to the stream and it
// is to be immediately moved to an errored state, with any queued-up writes discarded.
//
// It implements the WritableStreamDefaultController.error(e) [specification] algorithm.
//
// [specification]: https://streams.spec.whatwg.org/#ws-default-controller-error
func (controller *WritableStreamDefaultController) Error(e sobek.Value) {
	if e == nil {
		e = sobek.Undefined()
	}

	// 1. Let state be this.[[stream]].[[state]].
	state := controller.stream.state

	// 2. If state is not "writable", return.
	if state != WritableStreamStateWritable {
		return
	}

	// 3. Perform ! WritableStreamDefaultControllerError(this, e).
	controller.stream.withTransaction(func() {
		controller.error(e)
	})
}

// abortSteps implements the controller's [[AbortSteps]] method, as described in the
// [specification].
//
// [specification]: https://streams.spec.whatwg.org/#ws-default-controller-private-abort
func (controller *WritableStreamDefaultController) abortSteps(reason any) *sobek.Promise {
	// 1. Let result be the result of performing this.[[abortAlgorithm]], passing reason.
	result := controller.abortAlgorithm(reason)

	// 2. Perform ! WritableStreamDefaultControllerClearAlgorithms(this).
	controller.clearAlgorithms()

	// 3. Return result.
	return result
}

// errorSteps implements the controller's [[ErrorSteps]] method, as described in the
// [specification].
//
// [specification]: https://streams.spec.whatwg.org/#ws-default-controller-private-error
func (controller *WritableStreamDefaultController) errorSteps() {
	// 1. Perform ! ResetQueue(this).
	controller.resetQueue()
}

// close implements the [specification]'s WritableStreamDefaultControllerClose abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#writable-stream-default-controller-close
func (controller *WritableStreamDefaultController) close() {
	// 1. Perform ! EnqueueValueWithSize(controller, close sentinel, 0).
	if err := controller.queue.Enqueue(controller.stream.runtime.ToValue(closeSentinel), 0); err != nil {
		common.Throw(controller.stream.runtime, err)
	}

	// 2. Perform ! WritableStreamDefaultControllerAdvanceQueueIfNeeded(controller).
	controller.advanceQueueIfNeeded()
}

// getChunkSize implements the [specification]'s WritableStreamDefaultControllerGetChunkSize
// abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#writable-stream-default-controller-get-chunk-size
func (controller *WritableStreamDefaultController) getChunkSize(chunk sobek.Value) float64 {
	// If the algorithms have been cleared, fall back to a size of 1.
	if controller.strategySizeAlgorithm == nil {
		return 1
	}

	// 1. Let returnValue be the result of performing controller.[[strategySizeAlgorithm]],
	// passing in chunk, and interpreting the result as a completion record.
	size, err := controller.strategySizeAlgorithm(sobek.Undefined(), chunk)
	if err != nil {
		// 2. If returnValue is an abrupt completion,
		// 2.1. Perform ! WritableStreamDefaultControllerErrorIfNeeded(controller, returnValue.[[Value]]).
		controller.errorIfNeeded(exceptionValue(err))
		// 2.2. Return 1.
		return 1
	}

	// 3. Return returnValue.[[Value]].
	sizeFloat, err := valueToFloat(size)
	if err != nil {
		controller.errorIfNeeded(exceptionValue(err))
		return 1
	}

	return sizeFloat
}

// getDesiredSize implements the [specification]'s WritableStreamDefaultControllerGetDesiredSize
// abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#writable-stream-default-controller-get-desired-size
func (controller *WritableStreamDefaultController) getDesiredSize() float64 {
	// 1. Return controller.[[strategyHWM]] − controller.[[queueTotalSize]].
	return controller.strategyHWM - controller.queue.QueueTotalSize
}

// getBackpressure implements the [specification]'s WritableStreamDefaultControllerGetBackpressure
// abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#writable-stream-default-controller-get-backpressure
func (controller *WritableStreamDefaultController) getBackpressure() bool {
	// 1. Let desiredSize be ! WritableStreamDefaultControllerGetDesiredSize(controller).
	// 2. Return true if desiredSize ≤ 0, or false otherwise.
	return controller.getDesiredSize() <= 0
}

// write implements the [specification]'s WritableStreamDefaultControllerWrite abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#writable-stream-default-controller-write
func (controller *WritableStreamDefaultController) write(chunk sobek.Value, chunkSize float64) {
	// 1. Let enqueueResult be EnqueueValueWithSize(controller, chunk, chunkSize).
	err := controller.queue.Enqueue(chunk, chunkSize)
	// 2. If enqueueResult is an abrupt completion,
	if err != nil {
		// 2.1. Perform ! WritableStreamDefaultControllerErrorIfNeeded(controller, enqueueResult.[[Value]]).
		controller.errorIfNeeded(err)
		// 2.2. Return.
		return
	}

	// 3. Let stream be controller.[[stream]].
	stream := controller.stream

	// 4. If ! WritableStreamCloseQueuedOrInFlight(stream) is false and stream.[[state]] is "writable",
	if !stream.closeQueuedOrInFlight() && stream.state == WritableStreamStateWritable {
		// 4.1. Let backpressure be ! WritableStreamDefaultControllerGetBackpressure(controller).
		backpressure := controller.getBackpressure()
		// 4.2. Perform ! WritableStreamUpdateBackpressure(stream, backpressure).
		stream.updateBackpressure(backpressure)
	}

	// 5. Perform ! WritableStreamDefaultControllerAdvanceQueueIfNeeded(controller).
	controller.advanceQueueIfNeeded()
}

// advanceQueueIfNeeded implements the [specification]'s
// WritableStreamDefaultControllerAdvanceQueueIfNeeded abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#writable-stream-default-controller-advance-queue-if-needed
func (controller *WritableStreamDefaultController) advanceQueueIfNeeded() {
	// 1. Let stream be controller.[[stream]].
	stream := controller.stream

	// 2. If controller.[[started]] is false, return.
	if !controller.started {
		return
	}

	// 3. If stream.[[inFlightWriteRequest]] is not undefined, return.
	if stream.inFlightWriteRequest != nil {
		return
	}

	// 4. Let state be stream.[[state]].
	state := stream.state

	// 5. Assert: state is not "closed" or "errored".
	if state == WritableStreamStateClosed || state == WritableStreamStateErrored {
		common.Throw(stream.runtime, newError(AssertionError, "stream is closed or errored"))
	}

	// 6. If state is "erroring",
	if state == WritableStreamStateErroring {
		// 6.1. Perform ! WritableStreamFinishErroring(stream).
		stream.finishErroring()
		// 6.2. Return.
		return
	}

	// 7. If controller.[[queue]] is empty, return.
	if controller.queue.Len() == 0 {
		return
	}

	// 8. Let value be ! PeekQueueValue(controller).
	value, err := controller.queue.Peek()
	if err != nil {
		common.Throw(stream.runtime, newError(RuntimeError, err.Error()))
	}

	// 9. If value is the close sentinel, perform ! WritableStreamDefaultControllerProcessClose(controller).
	if controller.isCloseSentinel(value) {
		controller.processClose()
	} else {
		// 10. Otherwise, perform ! WritableStreamDefaultControllerProcessWrite(controller, value).
		controller.processWrite(value)
	}
}

// processClose implements the [specification]'s WritableStreamDefaultControllerProcessClose
// abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#writable-stream-default-controller-process-close
func (controller *WritableStreamDefaultController) processClose() {
	// 1. Let stream be controller.[[stream]].
	stream := controller.stream

	// 2. Perform ! WritableStreamMarkCloseRequestInFlight(stream).
	stream.markCloseRequestInFlight()

	// 3. Perform ! DequeueValue(controller).
	if _, err := controller.queue.Dequeue(); err != nil {
		common.Throw(stream.runtime, err)
	}

	// 4. Assert: controller.[[queue]] is empty.
	if controller.queue.Len() != 0 {
		common.Throw(stream.runtime, newError(AssertionError, "queue is not empty"))
	}

	// 5. Let sinkClosePromise be the result of performing controller.[[closeAlgorithm]].
	sinkClosePromise := controller.closeAlgorithm()

	// 6. Perform ! WritableStreamDefaultControllerClearAlgorithms(controller).
	controller.clearAlgorithms()

	_, err := promiseThen(stream.runtime, sinkClosePromise,
		// 7. Upon fulfillment of sinkClosePromise, perform ! WritableStreamFinishInFlightClose(stream).
		func(sobek.Value) {
			stream.withTransaction(func() {
				stream.finishInFlightClose()
			})
		},
		// 8. Upon rejection of sinkClosePromise with reason reason,
		// perform ! WritableStreamFinishInFlightCloseWithError(stream, reason).
		func(reason sobek.Value) {
			stream.withTransaction(func() {
				stream.finishInFlightCloseWithError(reason)
			})
		},
	)
	if err != nil {
		common.Throw(stream.runtime, err)
	}
}

// processWrite implements the [specification]'s WritableStreamDefaultControllerProcessWrite
// abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#writable-stream-default-controller-process-write
func (controller *WritableStreamDefaultController) processWrite(chunk sobek.Value) {
	// 1. Let stream be controller.[[stream]].
	stream := controller.stream

	// 2. Perform ! WritableStreamMarkFirstWriteRequestInFlight(stream).
	stream.markFirstWriteRequestInFlight()

	// 3. Let sinkWritePromise be the result of performing controller.[[writeAlgorithm]], passing in chunk.
	controllerObj, err := controller.toObject()
	if err != nil {
		common.Throw(stream.runtime, newError(RuntimeError, err.Error()))
	}
	sinkWritePromise := controller.writeAlgorithm(chunk, controllerObj)

	_, err = promiseThen(stream.runtime, sinkWritePromise,
		// 4. Upon fulfillment of sinkWritePromise,
		func(sobek.Value) {
			stream.withTransaction(func() {
				// 4.1. Perform ! WritableStreamFinishInFlightWrite(stream).
				stream.finishInFlightWrite()

				// 4.2. Let state be stream.[[state]].
				state := stream.state

				// 4.3. Assert: state is "writable" or "erroring".
				if state != WritableStreamStateWritable && state != WritableStreamStateErroring {
					common.Throw(stream.runtime, newError(AssertionError, "stream is not writable or erroring"))
				}

				// 4.4. Perform ! DequeueValue(controller).
				if _, dequeueErr := controller.queue.Dequeue(); dequeueErr != nil {
					common.Throw(stream.runtime, dequeueErr)
				}

				// 4.5. If ! WritableStreamCloseQueuedOrInFlight(stream) is false and state is "writable",
				if !stream.closeQueuedOrInFlight() && state == WritableStreamStateWritable {
					// 4.5.1. Let backpressure be ! WritableStreamDefaultControllerGetBackpressure(controller).
					backpressure := controller.getBackpressure()
					// 4.5.2. Perform ! WritableStreamUpdateBackpressure(stream, backpressure).
					stream.updateBackpressure(backpressure)
				}

				// 4.6. Perform ! WritableStreamDefaultControllerAdvanceQueueIfNeeded(controller).
				controller.advanceQueueIfNeeded()
			})
		},
		// 5. Upon rejection of sinkWritePromise with reason,
		func(reason sobek.Value) {
			stream.withTransaction(func() {
				// 5.1. If stream.[[state]] is "writable", perform ! WritableStreamDefaultControllerClearAlgorithms(controller).
				if stream.state == WritableStreamStateWritable {
					controller.clearAlgorithms()
				}

				// 5.2. Perform ! WritableStreamFinishInFlightWriteWithError(stream, reason).
				stream.finishInFlightWriteWithError(reason)
			})
		},
	)
	if err != nil {
		common.Throw(stream.runtime, err)
	}
}

// error implements the [specification]'s WritableStreamDefaultControllerError abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#writable-stream-default-controller-error
func (controller *WritableStreamDefaultController) error(e any) {
	// 1. Let stream be controller.[[stream]].
	stream := controller.stream

	// 2. Assert: stream.[[state]] is "writable".
	if stream.state != WritableStreamStateWritable {
		common.Throw(stream.runtime, newError(AssertionError, "stream is not writable"))
	}

	// 3. Perform ! WritableStreamDefaultControllerClearAlgorithms(controller).
	controller.clearAlgorithms()

	// 4. Perform ! WritableStreamStartErroring(stream, error).
	stream.startErroring(e)
}

// errorIfNeeded implements the [specification]'s WritableStreamDefaultControllerErrorIfNeeded
// abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#writable-stream-default-controller-error-if-needed
func (controller *WritableStreamDefaultController) errorIfNeeded(e any) {
	// 1. If controller.[[stream]].[[state]] is "writable", perform
	// ! WritableStreamDefaultControllerError(controller, error).
	if controller.stream.state == WritableStreamStateWritable {
		controller.error(e)
	}
}

// clearAlgorithms implements the [specification]'s WritableStreamDefaultControllerClearAlgorithms
// abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#writable-stream-default-controller-clear-algorithms
func (controller *WritableStreamDefaultController) clearAlgorithms() {
	// 1. Set controller.[[writeAlgorithm]] to undefined.
	controller.writeAlgorithm = nil

	// 2. Set controller.[[closeAlgorithm]] to undefined.
	controller.closeAlgorithm = nil

	// 3. Set controller.[[abortAlgorithm]] to undefined.
	controller.abortAlgorithm = nil

	// 4. Set controller.[[strategySizeAlgorithm]] to undefined.
	controller.strategySizeAlgorithm = nil
}

// resetQueue resets the controller's internal queue.
//
// It implements the [ResetQueue] abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#reset-queue
func (controller *WritableStreamDefaultController) resetQueue() {
	controller.queue = NewQueueWithSizes(controller.stream.runtime)
}

// isCloseSentinel returns true if the given value is the close sentinel.
func (controller *WritableStreamDefaultController) isCloseSentinel(value sobek.Value) bool {
	sentinel, ok := value.Export().(*closeSentinelType)
	return ok && sentinel == closeSentinel
}

func valueToFloat(value sobek.Value) (result float64, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			if recoveredErr, ok := recovered.(error); ok {
				err = recoveredErr
				return
			}

			panic(recovered)
		}
	}()

	return value.ToFloat(), nil
}

func (controller *WritableStreamDefaultController) toObject() (*sobek.Object, error) {
	if controller.object != nil {
		return controller.object, nil
	}

	object, err := NewWritableStreamDefaultControllerObject(controller)
	if err != nil {
		return nil, err
	}

	controller.object = object
	return object, nil
}

// exceptionValue extracts the underlying JavaScript value from a Go error, if it wraps a
// [sobek.Exception]. Otherwise, it returns the error as-is.
func exceptionValue(err error) any {
	var ex *sobek.Exception
	if errors.As(err, &ex) {
		return ex.Value()
	}
	return err
}
