package streams

import (
	"github.com/grafana/sobek"
	"go.k6.io/k6/js/common"
	"gopkg.in/guregu/null.v3"
)

// ReadableStreamDefaultController is the default controller for a ReadableStream. It has
// methods to control the stream's state and internal queue.
//
// For more details, see the [specification].
//
// [specification]: https://streams.spec.whatwg.org/#rs-default-controller-class
type ReadableStreamDefaultController struct {
	// Internal slots
	cancelAlgorithm UnderlyingSourceCancelCallback

	// closeRequested is a boolean flag indicating whether the stream has been closed by its
	// [UnderlyingSource], but still has chunks in its internal queue that have not yet been
	// read.
	closeRequested bool

	// pullAgain is a boolean flag set to tru if the stream's mechanisms requested a call
	// to the [UnderlyingSource]'s pull algorithm to pull more data, but the pull could
	// not yet be done since a previous call is still executing.
	pullAgain bool

	// A promise-returning algorithm that pulls data from the underlying source.
	pullAlgorithm UnderlyingSourcePullCallback

	// pulling is a boolean flag set to tru while the [UnderlyingSource]'s pull algorithm is
	// executing and the returned promise has not yet fulfilled, used to prevent reentrant
	// calls.
	pulling bool

	// queue is a list representing the stream's internal queue of chunks.
	queue *QueueWithSizes

	// started is a boolean flag indicating whether the [UnderlyingSource] has finished starting.
	started bool

	// strategyHWM is a number supplied to the constructor as part of the stream's queuing
	// strategy, indicating the point at which the stream will apply backpressure to its
	// [UnderlyingSource].
	strategyHWM float64

	// strategySizeAlgorithm is an algorithm to calculate the size of enqueued chunks, as part
	// of stream's queuing strategy.
	strategySizeAlgorithm SizeAlgorithm

	// stream is the readable stream that this controller controls.
	stream *ReadableStream
}

// Ensure that ReadableStreamDefaultController implements the ReadableStreamController interface.
var _ ReadableStreamController = &ReadableStreamDefaultController{}

// NewReadableStreamDefaultControllerObject creates a new [sobek.Object] from a
// [ReadableStreamDefaultController] instance.
func NewReadableStreamDefaultControllerObject(controller *ReadableStreamDefaultController) (*sobek.Object, error) {
	rt := controller.stream.runtime
	obj := rt.NewObject()
	objName := "ReadableStreamDefaultController"

	err := obj.DefineAccessorProperty("desiredSize", rt.ToValue(func() sobek.Value {
		desiredSize := controller.getDesiredSize()
		if !desiredSize.Valid {
			return sobek.Null()
		}
		return rt.ToValue(desiredSize.Float64)
	}), nil, sobek.FLAG_FALSE, sobek.FLAG_TRUE)
	if err != nil {
		return nil, err
	}

	// Exposing the properties of the [ReadableStreamController] interface
	if err := setReadOnlyPropertyOf(obj, objName, "constructor", rt.ToValue(func() sobek.Value {
		return rt.ToValue(&ReadableStreamDefaultController{})
	})); err != nil {
		return nil, err
	}

	if err := setReadOnlyPropertyOf(obj, objName, "close", rt.ToValue(controller.Close)); err != nil {
		return nil, err
	}

	if err := setReadOnlyPropertyOf(obj, objName, "enqueue", rt.ToValue(controller.Enqueue)); err != nil {
		return nil, err
	}

	if err := setReadOnlyPropertyOf(obj, objName, "error", rt.ToValue(controller.Error)); err != nil {
		return nil, err
	}

	return rt.CreateObject(obj), nil
}

// Close closes the stream.
//
// It implements the ReadableStreamDefaultController.close() [specification] algorithm.
//
// [specification]: https://streams.spec.whatwg.org/#rs-default-controller-close
func (controller *ReadableStreamDefaultController) Close() {
	rt := controller.stream.vu.Runtime()

	// 1. If ! ReadableStreamDefaultControllerCanCloseOrEnqueue(this) is false, throw a TypeError exception.
	if !controller.canCloseOrEnqueue() {
		throw(rt, newTypeError(rt, "cannot close or enqueue"))
	}

	// 2. Perform ! ReadableStreamDefaultControllerClose(this).
	controller.close()
}

// Enqueue enqueues a chunk to the stream's internal queue.
//
// It implements the ReadableStreamDefaultController.enqueue(chunk) [specification] algorithm.
//
// [specification]: https://streams.spec.whatwg.org/#rs-default-controller-enqueue
func (controller *ReadableStreamDefaultController) Enqueue(chunk sobek.Value) {
	rt := controller.stream.vu.Runtime()

	// 1. If ! ReadableStreamDefaultControllerCanCloseOrEnqueue(this) is false, throw a TypeError exception.
	if !controller.canCloseOrEnqueue() {
		throw(rt, newTypeError(rt, "cannot close or enqueue"))
	}

	// 2. Perform ? ReadableStreamDefaultControllerEnqueue(this, chunk).
	if err := controller.enqueue(chunk); err != nil {
		throw(rt, err)
	}
}

// Error signals that the stream has been errored, and performs the necessary cleanup
// steps.
//
// It implements the ReadableStreamDefaultController.error(e) [specification] algorithm.
//
// [specification]: https://streams.spec.whatwg.org/#rs-default-controller-error
func (controller *ReadableStreamDefaultController) Error(err sobek.Value) {
	if err == nil {
		err = sobek.Undefined()
	}
	controller.error(err)
}

// cancelSteps performs the controller’s steps that run in reaction to
// the stream being canceled, used to clean up the state stored in the
// controller and inform the underlying source.
//
// It implements the ReadableStreamDefaultControllerCancelSteps [specification]
// algorithm.
//
// [specification]: https://streams.spec.whatwg.org/#readable-stream-default-controller-cancel-steps
func (controller *ReadableStreamDefaultController) cancelSteps(reason any) *sobek.Promise {
	// 1. Perform ! ResetQueue(this).
	controller.resetQueue()

	// 2. Let result be the result of performing this.[[cancelAlgorithm]], passing reason.
	result := controller.cancelAlgorithm(reason)

	// 3. Perform ! ReadableStreamDefaultControllerClearAlgorithms(this).
	controller.clearAlgorithms()

	// 4. Return result.
	if p, ok := result.Export().(*sobek.Promise); ok {
		return p
	}

	return newRejectedPromise(controller.stream.vu, newError(RuntimeError, "cancel algorithm error"))
}

// pullSteps performs the controller’s steps that run when a default reader
// is read from, used to pull from the controller any queued chunks, or
// pull from the underlying source to get more chunks.
//
// It implements the [ReadableStreamDefaultControllerPullSteps] specification
// algorithm.
//
// [ReadableStreamDefaultControllerPullSteps]: https://streams.spec.whatwg.org/#rs-default-controller-private-pull
func (controller *ReadableStreamDefaultController) pullSteps(readRequest ReadRequest) {
	// 1. Let stream be this.[[stream]].
	stream := controller.stream

	// 2. If this.[[queue]] is not empty,
	if controller.queue.Len() > 0 {
		// 2.1. Let chunk be ! DequeueValue(this).
		chunk, err := controller.queue.Dequeue()
		if err != nil {
			common.Throw(stream.vu.Runtime(), err)
		}

		// 2.2. If this.[[closeRequested]] is true and this.[[queue]] is empty,
		if controller.closeRequested && controller.queue.Len() == 0 {
			// 2.2.1. Perform ! ReadableStreamDefaultControllerClearAlgorithms(this).
			controller.clearAlgorithms()
			// 2.2.2. Perform ! ReadableStreamClose(stream).
			stream.close()
		} else {
			// 2.3. Otherwise, perform ! ReadableStreamDefaultControllerCallPullIfNeeded(this).
			controller.callPullIfNeeded()
		}

		// 2. 4. Perform readRequest’s chunk steps, given chunk.
		readRequest.chunkSteps(chunk)
	} else { // 3. Otherwise,
		// 3.1. Perform ! ReadableStreamAddReadRequest(stream, readRequest).
		stream.addReadRequest(readRequest)

		// 3.2. Perform ! ReadableStreamDefaultControllerCallPullIfNeeded(this).
		controller.callPullIfNeeded()
	}
}

// releaseSteps implements the [ReleaseSteps] contract following the default controller's
// [specification].
//
// [ReleaseSteps]: https://streams.spec.whatwg.org/#abstract-opdef-readablestreamcontroller-releasesteps
// [specification]: https://streams.spec.whatwg.org/#abstract-opdef-readablestreamdefaultcontroller-releasesteps
func (controller *ReadableStreamDefaultController) releaseSteps() {
	// 1.
	return //nolint:gosimple
}

// close implements the [ReadableStreamDefaultControllerClose] algorithm
//
// [ReadableStreamDefaultControllerClose]: https://streams.spec.whatwg.org/#readable-stream-default-controller-close
func (controller *ReadableStreamDefaultController) close() {
	// 1. If ! ReadableStreamDefaultControllerCanCloseOrEnqueue(controller) is false, return.
	if !controller.canCloseOrEnqueue() {
		return
	}

	// 2. Let stream be controller.[[stream]]
	stream := controller.stream

	// 3. Set controller.[[closeRequested]] to true.
	controller.closeRequested = true

	// 4. If controller.[[queue]] is empty,
	if controller.queue.Len() == 0 {
		// 4.1. Perform ! ReadableStreamDefaultControllerClearAlgorithms(controller).
		controller.clearAlgorithms()

		// 4.2. Perform ! ReadableStreamClose(stream).
		stream.close()
	}
}

// enqueue implements the ReadableStreamDefaultControllerEnqueue(chunk) [specification]
// algorithm.
//
// [specification]: https://streams.spec.whatwg.org/#readable-stream-default-controller-enqueue
func (controller *ReadableStreamDefaultController) enqueue(chunk sobek.Value) error {
	// 1. If ! ReadableStreamDefaultControllerCanCloseOrEnqueue(controller) is false, return.
	if !controller.canCloseOrEnqueue() {
		return nil
	}

	// 2. Let stream be controller.[[stream]].
	stream := controller.stream

	// 3. If ! IsReadableStreamLocked(stream) is true and ! ReadableStreamGetNumReadRequests(stream) > 0,
	// perform ! ReadableStreamFulfillReadRequest(stream, chunk, false).
	if stream.isLocked() && stream.getNumReadRequests() > 0 {
		stream.fulfillReadRequest(chunk, false)
	} else { // 4. Otherwise,
		// 4.1. Let result be the result of performing controller.[[strategySizeAlgorithm]],
		// passing in chunk, and interpreting the result as a completion record.
		size, err := controller.strategySizeAlgorithm(sobek.Undefined(), chunk)
		// 4.2 If result is an abrupt completion,
		if err != nil {
			// 4.2.1. Perform ! ReadableStreamDefaultControllerError(controller, result.[[Value]]).
			controller.error(err)
			// 4.2.2. Return result.
			return err
		}

		// 4.3. Let chunkSize be result.[[Value]].
		chunkSize := size.ToFloat()

		// 4.4. Let enqueueResult be EnqueueValueWithSize(controller, chunk, chunkSize).
		err = controller.queue.Enqueue(chunk, chunkSize)
		// 4.5. If enqueueResult is an abrupt completion,
		if err != nil {
			// 4.5.1. Perform ! ReadableStreamDefaultControllerError(controller, enqueueResult.[[Value]]).
			controller.error(err)
			// 4.5.2. Return enqueueResult.
			return err
		}
	}

	// 5. Perform ! ReadableStreamDefaultControllerCallPullIfNeeded(controller).
	controller.callPullIfNeeded()
	return nil
}

// error implements the [ReadableStreamDefaultControllerError(e)] specification
// algorithm.
//
// [ReadableStreamDefaultControllerError(e)]: https://streams.spec.whatwg.org/#readable-stream-default-controller-error
func (controller *ReadableStreamDefaultController) error(e any) {
	// 1. Let stream be controller.[[stream]].
	stream := controller.stream

	// 2. If stream.[[state]] is not "readable", return.
	if stream.state != ReadableStreamStateReadable {
		return
	}

	// 3. Perform ! ResetQueue(controller).
	controller.resetQueue()

	// 4. Perform ! ReadableStreamDefaultControllerClearAlgorithms(controller).
	controller.clearAlgorithms()

	// 5.Perform ! ReadableStreamError(stream, e).
	stream.error(e)
}

// clearAlgorithms is called once the stream is closed or errored and the algorithms will
// not be executed anymore.
//
// It implements the ReadableStreamDefaultControllerClearAlgorithms [specification]
// algorithm.
//
// [specification]: https://streams.spec.whatwg.org/#readable-stream-default-controller-clear-algorithms
func (controller *ReadableStreamDefaultController) clearAlgorithms() {
	// 1. Set controller.[[pullAlgorithm]] to undefined.
	controller.pullAlgorithm = nil

	// 2. Set controller.[[cancelAlgorithm]] to undefined.
	controller.cancelAlgorithm = nil

	// 3. Set controller.[[strategySizeAlgorithm]] to undefined.
	controller.strategySizeAlgorithm = nil
}

// canCloseOrEnqueue returns true if the stream is in a state where it can be closed or
// enqueued to, and false otherwise.
//
// It implements the ReadableStreamDefaultControllerCanCloseOrEnqueue [specification]
// algorithm.
//
// [specification]: https://streams.spec.whatwg.org/#readable-stream-default-controller-can-close-or-enqueue
func (controller *ReadableStreamDefaultController) canCloseOrEnqueue() bool {
	// 1. Let state be controller.[[stream]].[[state]].
	state := controller.stream.state

	// 2. If controller.[[closeRequested]] is false and state is "readable", return true.
	if !controller.closeRequested && state == ReadableStreamStateReadable {
		return true
	}

	// 3. Otherwise, return false.
	return false
}

// resetQueue resets the controller's internal queue.
//
// It implements the [ReadableStreamDefaultControllerResetQueue] algorithm's specification
//
// [ReadableStreamDefaultControllerResetQueue]: https://streams.spec.whatwg.org/#reset-queue
func (controller *ReadableStreamDefaultController) resetQueue() {
	// 1. Assert: container has [[queue]] and [[queueTotalSize]] internal slots.
	// ReadableStreamDefaultController.queue && ReadableStreamDefaultController.queueTotalSize

	// 2. Set container.[[queue]] to a new empty list.
	// 3. Set container.[[queueTotalSize]] to 0.
	controller.queue = NewQueueWithSizes(controller.stream.runtime)
}

// callPullIfNeeded implements the [specification]'s ReadableStreamDefaultControllerCallPullIfNeeded algorithm
//
// [specification]: https://streams.spec.whatwg.org/#readable-stream-default-controller-call-pull-if-needed
func (controller *ReadableStreamDefaultController) callPullIfNeeded() {
	// 1. Let shouldPull be ! ReadableStreamDefaultControllerShouldCallPull(controller).
	shouldPull := controller.shouldCallPull()

	// 2. If shouldPull is false, return.
	if !shouldPull {
		return
	}

	// 3. If controller.[[pulling]] is true,
	if controller.pulling {
		// 3.1. Set controller.[[pullAgain]] to true.
		controller.pullAgain = true
		// 3.2. Return.
		return
	}

	// 4. Assert: controller.[[pullAgain]] is false.
	if controller.pullAgain {
		common.Throw(controller.stream.vu.Runtime(), newError(AssertionError, "controller.pullAgain is true"))
	}

	// 5. Set controller.[[pulling]] to true.
	controller.pulling = true

	// 6. Let pullPromise be the result of performing controller.[[pullAlgorithm]].
	controllerObj, err := controller.toObject()
	if err != nil {
		common.Throw(controller.stream.vu.Runtime(), newError(RuntimeError, err.Error()))
	}
	pullPromise := controller.pullAlgorithm(controllerObj)

	_, err = promiseThen(controller.stream.vu.Runtime(), pullPromise,
		// 7. Upon fulfillment of pullPromise
		func(sobek.Value) {
			// 7.1. Set controller.[[pulling]] to false.
			controller.pulling = false

			// 7.2. If controller.[[pullAgain]] is true,
			if controller.pullAgain {
				// 7.2.1. Set controller.[[pullAgain]] to false.
				controller.pullAgain = false
				// 7.2.2. Perform ! ReadableStreamDefaultControllerCallPullIfNeeded(controller).
				controller.callPullIfNeeded()
			}
		},

		// 8. Upon rejection of pullPromise with reason e,
		func(reason sobek.Value) {
			// 8.1. Perform ! ReadableStreamDefaultControllerError(controller, e).
			controller.error(reason)
		},
	)
	if err != nil {
		common.Throw(controller.stream.vu.Runtime(), err)
	}
}

// shouldCallPull implements the [specification]'s ReadableStreamDefaultControllerShouldCallPull algorithm
//
// [specification]: https://streams.spec.whatwg.org/#readable-stream-default-controller-should-call-pull
func (controller *ReadableStreamDefaultController) shouldCallPull() bool {
	// 1. Let stream be controller.[[stream]].
	stream := controller.stream

	// 2. If ! ReadableStreamDefaultControllerCanCloseOrEnqueue(controller) is false, return false.
	if !controller.canCloseOrEnqueue() {
		return false
	}

	// 3. If controller.[[started]] is false, return false.
	if !controller.started {
		return false
	}

	// 4. If ! IsReadableStreamLocked(stream) is true and ! ReadableStreamGetNumReadRequests(stream) > 0, return true.
	if stream.isLocked() && stream.getNumReadRequests() > 0 {
		return true
	}

	// 5. Let desiredSize be ! ReadableStreamDefaultControllerGetDesiredSize(controller).
	desiredSize := controller.getDesiredSize()

	// 6. Assert: desiredSize is not null.
	if !desiredSize.Valid {
		common.Throw(controller.stream.vu.Runtime(), newError(AssertionError, "desiredSize is null"))
	}

	// 7. If desiredSize > 0, return true.
	if desiredSize.Float64 > 0 {
		return true
	}

	// 8. Return false.
	return false
}

func (controller *ReadableStreamDefaultController) getDesiredSize() null.Float {
	state := controller.stream.state

	if state == ReadableStreamStateErrored {
		return null.NewFloat(0, false)
	}

	if state == ReadableStreamStateClosed {
		return null.NewFloat(0, true)
	}

	return null.NewFloat(controller.strategyHWM-controller.queue.QueueTotalSize, true)
}

func (controller *ReadableStreamDefaultController) toObject() (*sobek.Object, error) {
	return NewReadableStreamDefaultControllerObject(controller)
}
