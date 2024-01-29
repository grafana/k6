package streams

import (
	"github.com/dop251/goja"
	"go.k6.io/k6/js/common"
	null "gopkg.in/guregu/null.v3"
)

// ReadableStreamDefaultController is the default controller for a ReadableStream. It has
// methods to control the stream's state and internal queue.
//
// For more details, see the [specification].
//
// [specification]: https://streams.spec.whatwg.org/#rs-default-controller-class
type ReadableStreamDefaultController struct {
	// FIXME: readonly attribute desiredSize;
	// desiredSize func() int

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

	// queueTotalSize is the total size of all the chunks stored in the [queue].
	queueTotalSize int

	// started is a boolean flag indicating whether the [UnderlyingSource] has finished starting.
	started bool

	// strategyHWM is a number supplied to the constructor as part of the stream's queuing
	// strategy, indicating the point at which the stream will apply backpressure to its
	// [UnderlyingSource].
	strategyHWM int

	// strategySizeAlgorithm is an algorithm to calculate the size of enqueued chunks, as part
	// of stream's queuing strategy.
	strategySizeAlgorithm SizeAlgorithm

	// stream is the readable stream that this controller controls.
	stream *ReadableStream
}

// Ensure that ReadableStreamDefaultController implements the ReadableStreamController interface.
var _ ReadableStreamController = &ReadableStreamDefaultController{}

// Close closes the stream.
//
// It implements the ReadableStreamDefaultController.close() [specification] algorithm.
//
// [specification]: https://streams.spec.whatwg.org/#rs-default-controller-close
func (controller *ReadableStreamDefaultController) Close() {
	// 1.
	if !controller.canCloseOrEnqueue() {
		common.Throw(controller.stream.vu.Runtime(), newError(TypeError, "cannot close or enqueue"))
	}

	// 2.
	controller.close()
}

// Enqueue enqueues a chunk to the stream's internal queue.
//
// It implements the ReadableStreamDefaultController.enqueue(chunk) [specification] algorithm.
//
// [specification]: https://streams.spec.whatwg.org/#rs-default-controller-enqueue
func (controller *ReadableStreamDefaultController) Enqueue(chunk goja.Value) {
	// 1.
	if !controller.canCloseOrEnqueue() {
		common.Throw(controller.stream.vu.Runtime(), newError(TypeError, "cannot close or enqueue"))
	}

	// 2.
	if err := controller.enqueue(chunk); err != nil {
		common.Throw(controller.stream.vu.Runtime(), newError(RuntimeError, err.Error()))
	}
}

// Error signals that the stream has been errored, and performs the necessary cleanup
// steps.
//
// It implements the ReadableStreamDefaultController.error(e) [specification] algorithm.
//
// [specification]: https://streams.spec.whatwg.org/#rs-default-controller-error
func (controller *ReadableStreamDefaultController) Error(_ error) {
}

// cancelSteps performs the controller’s steps that run in reaction to
// the stream being canceled, used to clean up the state stored in the
// controller and inform the underlying source.
//
// It implements the ReadableStreamDefaultControllerCancelSteps [specification]
// algorithm.
//
// [specification]: https://streams.spec.whatwg.org/#readable-stream-default-controller-cancel-steps
func (controller *ReadableStreamDefaultController) cancelSteps(reason any) *goja.Promise {
	// 1.
	controller.resetQueue()

	// 2.
	result := controller.cancelAlgorithm(reason)

	// 3.
	controller.clearAlgorithms()

	// 4.
	return result
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
	// 1.
	stream := controller.stream

	// 2.
	if controller.queue.Len() > 0 {
		chunk, err := controller.queue.Dequeue()
		if err != nil {
			common.Throw(stream.vu.Runtime(), err)
		}

		if controller.closeRequested && controller.queue.Len() == 0 {
			controller.clearAlgorithms() // ReadableStreamDefaultControllerClearAlgorithms(controller).
			stream.close()               // ReadableStreamClose(stream).
		} else {
			controller.callPullIfNeeded()
		}

		readRequest.chunkSteps(chunk)
	} else {
		stream.addReadRequest(readRequest)
		controller.callPullIfNeeded()
	}
}

// releaseSteps performs the controller’s steps that run when a reader is
// released, used to clean up reader-specific resources stored in the controller.
//
// It implements the ReadableStreamDefaultControllerReleaseSteps [specification]
// algorithm.
//
// [specification]: https://streams.spec.whatwg.org/#readable-stream-default-controller-release-steps
func (controller *ReadableStreamDefaultController) releaseSteps() {
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

	// If controller.[[queue]] is empty
	if controller.queue.Len() == 0 {
		// 1. Perform ! ReadableStreamDefaultControllerClearAlgorithms(controller).
		controller.clearAlgorithms()

		// 2. Perform ! ReadableStreamClose(stream).
		stream.close()
	}
}

// enqueue implements the ReadableStreamDefaultControllerEnqueue(chunk) [specification]
// algorithm.
//
// [specification]: https://streams.spec.whatwg.org/#readable-stream-default-controller-enqueue
func (controller *ReadableStreamDefaultController) enqueue(chunk goja.Value) error {
	// 1.
	if !controller.canCloseOrEnqueue() {
		return nil
	}

	// 2.
	stream := controller.stream

	// 3.
	if stream.isLocked() && stream.getNumReadRequests() > 0 {
		stream.fulfillReadRequest(chunk, false)
	} else {
		// 4.1.
		result, err := controller.strategySizeAlgorithm(chunk)
		if err != nil { // If result is an abrupt completion
			controller.error(err)
			return err
		}

		err = controller.queue.Enqueue(chunk, result)
		if err != nil {
			controller.error(err)
			return err
		}

		// Perform any additional actions required after enqueuing.
		controller.callPullIfNeeded()
	}

	return nil
}

// error implements the [ReadableStreamDefaultControllerError(e)] specification
// algorithm.
//
// [ReadableStreamDefaultControllerError(e)]: https://streams.spec.whatwg.org/#readable-stream-default-controller-error
func (controller *ReadableStreamDefaultController) error(e any) {
	// 1.
	stream := controller.stream

	// 2.
	if stream.state != ReadableStreamStateReadable {
		return
	}

	// 3.
	controller.resetQueue()

	// 4.
	controller.clearAlgorithms()

	// 5.
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
	// 1.
	controller.pullAlgorithm = nil

	// 2.
	controller.cancelAlgorithm = nil

	// 3.
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
	// 1.
	state := controller.stream.state

	if !controller.closeRequested && state == ReadableStreamStateReadable {
		return true
	}

	// 3.
	return false
}

// resetQueue resets the controller's internal queue.
//
// It implements the [ReadableStreamDefaultControllerResetQueue] algorithm's specification
//
// [ReadableStreamDefaultControllerResetQueue]: https://streams.spec.whatwg.org/#reset-queue
func (controller *ReadableStreamDefaultController) resetQueue() {
	// 2.
	controller.queue = NewQueueWithSizes()

	// 3.
	controller.queueTotalSize = 0
}

// callPullIfNeeded implements the [specification]'s ReadableStreamDefaultControllerCallPullIfNeeded algorithm
//
// [specification]: https://streams.spec.whatwg.org/#readable-stream-default-controller-call-pull-if-needed
func (controller *ReadableStreamDefaultController) callPullIfNeeded() {
	// 1. let shouldPull be ! ReadableStreamDefaultControllerShouldCallPull(controller).
	shouldPull := controller.shouldCallPull()

	// 2. if shouldPull is false, return.
	if !shouldPull {
		return
	}

	// 3. If controller.[[pulling]] is true,
	if controller.pulling {
		// 3.1. Set controller.[[pullAgain]] to true.
		controller.pullAgain = true
		return
	}

	// 4. Assert: controller.[[pullAgain]] is false.
	if controller.pullAgain {
		common.Throw(controller.stream.vu.Runtime(), newError(AssertionError, "controller.pullAgain is true"))
	}

	// 5. Set controller.[[pulling]] to true.
	controller.pulling = true

	// 6. let pullPromise be the result of performing controller.[[pullAlgorithm]].
	pullPromise := controller.pullAlgorithm(controller)

	_, err := promiseThen(controller.stream.vu.Runtime(), pullPromise,
		// 7. Upon fulfillment of pullPromise
		func(value goja.Value) {
			// 1. Set controller.[[pulling]] to false.
			controller.pulling = false

			// 2. If controller.[[pullAgain]] is true,
			// 2.1. Set controller.[[pullAgain]] to false.
			controller.pullAgain = false
			// 2.2. Perform ! ReadableStreamDefaultControllerCallPullIfNeeded(controller).
			controller.callPullIfNeeded()
		},

		// 8. Upon rejection of pullPromise with reason e
		func(reason goja.Value) {
			// 1. Perform ! ReadableStreamDefaultControllerError(controller, e).
			// FIXME: handle error properly, this is not safe. Argument should probably be `any`?
			err, ok := reason.Export().(error)
			if !ok {
				common.Throw(controller.stream.vu.Runtime(), newError(AssertionError, "reason is not an error"))
			}

			controller.error(err)
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
	// 1. Let stream be controller.[[controlledReadableStream]].
	stream := controller.stream

	if !controller.canCloseOrEnqueue() {
		return false
	}

	if !controller.started {
		return false
	}

	if stream.isLocked() && stream.getNumReadRequests() > 0 {
		return true
	}

	desiredSize := controller.getDesiredSize()
	if !desiredSize.Valid {
		common.Throw(controller.stream.vu.Runtime(), newError(AssertionError, "desiredSize is null"))
	}

	if desiredSize.Int64 > 0 {
		return true
	}

	return false
}

func (controller *ReadableStreamDefaultController) getDesiredSize() null.Int {
	state := controller.stream.state

	if state == ReadableStreamStateErrored {
		return null.NewInt(0, false)
	}

	if state == ReadableStreamStateClosed {
		return null.NewInt(0, true)
	}

	return null.NewInt(int64(controller.strategyHWM-controller.queueTotalSize), true)
}
