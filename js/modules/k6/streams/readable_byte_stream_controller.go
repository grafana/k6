package streams

import (
	"math"

	"github.com/dop251/goja"
	"go.k6.io/k6/js/common"
	null "gopkg.in/guregu/null.v3"
)

// ReadableByteStreamController allows control of a [ReadableStream]’s state and internal queue of chunks.
type ReadableByteStreamController struct {
	ByobRequest *ReadableStreamBYOBRequest

	// FIXME: using a pointer to mark it as optional, find a better solution?
	DesiredSize *int

	// autoAllocateChunkSize holds a positive integer, when the automatic buffer allocation
	// feature is enabled. In that case, this value specifies the size of buffer to allocate.
	// It is set to 0 otherwise.
	autoAllocateChunkSize int64

	// cancelAlgorithm holds A promise-returning algorithm, taking one argument (the cancel
	// reason), which communicates a requested cancellation to the underlying byte source.
	cancelAlgorithm func(reason any) *goja.Promise

	// closeRequested holds a boolean flag indicating whether the stream has been
	// closed by its underlying byte source, but still has chunks in its internal
	// queue that have not yet been read.
	closeRequested bool

	// pullAgain a boolean flag set to true if the stream’s mechanisms requested a
	// call to the underlying byte source's pull algorithm to pull more data, but
	// the pull could not yet be done since a previous call is still executing.
	pullAgain bool

	// pullAlgorithm is a promise-returning algorithm that pulls data from the
	// underlying byte source.
	pullAlgorithm func() *goja.Promise

	// FIXME: should this be atomic?
	// pulling is a boolean flag set to true while the underlying byte source's pull
	// algorithm is executing and the returned promise has not yet fulfilled, used
	// to prevent reentrant calls.
	pulling bool

	// pendingPullIntos holds a list of pull-into descriptors
	pendingPullIntos []PullIntoDescriptor

	// queue holds a list of readable byte stream queue entries representing the stream's
	// internal queue of chunks.
	// FIXME: use QueueWithSizes here?
	queue []ReadableByteStreamQueueEntry

	// queueTotalSize holds the total size of all the chunks stored in the [queue].
	queueTotalSize int64

	// started holds a boolean flag indicating whether the underlying byte source has
	// finished starting.
	started bool

	// strategyHWM holds a number supplied to the constructor as part of the stream's
	// queuing strategy, indicating the point at which the stream will apply backpressure
	// to its underlying byte source.
	strategyHWM int64

	// stream points to the ReadableStream that this controller controls.
	stream *ReadableStream
}

// Ensure that ReadableByteStreamController implements the ReadableStreamController interface.
var _ ReadableStreamController = &ReadableByteStreamController{}

// Close implements the [specification]'s ReadableByteStreamController close() algorithm.
//
// [specification]: https://streams.spec.whatwg.org/#rbs-controller-close
func (controller *ReadableByteStreamController) Close() {
	// 1.
	stream := controller.stream

	// 2.
	if controller.closeRequested || stream.state != ReadableStreamStateReadable {
		return
	}

	// 3.
	if controller.queueTotalSize > 0 {
		controller.closeRequested = true
		return
	}

	// 4.
	if len(controller.pendingPullIntos) > 0 {
		// 1.
		firstPendingPullInto := controller.pendingPullIntos[0]

		// 2.
		if firstPendingPullInto.bytesFilled%firstPendingPullInto.elementSize != 0 {
			e := newError(TypeError, "bytesFilled is not a multiple of elementSize")
			controller.error(e)
			common.Throw(controller.stream.vu.Runtime(), e)
		}
	}

	// 5.
	controller.clearAlgorithms()

	// 6.
	stream.close()
}

// Enqueue implements the [specification]'s ReadableByteStreamController enqueue(chunk) algorithm.
//
// [specification]: https://streams.spec.whatwg.org/#rbs-controller-enqueue
func (controller *ReadableByteStreamController) Enqueue(chunk goja.Value) {
	// 1.
	stream := controller.stream

	// 2.
	if controller.closeRequested || stream.state != ReadableStreamStateReadable {
		return
	}

	// 3.
	concreteChunk, err := asViewedArrayBuffer(controller.stream.vu.Runtime(), chunk)
	if err != nil {
		common.Throw(
			controller.stream.vu.Runtime(),
			newError(RuntimeError, "chunk is neither an ArrayBuffer, TypedArray nor DataView"),
		)
	}
	buffer := concreteChunk.ArrayBuffer

	// 4. 5.
	byteOffset := concreteChunk.ByteOffset
	byteLength := concreteChunk.ByteLength

	// 6.
	if buffer.Detached() {
		common.Throw(controller.stream.vu.Runtime(), newError(TypeError, "chunk is a detached ArrayBuffer"))
	}

	// 7.
	transferredBuffer := transferArrayBuffer(controller.stream.runtime, buffer)

	// 8.
	if len(controller.pendingPullIntos) > 0 {
		// 1.
		firstPendingPullInto := controller.pendingPullIntos[0]

		// 2.
		if firstPendingPullInto.buffer.Detached() {
			common.Throw(
				controller.stream.vu.Runtime(),
				newError(TypeError, "firstPendingPullInto.buffer is a detached ArrayBuffer"),
			)
		}

		// 3.
		controller.invalidateBYOBRequest()

		// 4.
		firstPendingPullInto.buffer = transferArrayBuffer(controller.stream.runtime, firstPendingPullInto.buffer)

		// 5.
		if firstPendingPullInto.readerType == ReaderTypeNone {
			controller.enqueueDetachedPullIntoToQueue(firstPendingPullInto)
		}
	}

	// 9.
	if stream.hasDefaultReader() {
		// 1.
		controller.processReadRequestsUsingQueue()

		// 2.
		if stream.getNumReadRequests() == 0 {
			// 2.1.
			if len(controller.pendingPullIntos) > 0 {
				common.Throw(controller.stream.vu.Runtime(), newError(AssertionError, "pendingPullIntos is not empty"))
			}

			// 2.2.
			controller.enqueueChunkToQueue(transferredBuffer, byteOffset, byteLength)
		}

		// 3.
		// 3.1.
		if len(controller.queue) != 0 {
			common.Throw(controller.stream.vu.Runtime(), newError(AssertionError, "queue is not empty"))
		}

		// 3.2.
		if len(controller.pendingPullIntos) > 0 {
			// 3.2.1.
			if controller.pendingPullIntos[0].readerType != ReaderTypeDefault {
				common.Throw(
					controller.stream.vu.Runtime(),
					newError(AssertionError, "pendingPullIntos[0].readerType is not default"),
				)
			}

			// 3.2.2.
			controller.shiftPendingPullInto()
		}

		// 3.3.
		transferredView, err := newUint8Array(controller.stream.runtime, transferredBuffer.Bytes(), byteOffset, byteLength)
		if err != nil {
			common.Throw(controller.stream.vu.Runtime(), newError(RuntimeError, "failed to create Uint8Array"))
		}

		// 3.4.
		stream.fulfillReadRequest(transferredView, false)
	}

	// 10.
	if stream.hasBYOBReader() {
		controller.enqueueChunkToQueue(transferredBuffer, byteOffset, byteLength)
		controller.processPullIntoDescriptorsUsingQueue()
	}

	// 11.
	// 11.1.
	if stream.isLocked() {
		common.Throw(controller.stream.vu.Runtime(), newError(AssertionError, "stream is locked"))
	}

	// 11.2.
	controller.enqueueChunkToQueue(transferredBuffer, byteOffset, byteLength)

	// 12.
	controller.callPullIfNeeded()
}

// Error implements the [specification]'s ReadableByteStreamController error(e) algorithm.
//
// [specification]: https://streams.spec.whatwg.org/#rbs-controller-error
func (controller *ReadableByteStreamController) Error(err error) {
	// 1.
	stream := controller.stream

	// 2.
	if stream.state != ReadableStreamStateReadable {
		return
	}

	// 3.
	controller.clearPendingPullIntos()

	// 4.
	controller.resetQueue()

	// 5.
	controller.clearAlgorithms()

	// 6.
	stream.error(err)
}

// cancelSteps performs the controller’s steps that run in reaction to
// the stream being canceled, used to clean up the state stored in the
// controller and inform the underlying source.
func (controller *ReadableByteStreamController) cancelSteps(_ any) *goja.Promise {
	panic("not implemented") // TODO: Implement
}

// pullSteps performs the controller’s steps that run when a default reader
// is read from, used to pull from the controller any queued chunks, or
// pull from the underlying source to get more chunks.
func (controller *ReadableByteStreamController) pullSteps(_ ReadRequest) {
	panic("not implemented") // TODO: Implement
}

// releaseSteps performs the controller’s steps that run when a reader is
// released, used to clean up reader-specific resources stored in the controller.
func (controller *ReadableByteStreamController) releaseSteps() {
	panic("not implemented") // TODO: Implement
}

func (controller *ReadableByteStreamController) error(err error) {
	// 1.
	stream := controller.stream

	// 2.
	if stream.state != ReadableStreamStateReadable {
		return
	}

	// 3.
	controller.clearPendingPullIntos()

	// 4.
	controller.resetQueue()

	// 5.
	controller.clearAlgorithms()

	// 6.
	stream.error(err)
}

// respond implements the ReadableByteStreamControllerRespond(bytesWritten) [specification].
//
// [specification]: https://streams.spec.whatwg.org/#readable-byte-stream-controller-respond
func (controller *ReadableByteStreamController) respond(bytesWritten int64) {
	// 1.
	if controller.pendingPullIntos == nil {
		common.Throw(
			controller.stream.vu.Runtime(),
			newError(AssertionError, "respond() called on a controller with no pending pullIntos"),
		)
	}

	// 2.
	firstDescriptor := controller.pendingPullIntos[0]

	// 3.
	state := controller.stream.state

	switch state {
	case ReadableStreamStateClosed: // 4.
		if bytesWritten != 0 {
			common.Throw(
				controller.stream.vu.Runtime(),
				newError(TypeError, "bytesWritten must be 0 when calling respond() on a closed stream"),
			)
		}
	default: // 5.
		// 5.1.
		if state != ReadableStreamStateReadable {
			common.Throw(controller.stream.vu.Runtime(), newError(AssertionError, "respond() called on a non-readable stream"))
		}

		// 5.2.
		if bytesWritten == 0 {
			common.Throw(
				controller.stream.vu.Runtime(),
				newError(TypeError, "bytesWritten must be > 0 when calling respond() on a readable stream"),
			)
		}

		// 5.3.
		// FIXME: If firstDescriptor’s bytes filled + bytesWritten > firstDescriptor’s byte length, throw a
		// RangeError exception.
		if firstDescriptor.bytesFilled+bytesWritten > firstDescriptor.byteLength {
			common.Throw(
				controller.stream.vu.Runtime(),
				newError(RangeError, "bytesWritten must be > 0 when calling respond() on a readable stream"),
			)
		}
	}

	// 6.
	firstDescriptor.buffer = transferArrayBuffer(controller.stream.runtime, firstDescriptor.buffer)

	// 7.
	controller.respondInternal(bytesWritten)
}

// respondInternal implements the ReadableByteStreamControllerRespondInternal [specification]
//
// [specification]: https://streams.spec.whatwg.org/#readable-byte-stream-controller-respond-internal
func (controller *ReadableByteStreamController) respondInternal(bytesWritten int64) {
	// 1.
	firstDescriptor := controller.pendingPullIntos[0]

	// 2.
	if !canTransferArrayBuffer(controller.stream.runtime, firstDescriptor.buffer) {
		common.Throw(controller.stream.vu.Runtime(), newError(AssertionError, "Cannot transfer buffer"))
	}

	// 3.
	controller.invalidateBYOBRequest()

	// 4.
	state := controller.stream.state

	// 5.
	if state == ReadableStreamStateClosed {
		// 5.1.
		if bytesWritten != 0 {
			common.Throw(
				controller.stream.vu.Runtime(),
				newError(AssertionError, "bytesWritten must be 0 when calling respond() on a closed stream"),
			)
		}

		// 5.2.
		controller.respondInClosedState(firstDescriptor)
	} else { // 6.
		// 6.1.
		if state != ReadableStreamStateReadable {
			common.Throw(controller.stream.vu.Runtime(), newError(AssertionError, "respond() called on a non-readable stream"))
		}

		// 6.2.
		if bytesWritten <= 0 {
			common.Throw(
				controller.stream.vu.Runtime(),
				newError(AssertionError, "bytesWritten must be > 0 when calling respond() on a readable stream"),
			)
		}

		// 6.3.
		controller.respondInReadableState(bytesWritten, firstDescriptor)
	}

	// 7.
	controller.callPullIfNeeded()
}

// respondInClosedState implements the ReadableByteStreamControllerRespondInClosedState [specification]
//
// [specification]: https://streams.spec.whatwg.org/#readable-byte-stream-controller-respond-in-closed-state
func (controller *ReadableByteStreamController) respondInClosedState(firstDescriptor PullIntoDescriptor) {
	// 1.
	if (firstDescriptor.bytesFilled / firstDescriptor.elementSize) == 0 {
		common.Throw(
			controller.stream.vu.Runtime(),
			newError(AssertionError, "firstDescriptor.bytesFilled is not a multiple of firstDescriptor.elementSize"),
		)
	}

	// 2.
	if firstDescriptor.readerType == ReaderTypeNone {
		controller.shiftPendingPullInto()
	}

	// 3.
	stream := controller.stream

	// 4.
	if stream.hasBYOBReader() {
		// 4.1.
		for stream.getNumReadIntoRequests() > 0 {
			// 4.1.1.
			pullIntoDescriptor := controller.shiftPendingPullInto()

			// 4.1.2.
			controller.commitPullIntoDescriptor(pullIntoDescriptor)
		}
	}
}

// respondInReadableState implements the ReadableByteStreamControllerRespondInReadableState [specification].
//
// [specification]: https://streams.spec.whatwg.org/#readable-byte-stream-controller-respond-in-readable-state
func (controller *ReadableByteStreamController) respondInReadableState(bytesWritten int64, pullIntoDescriptor PullIntoDescriptor) {
	// 1.
	if pullIntoDescriptor.bytesFilled+bytesWritten > pullIntoDescriptor.byteLength {
		common.Throw(
			controller.stream.vu.Runtime(),
			newError(AssertionError, "pullIntoDescriptor.bytesFilled + bytesWritten > pullIntoDescriptor.byteLength"),
		)
	}

	// 2.
	controller.fillHeadPullIntoDescriptor(bytesWritten, pullIntoDescriptor)

	// 3.
	if pullIntoDescriptor.readerType == ReaderTypeNone {
		// 3.1.
		controller.enqueueDetachedPullIntoToQueue(pullIntoDescriptor)

		// 3.2.
		controller.processPullIntoDescriptorsUsingQueue()

		// 3.3.
		return
	}

	// 4.
	if pullIntoDescriptor.bytesFilled < pullIntoDescriptor.minimumFill {
		return
	}

	// 5.
	controller.shiftPendingPullInto()

	// 6.
	remainderSize := pullIntoDescriptor.bytesFilled % pullIntoDescriptor.elementSize

	// 7.
	if remainderSize > 0 {
		// 7.1.
		end := pullIntoDescriptor.byteOffset + pullIntoDescriptor.bytesFilled

		// 7.2.
		controller.enqueueClonedChunkToQueue(pullIntoDescriptor.buffer, end-remainderSize, remainderSize)
	}

	// 8.
	pullIntoDescriptor.bytesFilled -= remainderSize

	// 9.
	controller.commitPullIntoDescriptor(pullIntoDescriptor)

	// 10.
	controller.processPullIntoDescriptorsUsingQueue()
}

// invalidateBYOBRequest implements the ReadableByteStreamControllerInvalidateBYOBRequest [specification].
//
// [specification]: https://streams.spec.whatwg.org/#readable-byte-stream-controller-invalidate-byob-request
func (controller *ReadableByteStreamController) invalidateBYOBRequest() {
	// 1.
	// FIXME: we're supposed to modify the internal field here
	if controller.ByobRequest == nil {
		return
	}

	// 2.
	// FIXME: we're supposed to modify the internal field here
	controller.ByobRequest.controller = nil

	// 3.
	// FIXME: we're supposed to modify the internal field here
	controller.ByobRequest.View = nil

	// 4.
	// FIXME: we're supposed to modify the internal field here
	controller.ByobRequest = nil
}

// callPullIfNeeded implements the ReadableByteStreamControllerCallPullIfNeeded [specification].
//
// [specification]: https://streams.spec.whatwg.org/#readable-byte-stream-controller-call-pull-if-needed
func (controller *ReadableByteStreamController) callPullIfNeeded() {
	// 1.
	shouldPull := controller.shouldCallPull()

	// 2.
	if !shouldPull {
		return
	}

	// 3.
	if controller.pulling {
		controller.pullAgain = true
		return
	}

	// 4.
	if controller.pullAgain {
		common.Throw(controller.stream.vu.Runtime(), newError(AssertionError, "pullAgain should be false"))
	}

	// 5.
	controller.pulling = true

	// 6.
	pullPromise := controller.pullAlgorithm()

	_, err := promiseThen(controller.stream.runtime, pullPromise,
		// 7. upon fulfillment
		func(value goja.Value) {
			// 7.1.
			controller.pulling = false

			// 7.2.
			if controller.pullAgain {
				controller.pullAgain = false
				controller.callPullIfNeeded()
			}
		},

		// 8. upon rejection
		func(reason goja.Value) {
			// FIXME: I'm pretty sure this doesn't work, we should probably pass down goja.Value all the way instead of error
			reasonError, _ := reason.Export().(error)
			controller.error(reasonError)
		},
	)
	if err != nil {
		common.Throw(controller.stream.vu.Runtime(), err)
	}
}

// shouldCallPull implements the ReadableByteStreamControllerShouldCallPull [specification].
//
// [specification]: https://streams.spec.whatwg.org/#readable-byte-stream-controller-should-call-pull
func (controller *ReadableByteStreamController) shouldCallPull() bool {
	// 1.
	stream := controller.stream

	// 2.
	if stream.state != ReadableStreamStateReadable {
		return false
	}

	// 3.
	if controller.closeRequested {
		return false
	}

	// 4.
	if !controller.started {
		return false
	}

	// 5.
	if stream.hasDefaultReader() && stream.getNumReadRequests() > 0 {
		return true
	}

	// 6.
	if stream.hasBYOBReader() && stream.getNumReadIntoRequests() > 0 {
		return true
	}

	// 7.
	desiredSize := controller.getDesiredSize()

	// 8.
	if !desiredSize.Valid {
		common.Throw(controller.stream.vu.Runtime(), newError(AssertionError, "desiredSize is nil"))
	}

	// 9.
	if desiredSize.ValueOrZero() > 0 {
		return true
	}

	return false
}

// shitPendingPullInto implements the ReadableByteStreamControllerShiftPendingPullInto [specification].
//
// [specification]: https://streams.spec.whatwg.org/#readable-byte-stream-controller-shift-pending-pull-into
func (controller *ReadableByteStreamController) shiftPendingPullInto() PullIntoDescriptor {
	// 1.
	if controller.ByobRequest == nil {
		common.Throw(controller.stream.vu.Runtime(), newError(AssertionError, "controller.ByobRequest is nil"))
	}

	// 2.
	descriptor := controller.pendingPullIntos[0]

	// 3.
	controller.pendingPullIntos = controller.pendingPullIntos[1:]

	// 4.
	return descriptor
}

// commitPullIntoDescriptor implements the ReadableByteStreamControllerCommitPullIntoDescriptor [specification].
//
// [specification]: https://streams.spec.whatwg.org/#readable-byte-stream-controller-commit-pull-into-descriptor
func (controller *ReadableByteStreamController) commitPullIntoDescriptor(pullIntoDescriptor PullIntoDescriptor) {
	// 1.
	if controller.stream.state == ReadableStreamStateErrored {
		common.Throw(controller.stream.vu.Runtime(), newError(AssertionError, "stream.state is errored"))
	}

	// 2.
	if pullIntoDescriptor.readerType == ReaderTypeNone {
		common.Throw(controller.stream.vu.Runtime(), newError(AssertionError, "pullIntoDescriptor.readerType is none"))
	}

	// 3.
	done := false

	// 4.
	if controller.stream.state == ReadableStreamStateClosed {
		// 4.1.
		if (pullIntoDescriptor.bytesFilled % pullIntoDescriptor.elementSize) != 0 {
			common.Throw(controller.stream.vu.Runtime(), newError(RangeError, "bytesFilled is not a multiple of elementSize"))
		}

		// 4.2.
		done = true
	}

	// 5.
	filledView := controller.convertPullIntoDescriptor(pullIntoDescriptor)

	// 6.
	if pullIntoDescriptor.readerType == ReaderTypeDefault {
		// 6.1.
		controller.stream.fulfillReadRequest(filledView, done)
	}

	// 7.
	if pullIntoDescriptor.readerType != ReaderTypeByob {
		common.Throw(controller.stream.vu.Runtime(), newError(AssertionError, "pullIntoDescriptor.readerType is not byob"))
	}

	// 8.
	controller.stream.fulfillReadIntoRequest(filledView, done)
}

func (controller *ReadableByteStreamController) convertPullIntoDescriptor(pullIntoDescriptor PullIntoDescriptor) goja.Value {
	// 1.
	bytesFilled := pullIntoDescriptor.bytesFilled

	// 2.
	elementSize := pullIntoDescriptor.elementSize

	// 3.
	if bytesFilled > pullIntoDescriptor.byteLength {
		common.Throw(controller.stream.vu.Runtime(), newError(AssertionError, "bytesFilled > pullIntoDescriptor.byteLength"))
	}

	// 4.
	if (bytesFilled % elementSize) != 0 {
		common.Throw(controller.stream.vu.Runtime(), newError(RangeError, "bytesFilled is not a multiple of elementSize"))
	}

	// 5.
	// FIXME
	// buffer := transferArrayBuffer(controller.stream.runtime, pullIntoDescriptor.buffer)

	// 6.
	// FIXME: this is not the way...
	// return pullIntoDescriptor.viewConstructor()(buffer, pullIntoDescriptor.byteOffset, bytesFilled%elementSize)
	// FIXME: drop once above is fixed
	return goja.Undefined()
}

// fillHeadPullIntoDescriptor implements the [ReadableByteStreamControllerFillHeadPullIntoDescriptor(size, pullIntoDescriptor)] algorithm
//
// [ReadableByteStreamControllerFillHeadPullIntoDescriptor(bytesWritten, pullIntoDescriptor)]: https://streams.spec.whatwg.org/#readable-byte-stream-controller-fill-head-pull-into-descriptor
func (controller *ReadableByteStreamController) fillHeadPullIntoDescriptor(size int64, pullIntoDescriptor PullIntoDescriptor) {
	// 1.
	// TODO: reassess this is the right way to match the assertion
	// FIXME: make it work somehow...
	// if len(controller.pendingPullIntos) != 0 && controller.pendingPullIntos[0] != pullIntoDescriptor {
	// 	common.Throw(controller.stream.vu.Runtime(), newError(AssertionError, "pullIntoDescriptor is not the first element of controller.pendingPullIntos"))
	// }

	// 2.
	if controller.ByobRequest != nil {
		common.Throw(controller.stream.vu.Runtime(), newError(AssertionError, "controller.ByobRequest is not nil"))
	}

	// 3.
	pullIntoDescriptor.bytesFilled += size
}

// enqueueDetachedPullIntoToQueue implements the [ReadableByteStreamControllerEnqueueDetachedPullIntoToQueue(pullIntoDescriptor)] algorithm
//
// [ReadableByteStreamControllerEnqueueDetachedPullIntoToQueue(pullIntoDescriptor)]: https://streams.spec.whatwg.org/#readable-byte-stream-controller-enqueue-detached-pull-into-to-queue
func (controller *ReadableByteStreamController) enqueueDetachedPullIntoToQueue(pullIntoDescriptor PullIntoDescriptor) {
	// 1.
	if pullIntoDescriptor.readerType != ReaderTypeNone {
		common.Throw(controller.stream.vu.Runtime(), newError(AssertionError, "pullIntoDescriptor.readerType is not none"))
	}

	// 2.
	if pullIntoDescriptor.bytesFilled > 0 {
		controller.enqueueClonedChunkToQueue(pullIntoDescriptor.buffer, pullIntoDescriptor.byteOffset, pullIntoDescriptor.bytesFilled)
	}

	// 3.
	controller.shiftPendingPullInto()
}

func (controller *ReadableByteStreamController) processPullIntoDescriptorsUsingQueue() {
	// 1.
	if controller.closeRequested {
		common.Throw(controller.stream.vu.Runtime(), newError(AssertionError, "closeRequested is true"))
	}

	// 2.
	for len(controller.pendingPullIntos) > 0 {
		// 2.1.
		if controller.queueTotalSize == 0 {
			return
		}

		// 2.2.
		pullIntoDescriptor := controller.pendingPullIntos[0]

		// 2.3.
		if controller.fillPullIntoDescriptorFromQueue(pullIntoDescriptor) {
			controller.shiftPendingPullInto()
			controller.commitPullIntoDescriptor(pullIntoDescriptor)
		}
	}
}

// processReadRequestsUsingQueue implements the [ReadableByteStreamControllerProcessReadRequestsUsingQueue()] algorithm
//
// [ReadableByteStreamControllerProcessReadRequestsUsingQueue()]: https://streams.spec.whatwg.org/#readable-byte-stream-controller-process-read-requests-using-queue
func (controller *ReadableByteStreamController) processReadRequestsUsingQueue() {
	// 1.
	reader := controller.stream.reader

	// 2.
	defaultReader, implementsDefaultReader := reader.(*ReadableStreamDefaultReader)
	if !implementsDefaultReader {
		common.Throw(controller.stream.vu.Runtime(), newError(AssertionError, "reader is not a ReadableStreamDefaultReader"))
	}

	// 3.
	for len(defaultReader.readRequests) > 0 {
		if controller.queueTotalSize == 0 {
			return
		}

		readRequest := defaultReader.readRequests[0]
		defaultReader.readRequests = defaultReader.readRequests[1:]
		controller.fillReadRequestFromQueue(readRequest)
	}
}

// fillReadRequestFromQueue implements the ReadableByteStreamControllerFillReadRequestFromQueue [specification].
//
// [specification]: https://streams.spec.whatwg.org/#readable-byte-stream-controller-fill-read-request-from-queue
func (controller *ReadableByteStreamController) fillReadRequestFromQueue(readRequest ReadRequest) {
	// 1.
	if controller.queueTotalSize == 0 {
		common.Throw(controller.stream.vu.Runtime(), newError(AssertionError, "queueTotalSize is 0"))
	}

	// 2.
	entry := controller.queue[0]

	// 3.
	controller.queue = controller.queue[1:]

	// 4.
	controller.queueTotalSize -= entry.byteLength

	// 5.
	controller.handleQueueDrain()

	// 6.
	// TODO: verify this is the right way to do it
	view, err := newUint8Array(controller.stream.runtime, entry.buffer.Bytes(), entry.byteOffset, entry.byteLength)
	if err != nil {
		common.Throw(controller.stream.vu.Runtime(), newError(RuntimeError, "failed to create Uint8Array"))
	}

	// 7.
	readRequest.chunkSteps(view)
}

func (controller *ReadableByteStreamController) fillPullIntoDescriptorFromQueue(pullIntoDescriptor PullIntoDescriptor) bool {
	// 1.
	// FIXME: replace with a call to Go's standard `min` function as soon as k6 uses Go version >= 1.21
	maxBytesToCopy := int64(math.Min(
		float64(controller.queueTotalSize),
		float64(pullIntoDescriptor.byteLength-pullIntoDescriptor.bytesFilled),
	))

	// 2.
	maxBytesFilled := pullIntoDescriptor.bytesFilled + maxBytesToCopy

	// 3.
	totalBytesToCopyRemaining := maxBytesToCopy

	// 4.
	ready := false

	// 5.
	if pullIntoDescriptor.bytesFilled >= pullIntoDescriptor.minimumFill {
		common.Throw(
			controller.stream.vu.Runtime(),
			newError(AssertionError, "pullIntoDescriptor.bytesFilled >= pullIntoDescriptor.minimumFill"),
		)
	}

	// 6.
	remainderBytes := maxBytesFilled % pullIntoDescriptor.elementSize

	// 7.
	maxAlignedBytes := maxBytesFilled - remainderBytes

	// 8.
	if maxAlignedBytes >= pullIntoDescriptor.minimumFill {
		totalBytesToCopyRemaining = maxAlignedBytes - pullIntoDescriptor.bytesFilled
		ready = true
	}

	// 9.
	queue := controller.queue

	// 10.
	for totalBytesToCopyRemaining > 0 {
		headOfQueue := queue[0]
		bytesToCopy := int64(math.Min(float64(totalBytesToCopyRemaining), float64(headOfQueue.byteLength)))

		// FIXME: reactivate when the step 4. is implemented
		// destStart := pullIntoDescriptor.byteOffset + pullIntoDescriptor.bytesFilled
		// 4.
		// TODO: Perform ! CopyDataBlockBytes(pullIntoDescriptor’s buffer.[[ArrayBufferData]], destStart, headOfQueue’s buffer.[[ArrayBufferData]], headOfQueue’s byte offset, bytesToCopy).

		if headOfQueue.byteLength == bytesToCopy {
			queue = queue[1:]
		} else {
			// 6.
			headOfQueue.byteOffset += bytesToCopy
			headOfQueue.byteLength -= bytesToCopy
		}

		controller.queueTotalSize -= bytesToCopy
		controller.fillHeadPullIntoDescriptor(bytesToCopy, pullIntoDescriptor)
		totalBytesToCopyRemaining -= bytesToCopy
	}

	// 11.
	if !ready {
		if controller.queueTotalSize != 0 {
			common.Throw(controller.stream.vu.Runtime(), newError(AssertionError, "queueTotalSize is not 0"))
		}

		if pullIntoDescriptor.bytesFilled <= 0 {
			common.Throw(controller.stream.vu.Runtime(), newError(AssertionError, "pullIntoDescriptor.bytesFilled is not > 0"))
		}

		if pullIntoDescriptor.bytesFilled >= pullIntoDescriptor.minimumFill {
			common.Throw(
				controller.stream.vu.Runtime(),
				newError(AssertionError, "pullIntoDescriptor.bytesFilled >= pullIntoDescriptor.minimumFill"),
			)
		}
	}

	return ready
}

// handleQueueDrain implements the [specification] ReadableByteStreamControllerHandleQueueDrain() algorithm
//
// [specification]: https://streams.spec.whatwg.org/#readable-byte-stream-controller-handle-queue-drain
func (controller *ReadableByteStreamController) handleQueueDrain() {
	// 1.
	if controller.stream.state != ReadableStreamStateReadable {
		common.Throw(controller.stream.vu.Runtime(), newError(AssertionError, "stream.state is not readable"))
	}

	// 2.
	if controller.queueTotalSize == 0 && controller.closeRequested {
		controller.clearAlgorithms()
		controller.stream.close()
	}

	// 3.
	controller.callPullIfNeeded()
}

// enqueueClonedChunkToQueue implements the [specification] ReadableByteStreamControllerEnqueueClonedChunkToQueue
// algorithm.
//
// [specification]: https://streams.spec.whatwg.org/#readable-byte-stream-controller-enqueue-cloned-chunk-to-queue
func (controller *ReadableByteStreamController) enqueueClonedChunkToQueue(buffer goja.ArrayBuffer, byteOffset int64, byteLength int64) {
	// 1.
	cloneResult, err := CloneArrayBuffer(controller.stream.runtime, buffer, byteOffset, byteLength)
	// 2.
	if err != nil {
		// FIXME: uncomment the following lines
		// controller.error(cloneResult)
		// return cloneResult
	}

	// 3.
	controller.enqueueChunkToQueue(cloneResult, 0, byteLength)
}

// enqueueChunkToQueue implements the [ReadableByteStreamControllerEnqueueChunkToQueue(buffer, byteOffset, byteLength)] algorithm
//
// [ReadableByteStreamControllerEnqueueChunkToQueue(buffer, byteOffset, byteLength)]: https://streams.spec.whatwg.org/#readable-byte-stream-controller-enqueue-chunk-to-queue
func (controller *ReadableByteStreamController) enqueueChunkToQueue(buffer goja.ArrayBuffer, byteOffset int64, byteLength int64) {
	// 1.
	// FIXME: This should probably use queue.Enqueue??
	controller.queue = append(controller.queue, ReadableByteStreamQueueEntry{
		buffer:     buffer,
		byteOffset: byteOffset,
		byteLength: byteLength,
	})

	// 2.
	controller.queueTotalSize += byteLength
}

func (controller *ReadableByteStreamController) getDesiredSize() null.Int {
	// 1.
	state := controller.stream.state

	// 2.
	if state == ReadableStreamStateErrored {
		return null.Int{}
	}

	// 3.
	if state == ReadableStreamStateClosed {
		return null.IntFrom(0)
	}

	// 4.
	return null.IntFrom(controller.strategyHWM - controller.queueTotalSize)
}

// clearPendingPullIntos implements the ReadableByteStreamControllerClearPendingPullIntos [specification].
//
// [specification]: https://streams.spec.whatwg.org/#readable-byte-stream-controller-clear-pending-pull-intos
func (controller *ReadableByteStreamController) clearPendingPullIntos() {
	// 1.
	controller.invalidateBYOBRequest()

	// 2.
	controller.pendingPullIntos = []PullIntoDescriptor{}
}

// resetQueue implements the [ResetQueue(container)] algorithm for [ReadableByteStreamController].
//
// [ResetQueue(container)]: https://streams.spec.whatwg.org/#reset-queue
func (controller *ReadableByteStreamController) resetQueue() {
	controller.queue = []ReadableByteStreamQueueEntry{}
	controller.queueTotalSize = 0
}

// clearAlgorithms implements the [ReadableByteStreamControllerClearAlgorithms()] algorithm
//
// [ReadableByteStreamControllerClearAlgorithms()]: https://streams.spec.whatwg.org/#readable-byte-stream-controller-clear-algorithms
func (controller *ReadableByteStreamController) clearAlgorithms() {
	controller.pullAlgorithm = nil
	controller.cancelAlgorithm = nil
}

type ReadableStreamBYOBRequest struct {
	// View holds the view for writing in to, or null if the BYOB request has alread been
	// responded to.
	View goja.Value

	// controller points to the parent [ReadableByteStreamController] instance
	controller *ReadableByteStreamController

	// TODO: not necessary in our Go implementation?
	// view holds a typed array representing the destination region to which the
	// controller can write generated data, or null after the BYOB request has
	// been invalidated.
	// view []byte
}

// Respond implements the [respond(bytesWritten)] specification algorithm.
//
// It is used to respond to a BYOB request with the given number of bytes written.
//
// [respond(bytesWritten)]: https://streams.spec.whatwg.org/#rs-byob-request-respond
func (rsbr *ReadableStreamBYOBRequest) Respond(bytesWritten int) {
	if rsbr.controller == nil {
		common.Throw(rsbr.controller.stream.vu.Runtime(), newError(TypeError, "unable to respond to a BYOB request that has already been responded to"))
	}

	var ab goja.ArrayBuffer
	if err := rsbr.controller.stream.runtime.ExportTo(rsbr.View, &ab); err != nil {
		common.Throw(rsbr.controller.stream.vu.Runtime(), newError(RuntimeError, "unable to treat BYOB request's view as an ArrayBuffer"))
	}

	if ab.Detached() {
		common.Throw(rsbr.controller.stream.vu.Runtime(), newError(TypeError, "unable to respond to a BYOB request that has already been responded to"))
	}

	// In Goja world I believe rsbr caters to both steps 3. and 4. of the specs.
	if len(ab.Bytes()) <= 0 {
		common.Throw(rsbr.controller.stream.vu.Runtime(), newError(AssertionError, "cannot respond to a BYOB request with an empty view"))
	}

	// 5. Perform ReadableByteStreamControllerRespond(rsbr.[[controller]], bytesWritten).
	// TODO: implement
}

// ReadableByteStreamQueueEntry encapsulates the important aspects of a chunk for the specific
// case of readable byte streams.
type ReadableByteStreamQueueEntry struct {
	// buffer holds an ArrayBuffer, which will be a transferred version of the one originally supplied by the underlying byte source.
	buffer goja.ArrayBuffer

	// byteOffset holds a nonnegative integer number giving the byte offset derived from the view originally supplied by the underlying byte source.
	byteOffset int64

	// byteLength holds a nonnegative integer number giving the byte length derived from the view originally supplied by the underlying byte source.
	byteLength int64
}

type PullIntoDescriptor struct {
	// buffer holds an ArrayBuffer which will be a transferred version of the one
	// originally supplied by the underlying byte source.
	buffer goja.ArrayBuffer

	// bufferByteLength holds a positive integer representing the initial
	// byte length of buffer
	bufferByteLength int64

	// byteOffset holds a nonnegative integer offset into the buffer
	// where the underlying byt source will be starting.
	byteOffset int64

	// byteLength holds a positive integer number of bytes which
	// can be written into the buffer
	byteLength int64

	// bytesFilled holds a nonnegative integer number of bytes that
	// have been written into the buffer so far.
	bytesFilled int64

	// minimumFill holds a positive integer representing the minimum
	// number of bytes that must be written into the buffer before
	// the associated read() request may be fulfilled.
	//
	// By default, this equals the element size.
	minimumFill int64

	// elementsSize holds a positive integer representing the number
	// of bytes that can be written into the buffer at a time, using
	// views of the type described by the view constructor.
	elementSize int64

	// viewConstructor holds a typed array constructor function, used for constructing
	// a view with which to write into the buffer.
	// FIXME: is this the correct type?
	viewConstructor func() goja.ArrayBuffer

	// readerType holds either "default" or "byob", indicating what
	// type of readable stream reader initiated this request, or
	// "none" if the initiating reader was released.
	readerType ReaderType
}

// ReaderType is a type alias for the possible values of the [pullIntoDescriptor]'s readerType field
type ReaderType = string

const (
	// ReaderTypeDefault is the value of the [pullIntoDescriptor]'s readerType field when the
	// initiating reader is a default reader.
	ReaderTypeDefault ReaderType = "default"

	// ReaderTypeByob is the value of the [pullIntoDescriptor]'s readerType field when the
	// initiating reader is a BYOB reader.
	ReaderTypeByob ReaderType = "byob"

	// ReaderTypeNone is the value of the [pullIntoDescriptor]'s readerType field when the
	// initiating reader has been released.
	ReaderTypeNone ReaderType = "none"
)
