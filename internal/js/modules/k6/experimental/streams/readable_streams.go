package streams

import (
	"errors"

	"github.com/grafana/sobek"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
)

// ReadableStream is a concrete instance of the general [readable stream] concept.
//
// It is adaptable to any chunk type, and maintains an internal queue to keep track of
// data supplied by the underlying source but not yet read by any consumer.
//
// [readable stream]: https://streams.spec.whatwg.org/#rs-class
type ReadableStream struct {
	// Locked indicate whether the readable stream is locked to a reader
	Locked bool

	// controller holds a [ReadableStreamDefaultController] or [ReadableByteStreamController] created
	// with the ability to control the state and queue of this stream.
	controller ReadableStreamController

	// disturbed is true when the stream has been read from or canceled
	disturbed bool

	// reader holds the current reader of the stream if the stream is locked to a reader
	// or nil otherwise.
	reader any

	// state holds the current state of the stream
	state ReadableStreamState

	// storedError holds the error that caused the stream to be errored
	storedError any

	Source *sobek.Object

	runtime *sobek.Runtime
	vu      modules.VU
}

// Cancel cancels the stream and returns a Promise to the user
func (stream *ReadableStream) Cancel(reason sobek.Value) (*sobek.Promise, error) {
	// 1. IsReadableStreamLocked(this) is true, return a promise rejected with a TypeError exception.
	if stream.isLocked() {
		promise, _, reject := stream.vu.Runtime().NewPromise()

		if err := reject(newTypeError(stream.runtime, "cannot cancel a locked stream").Err()); err != nil {
			return nil, err
		}

		return promise, nil
	}

	// 2. Return ! ReadableStreamCancel(reason)
	return stream.cancel(reason), nil
}

// GetReader implements the [getReader] operation.
//
// [getReader]: https://streams.spec.whatwg.org/#rs-get-reader
func (stream *ReadableStream) GetReader(options *sobek.Object) sobek.Value {
	// 1. If options["mode"] does not exist, return ? AcquireReadableStreamDefaultReader(this).
	if options == nil || common.IsNullish(options) ||
		options.Get("mode") == nil || sobek.IsUndefined(options.Get("mode")) {
		defaultReader := stream.acquireDefaultReader()
		defaultReaderObj, err := NewReadableStreamDefaultReaderObject(defaultReader)
		if err != nil {
			common.Throw(stream.runtime, err)
		}

		return defaultReaderObj
	}

	// 2. Assert: options["mode"] is "byob".
	if options.Get("mode").String() != "byob" {
		throw(stream.runtime, newTypeError(stream.runtime, "options.mode is not 'byob'"))
	}

	// 3. Return ? AcquireReadableStreamBYOBReader(this).
	common.Throw(stream.runtime, newError(NotSupportedError, "'byob' mode is not supported yet"))
	return sobek.Undefined()
}

// Tee implements the [tee] operation.
//
// [tee]: https://streams.spec.whatwg.org/#rs-tee
func (stream *ReadableStream) Tee() sobek.Value {
	common.Throw(stream.runtime, newError(NotSupportedError, "'tee()' is not supported yet"))
	return sobek.Undefined()
}

// ReadableStreamState represents the current state of a ReadableStream
type ReadableStreamState string

const (
	// ReadableStreamStateReadable indicates that the stream is readable, and that more data may be read from the stream.
	ReadableStreamStateReadable = "readable"

	// ReadableStreamStateClosed indicates that the stream is closed and cannot be read from.
	ReadableStreamStateClosed = "closed"

	// ReadableStreamStateErrored indicates that the stream has been aborted (errored).
	ReadableStreamStateErrored = "errored"
)

// ReadableStreamType represents the type of the ReadableStream
type ReadableStreamType = string

const (
	// ReadableStreamTypeBytes indicates that the stream is a byte stream.
	ReadableStreamTypeBytes = "bytes"
)

// isLocked implements the specification's [IsReadableStreamLocked()] abstract operation.
//
// [IsReadableStreamLocked()]: https://streams.spec.whatwg.org/#is-readable-stream-locked
func (stream *ReadableStream) isLocked() bool {
	return stream.reader != nil
}

// initialize implements the specification's [InitializeReadableStream()] abstract operation.
//
// [InitializeReadableStream()]: https://streams.spec.whatwg.org/#initialize-readable-stream
func (stream *ReadableStream) initialize() {
	stream.state = ReadableStreamStateReadable
	stream.reader = nil
	stream.Locked = false
	stream.storedError = nil
	stream.disturbed = false
}

// setupReadableStreamDefaultControllerFromUnderlyingSource implements the [specification]'s
// SetUpReadableStreamDefaultController abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#set-up-readable-stream-default-controller-from-underlying-source
func (stream *ReadableStream) setupReadableStreamDefaultControllerFromUnderlyingSource(
	underlyingSource *sobek.Object,
	underlyingSourceDict UnderlyingSource,
	highWaterMark float64,
	sizeAlgorithm SizeAlgorithm,
) {
	// 1. Let controller be a new ReadableStreamDefaultController.
	controller := &ReadableStreamDefaultController{}

	// 2. Let startAlgorithm be an algorithm that returns undefined.
	var startAlgorithm UnderlyingSourceStartCallback = func(*sobek.Object) sobek.Value {
		return sobek.Undefined()
	}

	// 3. Let pullAlgorithm be an algorithm that returns a promise resolved with undefined.
	var pullAlgorithm UnderlyingSourcePullCallback = func(*sobek.Object) *sobek.Promise {
		return newResolvedPromise(stream.vu, sobek.Undefined())
	}

	// 4. Let cancelAlgorithm be an algorithm that returns a promise resolved with undefined.
	var cancelAlgorithm UnderlyingSourceCancelCallback = func(any) sobek.Value {
		return stream.vu.Runtime().ToValue(newResolvedPromise(stream.vu, sobek.Undefined()))
	}

	// 5. If underlyingSourceDict["start"] exists, then set startAlgorithm to an algorithm
	// which returns the result of invoking underlyingSourceDict["start"] with argument
	// list « controller » and callback this value underlyingSource.
	if underlyingSourceDict.startSet {
		startAlgorithm = stream.startAlgorithm(underlyingSource, underlyingSourceDict)
	}

	// 6. If underlyingSourceDict["pull"] exists, then set pullAlgorithm to an algorithm which
	// returns the result of invoking underlyingSourceDict["pull"] with argument list
	// « controller » and callback this value underlyingSource.
	if underlyingSourceDict.pullSet {
		pullAlgorithm = stream.pullAlgorithm(underlyingSource, underlyingSourceDict)
	}

	// 7. If underlyingSourceDict["cancel"] exists, then set cancelAlgorithm to an algorithm which takes an argument
	// reason and returns the result of invoking underlyingSourceDict["cancel"] with argument list « reason » and
	// callback this value underlyingSource.
	if underlyingSourceDict.cancelSet {
		cancelAlgorithm = stream.cancelAlgorithm(underlyingSource, underlyingSourceDict)
	}

	// 8. Perform ? SetUpReadableStreamDefaultController(...)
	stream.setupDefaultController(controller, startAlgorithm, pullAlgorithm, cancelAlgorithm, highWaterMark, sizeAlgorithm)
}

func (stream *ReadableStream) startAlgorithm(
	underlyingSource *sobek.Object,
	underlyingSourceDict UnderlyingSource,
) UnderlyingSourceStartCallback {
	call, ok := sobek.AssertFunction(underlyingSourceDict.Start)
	if !ok {
		throw(stream.runtime, newTypeError(stream.runtime, "underlyingSource.[[start]] must be a function"))
	}

	return func(obj *sobek.Object) (v sobek.Value) {
		var err error
		v, err = call(underlyingSource, obj)
		if err != nil {
			panic(err)
		}

		return v
	}
}

func (stream *ReadableStream) pullAlgorithm(
	underlyingSource *sobek.Object,
	underlyingSourceDict UnderlyingSource,
) UnderlyingSourcePullCallback {
	call, ok := sobek.AssertFunction(underlyingSourceDict.Pull)
	if !ok {
		throw(stream.runtime, newTypeError(stream.runtime, "underlyingSource.[[pull]] must be a function"))
	}

	return func(obj *sobek.Object) *sobek.Promise {
		v, err := call(underlyingSource, obj)
		if err != nil {
			var ex *sobek.Exception
			if errors.As(err, &ex) {
				return newRejectedPromise(stream.vu, ex.Value())
			}
			return newRejectedPromise(stream.vu, err)
		}

		if p, ok := v.Export().(*sobek.Promise); ok {
			return p
		}

		return newResolvedPromise(stream.vu, v)
	}
}

func (stream *ReadableStream) cancelAlgorithm(
	underlyingSource *sobek.Object,
	underlyingSourceDict UnderlyingSource,
) UnderlyingSourceCancelCallback {
	call, ok := sobek.AssertFunction(underlyingSourceDict.Cancel)
	if !ok {
		throw(stream.runtime, newTypeError(stream.runtime, "underlyingSource.[[cancel]] must be a function"))
	}

	return func(reason any) sobek.Value {
		var p *sobek.Promise

		if e := stream.runtime.Try(func() {
			res, err := call(underlyingSource, stream.runtime.ToValue(reason))
			if err != nil {
				panic(err)
			}

			if cp, ok := res.Export().(*sobek.Promise); ok {
				p = cp
			}
		}); e != nil {
			p = newRejectedPromise(stream.vu, e.Value())
		}

		if p == nil {
			p = newResolvedPromise(stream.vu, sobek.Undefined())
		}

		return stream.vu.Runtime().ToValue(p)
	}
}

// setupDefaultController implements the specification's [SetUpReadableStreamDefaultController] abstract operation.
//
// [SetUpReadableStreamDefaultController]: https://streams.spec.whatwg.org/#set-up-readable-stream-default-controller
func (stream *ReadableStream) setupDefaultController(
	controller *ReadableStreamDefaultController,
	startAlgorithm UnderlyingSourceStartCallback,
	pullAlgorithm UnderlyingSourcePullCallback,
	cancelAlgorithm UnderlyingSourceCancelCallback,
	highWaterMark float64,
	sizeAlgorithm SizeAlgorithm,
) {
	rt := stream.vu.Runtime()

	// 1. Assert: stream.[[controller]] is undefined.
	if stream.controller != nil {
		common.Throw(rt, newError(AssertionError, "stream.[[controller]] is not undefined"))
	}

	// 2. Set controller.[[stream]] to stream.
	controller.stream = stream

	// 3. Perform ! ResetQueue(controller).
	controller.resetQueue()

	// 4. Set controller.[[started]], controller.[[closeRequested]], controller.[[pullAgain]], and
	// controller.[[pulling]] to false.
	controller.started, controller.closeRequested, controller.pullAgain, controller.pulling = false, false, false, false

	// 5. Set controller.[[strategySizeAlgorithm]] to sizeAlgorithm and controller.[[strategyHWM]] to highWaterMark.
	controller.strategySizeAlgorithm, controller.strategyHWM = sizeAlgorithm, highWaterMark

	// 6. Set controller.[[pullAlgorithm]] to pullAlgorithm.
	controller.pullAlgorithm = pullAlgorithm

	// 7. Set controller.[[cancelAlgorithm]] to cancelAlgorithm.
	controller.cancelAlgorithm = cancelAlgorithm

	// 8. Set stream.[[controller]] to controller.
	stream.controller = controller

	// 9. Let startResult be the result of performing startAlgorithm. (This might throw an exception.)
	controllerObj, err := controller.toObject()
	if err != nil {
		common.Throw(controller.stream.vu.Runtime(), newError(RuntimeError, err.Error()))
	}
	startResult := startAlgorithm(controllerObj)

	// 10. Let startPromise be a promise with startResult.
	var startPromise *sobek.Promise
	if common.IsNullish(startResult) {
		startPromise = newResolvedPromise(controller.stream.vu, startResult)
	} else if p, ok := startResult.Export().(*sobek.Promise); ok {
		if p.State() == sobek.PromiseStateRejected {
			controller.error(p.Result())
		}
		startPromise = p
	} else {
		startPromise = newResolvedPromise(controller.stream.vu, startResult)
	}
	_, err = promiseThen(stream.vu.Runtime(), startPromise,
		// 11. Upon fulfillment of startPromise,
		func(sobek.Value) {
			// 11.1. Set controller.[[started]] to true.
			controller.started = true
			// 11.2. Assert: controller.[[pulling]] is false.
			if controller.pulling {
				common.Throw(stream.vu.Runtime(), newError(AssertionError, "controller `pulling` state is not false"))
			}
			// 11.3. Assert: controller.[[pullAgain]] is false.
			if controller.pullAgain {
				common.Throw(stream.vu.Runtime(), newError(AssertionError, "controller `pullAgain` state is not false"))
			}
			// 11.4. Perform ! ReadableStreamDefaultControllerCallPullIfNeeded(controller).
			controller.callPullIfNeeded()
		},
		// 12. Upon rejection of startPromise with reason r,
		func(err sobek.Value) {
			controller.error(err)
		},
	)
	if err != nil {
		common.Throw(stream.vu.Runtime(), err)
	}
}

// acquireDefaultReader implements the specification's [AcquireReadableStreamDefaultReader] algorithm.
//
// [AcquireReadableStreamDefaultReader]: https://streams.spec.whatwg.org/#acquire-readable-stream-reader
func (stream *ReadableStream) acquireDefaultReader() *ReadableStreamDefaultReader {
	// 1. Let reader be a new ReadableStreamDefaultReader.
	reader := &ReadableStreamDefaultReader{}

	// 2. Perform ? SetUpReadableStreamDefaultReader(reader, stream).
	reader.setup(stream)

	// 3. Return reader.
	return reader
}

// addReadRequest implements the specification's [ReadableStreamAddReadRequest()] abstract operation.
//
// [ReadableStreamAddReadRequest()]: https://streams.spec.whatwg.org/#readable-stream-add-read-request
func (stream *ReadableStream) addReadRequest(readRequest ReadRequest) {
	// 1. Assert: stream.[[reader]] implements ReadableStreamDefaultReader.
	defaultReader, ok := stream.reader.(*ReadableStreamDefaultReader)
	if !ok {
		readRequest.errorSteps(newError(RuntimeError, "reader is not a ReadableStreamDefaultReader"))
		return
	}

	// 2. Assert: stream.[[state]] is "readable".
	if stream.state != ReadableStreamStateReadable {
		readRequest.errorSteps(newError(AssertionError, "stream is not readable"))
		return
	}

	// 3. Append readRequest to stream.[[reader]].[[readRequests]].
	defaultReader.readRequests = append(defaultReader.readRequests, readRequest)
}

// cancel implements the specification's [ReadableStreamCancel()] abstract operation.
//
// [ReadableStreamCancel()]: https://streams.spec.whatwg.org/#readable-stream-cancel
func (stream *ReadableStream) cancel(reason sobek.Value) *sobek.Promise {
	// 1. Set stream.[[disturbed]] to true.
	stream.disturbed = true

	// 2. If stream.[[state]] is "closed", return a promise resolved with undefined.
	if stream.state == ReadableStreamStateClosed {
		return newResolvedPromise(stream.vu, sobek.Undefined())
	}

	// 3. If stream.[[state]] is "errored", return a promise rejected with stream.[[storedError]].
	if stream.state == ReadableStreamStateErrored {
		if jsErr, ok := stream.storedError.(*jsError); ok {
			return newRejectedPromise(stream.vu, jsErr.Err())
		}
		return newRejectedPromise(stream.vu, stream.storedError)
	}

	// 4. Perform ! ReadableStreamClose(stream).
	stream.close()

	// 5. Let reader be stream.[[reader]].
	// 6. If reader is not undefined and reader implements ReadableStreamBYOBReader,
	// Not implemented yet: ReadableStreamBYOBReader is not supported yet.

	// 7. Let sourceCancelPromise be ! stream.[[controller]].[[CancelSteps]](reason).
	sourceCancelPromise := stream.controller.cancelSteps(reason)

	// 8. Return the result of reacting to sourceCancelPromise with a fulfillment step that returns undefined.
	promise, err := promiseThen(stream.vu.Runtime(), sourceCancelPromise,
		// Mimicking Deno's implementation: https://github.com/denoland/deno/blob/main/ext/web/06_streams.js#L405
		func(sobek.Value) {},
		func(err sobek.Value) { throw(stream.vu.Runtime(), err) },
	)
	if err != nil {
		common.Throw(stream.vu.Runtime(), err)
	}

	return promise
}

// close implements the specification's [ReadableStreamClose()] abstract operation.
//
// [ReadableStreamClose()]: https://streams.spec.whatwg.org/#readable-stream-close
func (stream *ReadableStream) close() {
	// 1. Assert: stream.[[state]] is "readable".
	if stream.state != ReadableStreamStateReadable {
		common.Throw(stream.vu.Runtime(), newError(AssertionError, "cannot close a stream that is not readable"))
	}

	// 2. Set stream.[[state]] to "closed".
	stream.state = ReadableStreamStateClosed

	// 3. Let reader be stream.[[reader]].
	reader := stream.reader

	// 4. If reader is undefined, return.
	if reader == nil {
		return
	}

	// 5. Resolve reader.[[closedPromise]] with undefined.
	genericReader, ok := reader.(ReadableStreamGenericReader)
	if !ok {
		common.Throw(stream.vu.Runtime(), newError(AssertionError, "reader is not a ReadableStreamGenericReader"))
	}

	_, resolveFunc, _ := genericReader.GetClosed()
	err := resolveFunc(sobek.Undefined())
	if err != nil {
		panic(err) // TODO(@mstoykov): propagate as error instead
	}

	// 6. If reader implements ReadableStreamDefaultReader,
	defaultReader, ok := reader.(*ReadableStreamDefaultReader)
	if ok {
		// 6.1. Let readRequests be reader.[[readRequests]].
		readRequests := defaultReader.readRequests

		// 6.2. Set reader.[[readRequests]] to an empty list.
		defaultReader.readRequests = []ReadRequest{}

		// 6.3. For each readRequest of readRequests,
		for _, readRequest := range readRequests {
			readRequest.closeSteps()
		}
	}
}

// error implements the specification's [ReadableStreamError] abstract operation.
//
// [ReadableStreamError]: https://streams.spec.whatwg.org/#readable-stream-error
func (stream *ReadableStream) error(e any) {
	// 1. Assert: stream.[[state]] is "readable".
	if stream.state != ReadableStreamStateReadable {
		common.Throw(stream.vu.Runtime(), newError(AssertionError, "cannot error a stream that is not readable"))
	}

	// 2. Set stream.[[state]] to "errored".
	stream.state = ReadableStreamStateErrored

	// 3. Set stream.[[storedError]] to e.
	stream.storedError = e

	// 4. Let reader be stream.[[reader]].
	reader := stream.reader

	// 5. If reader is undefined, return.
	if reader == nil {
		return
	}

	genericReader, ok := reader.(ReadableStreamGenericReader)
	if !ok {
		common.Throw(stream.vu.Runtime(), newError(AssertionError, "reader is not a ReadableStreamGenericReader"))
	}

	// 6. Reject reader.[[closedPromise]] with e.
	var err error
	promise, _, rejectFunc := genericReader.GetClosed()
	if jsErr, ok := e.(*jsError); ok {
		err = rejectFunc(jsErr.Err())
	} else {
		err = rejectFunc(e)
	}
	if err != nil {
		panic(err) // TODO(@mstoykov): propagate as error instead
	}

	// 7. Set reader.[[closedPromise]].[[PromiseIsHandled]] to true.
	// See https://github.com/dop251/goja/issues/565
	doNothing := func(sobek.Value) {}
	_, err = promiseThen(stream.vu.Runtime(), promise, doNothing, doNothing)
	if err != nil {
		common.Throw(stream.vu.Runtime(), newError(RuntimeError, err.Error()))
	}

	// 8. If reader implements ReadableStreamDefaultReader,
	defaultReader, ok := reader.(*ReadableStreamDefaultReader)
	if ok {
		// 8.1. Perform ! ReadableStreamDefaultReaderErrorReadRequests(reader, e).
		defaultReader.errorReadRequests(e)
		return
	}

	// 9. OTHERWISE, reader is a ReadableStreamBYOBReader
	// 9.1. Assert: reader implements ReadableStreamBYOBReader.
	common.Throw(stream.vu.Runtime(), newError(NotSupportedError, "ReadableStreamBYOBReader is not supported yet"))
}

// fulfillReadRequest implements the [ReadableStreamFulfillReadRequest()] algorithm.
//
// [ReadableStreamFulfillReadRequest()]: https://streams.spec.whatwg.org/#readable-stream-fulfill-read-request
func (stream *ReadableStream) fulfillReadRequest(chunk any, done bool) {
	// 1. Assert: ! ReadableStreamHasDefaultReader(stream) is true.
	if !stream.hasDefaultReader() {
		common.Throw(stream.vu.Runtime(), newError(AssertionError, "stream does not have a default reader"))
	}

	// 2. Let reader be stream.[[reader]].
	reader, ok := stream.reader.(*ReadableStreamDefaultReader)
	if !ok {
		common.Throw(stream.vu.Runtime(), newError(RuntimeError, "reader is not a ReadableStreamDefaultReader"))
	}

	// 3. Assert: reader.[[readRequests]] is not empty.
	if len(reader.readRequests) == 0 {
		common.Throw(stream.vu.Runtime(), newError(AssertionError, "reader.[[readRequests]] is empty"))
	}

	// 4. Let readRequest be reader.[[readRequests]][0].
	readRequest := reader.readRequests[0]

	// 5. Remove readRequest from reader.[[readRequests]].
	reader.readRequests = reader.readRequests[1:]

	if done {
		// 6. If done is true, perform readRequest’s close steps.
		readRequest.closeSteps()
	} else {
		// 7. Otherwise, perform readRequest’s chunk steps, given chunk.
		readRequest.chunkSteps(chunk)
	}
}

// getNumReadRequests implements the [ReadableStreamGetNumReadRequests()] algorithm.
//
// [ReadableStreamGetNumReadRequests()]: https://streams.spec.whatwg.org/#readable-stream-get-num-read-requests
func (stream *ReadableStream) getNumReadRequests() int {
	// 1. Assert: ! ReadableStreamHasDefaultReader(stream) is true.
	if !stream.hasDefaultReader() {
		common.Throw(stream.vu.Runtime(), newError(AssertionError, "stream does not have a default reader"))
	}

	// 2. Return stream.[[reader]].[[readRequests]]'s size.
	defaultReader, ok := stream.reader.(*ReadableStreamDefaultReader)
	if !ok {
		common.Throw(stream.vu.Runtime(), newError(RuntimeError, "reader is not a ReadableStreamDefaultReader"))
	}

	return len(defaultReader.readRequests)
}

// hasDefaultReader implements the [ReadableStreamHasDefaultReader()] algorithm.
//
// [ReadableStreamHasDefaultReader()]: https://streams.spec.whatwg.org/#readable-stream-has-default-reader
func (stream *ReadableStream) hasDefaultReader() bool {
	// 1. Let reader be stream.[[reader]].
	reader := stream.reader

	// 2. If reader is undefined, return false.
	if reader == nil {
		return false
	}

	// 3. If reader implements ReadableStreamDefaultReader, return true.
	_, ok := reader.(*ReadableStreamDefaultReader)
	return ok
}
