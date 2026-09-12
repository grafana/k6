package streams

import (
	"github.com/grafana/sobek"

	"go.k6.io/k6/v2/js/common"
	"go.k6.io/k6/v2/js/modules"
)

// TransformStream is a concrete instance of the general [transform stream] concept.
//
// It consists of a pair of streams: a [WritableStream], known as its writable side, and a
// [ReadableStream], known as its readable side. Writes to the writable side result, after a
// transformation, in new data being made available for reading from the readable side.
//
// [transform stream]: https://streams.spec.whatwg.org/#ts-class
type TransformStream struct {
	// backpressure holds whether there was backpressure on [readable] the last time it was
	// observed.
	backpressure bool

	// backpressureChangePromise is a promise which is fulfilled and replaced every time the
	// value of backpressure changes.
	backpressureChangePromise *promiseWrapper

	// controller holds a [TransformStreamDefaultController] created with the ability to
	// control [readable] and [writable].
	controller *TransformStreamDefaultController

	// readable holds the [ReadableStream] instance controlled by this object.
	readable *ReadableStream

	// writable holds the [WritableStream] instance controlled by this object.
	writable *WritableStream

	// readableObj and writableObj hold the JavaScript objects exposed through the `readable`
	// and `writable` getters.
	readableObj *sobek.Object
	writableObj *sobek.Object

	// txDepth tracks the nesting depth of the current settlement transaction (see
	// withTransaction).
	txDepth int

	// txQueue holds promise settlements deferred until the current transaction completes.
	txQueue []func()

	runtime *sobek.Runtime
	vu      modules.VU
}

// withTransaction runs fn, and then, once the outermost transaction completes, flushes any
// promise settlements that were deferred via settle() during fn.
//
// As with the WritableStream, this is necessary because, in the k6/Sobek event loop, resolving a
// promise from Go runs its reactions synchronously. The Streams specification instead assumes
// that settling a promise merely schedules its reactions, which run only after all synchronous
// state changes have completed. Deferring the settlements until the end of the transaction
// reproduces that behaviour, so that reactions (such as the readable side's pull reacting to a
// backpressure change) observe fully-updated stream state.
func (stream *TransformStream) withTransaction(fn func()) {
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
func (stream *TransformStream) drainSettlements() {
	// Keep the depth elevated while draining, so that any settlements scheduled by reactions are
	// appended to the queue and processed in order, rather than recursively.
	stream.txDepth++
	for len(stream.txQueue) > 0 {
		settle := stream.txQueue[0]
		stream.txQueue = stream.txQueue[1:]
		settle()
	}
	stream.txDepth--
}

// settle schedules a promise settlement. If a transaction is active, the settlement is deferred
// until the transaction completes; otherwise, it runs immediately.
func (stream *TransformStream) settle(fn func()) {
	if stream.txDepth == 0 {
		fn()
		return
	}
	stream.txQueue = append(stream.txQueue, fn)
}

// transformStreamGoRefKey is the name of a hidden, non-enumerable property that holds a
// reference to the Go [TransformStream] on the stream's JavaScript object. It allows retrieving
// the Go value back from the JavaScript object in the `readable` and `writable` brand-check
// getters.
const transformStreamGoRefKey = "__k6TransformStream__"

// transformStreamFromValue retrieves the Go [TransformStream] associated with a JavaScript
// value, or nil if the value is not a TransformStream object.
func transformStreamFromValue(rt *sobek.Runtime, value sobek.Value) *TransformStream {
	if value == nil || common.IsNullish(value) || !isObject(value) {
		return nil
	}

	ref := value.ToObject(rt).Get(transformStreamGoRefKey)
	if ref == nil {
		return nil
	}

	stream, _ := ref.Export().(*TransformStream)
	return stream
}

// readableController returns the [ReadableStreamDefaultController] of the transform stream's
// readable side. The transform stream always sets up a default controller for its readable
// side, so this type assertion is safe.
func (stream *TransformStream) readableController() *ReadableStreamDefaultController {
	rc, ok := stream.readable.controller.(*ReadableStreamDefaultController)
	if !ok {
		common.Throw(stream.runtime, newError(AssertionError, "readable controller is not a default controller"))
	}
	return rc
}

// initialize implements the [specification]'s InitializeTransformStream abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#initialize-transform-stream
func (stream *TransformStream) initialize(
	startPromise *promiseWrapper,
	writableHighWaterMark float64,
	writableSizeAlgorithm SizeAlgorithm,
	readableHighWaterMark float64,
	readableSizeAlgorithm SizeAlgorithm,
) {
	rt := stream.runtime

	// 1. Let startAlgorithm be an algorithm that returns startPromise.
	startAlgorithmSink := func(*sobek.Object) sobek.Value { return rt.ToValue(startPromise.promise) }
	startAlgorithmSource := func(*sobek.Object) sobek.Value { return rt.ToValue(startPromise.promise) }

	// 2. Let writeAlgorithm be an algorithm returning TransformStreamDefaultSinkWriteAlgorithm.
	writeAlgorithm := func(chunk sobek.Value, _ *sobek.Object) *sobek.Promise {
		return stream.sinkWriteAlgorithm(chunk)
	}

	// 3. Let abortAlgorithm be an algorithm returning TransformStreamDefaultSinkAbortAlgorithm.
	abortAlgorithm := func(reason any) *sobek.Promise {
		return stream.sinkAbortAlgorithm(reason)
	}

	// 4. Let closeAlgorithm be an algorithm returning TransformStreamDefaultSinkCloseAlgorithm.
	closeAlgorithm := func() *sobek.Promise {
		return stream.sinkCloseAlgorithm()
	}

	// 5. Set stream.[[writable]] to ! CreateWritableStream(...).
	stream.writable = createWritableStream(
		stream.vu,
		startAlgorithmSink,
		writeAlgorithm,
		closeAlgorithm,
		abortAlgorithm,
		writableHighWaterMark,
		writableSizeAlgorithm,
	)

	// 6. Let pullAlgorithm be an algorithm returning TransformStreamDefaultSourcePullAlgorithm.
	pullAlgorithm := func(*sobek.Object) *sobek.Promise {
		return stream.sourcePullAlgorithm()
	}

	// 7. Let cancelAlgorithm be an algorithm returning TransformStreamDefaultSourceCancelAlgorithm.
	cancelAlgorithm := func(reason any) sobek.Value {
		return rt.ToValue(stream.sourceCancelAlgorithm(reason))
	}

	// 8. Set stream.[[readable]] to ! CreateReadableStream(...).
	stream.readable = createReadableStream(
		stream.vu,
		startAlgorithmSource,
		pullAlgorithm,
		cancelAlgorithm,
		readableHighWaterMark,
		readableSizeAlgorithm,
	)

	// 9. Set stream.[[backpressure]] and stream.[[backpressureChangePromise]] to undefined.
	// We represent backpressure with a plain boolean initialized to false, and let step 10
	// initialize it to true (see the specification's note on this).
	stream.backpressure = false
	stream.backpressureChangePromise = nil

	// 10. Perform ! TransformStreamSetBackpressure(stream, true).
	stream.setBackpressure(true)

	// 11. Set stream.[[controller]] to undefined.
	stream.controller = nil
}

// resolveStartPromiseAsync resolves the transform stream's startPromise on a microtask, rather
// than synchronously.
//
// The specification resolves startPromise synchronously in the constructor, but relies on the
// readable and writable sides' start reactions running as microtasks afterwards. In the k6/Sobek
// event loop, resolving a promise from Go runs its reactions synchronously, which would run those
// start reactions during construction and diverge from the specification's observable ordering
// (e.g. code that runs synchronously right after the constructor would see the sides as already
// started). Deferring the resolution to a microtask restores the specification's behaviour.
func (stream *TransformStream) resolveStartPromiseAsync(startPromise *promiseWrapper, res sobek.Value) {
	resolved := newResolvedPromise(stream.vu, sobek.Undefined())
	if _, err := promiseThen(stream.runtime, resolved, func(sobek.Value) {
		// Resolving startPromise runs the readable and writable sides' start reactions
		// synchronously (in the k6/Sobek event loop). Wrapping the resolution in a transaction
		// ensures that backpressure settlements those reactions trigger are deferred until the
		// whole start cascade has completed (and, in particular, until the readable side has
		// registered its pull reaction), rather than running re-entrantly.
		stream.withTransaction(func() {
			startPromise.resolveWith(res)
		})
	}, nil); err != nil {
		common.Throw(stream.runtime, err)
	}
}

// error implements the [specification]'s TransformStreamError abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#transform-stream-error
func (stream *TransformStream) error(e any) {
	// 1. Perform ! ReadableStreamDefaultControllerError(stream.[[readable]].[[controller]], e).
	stream.readableController().error(e)

	// 2. Perform ! TransformStreamErrorWritableAndUnblockWrite(stream, e).
	stream.errorWritableAndUnblockWrite(e)
}

// errorWritableAndUnblockWrite implements the [specification]'s
// TransformStreamErrorWritableAndUnblockWrite abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#transform-stream-error-writable-and-unblock-write
func (stream *TransformStream) errorWritableAndUnblockWrite(e any) {
	// 1. Perform ! TransformStreamDefaultControllerClearAlgorithms(stream.[[controller]]).
	stream.controller.clearAlgorithms()

	// 2. Perform ! WritableStreamDefaultControllerErrorIfNeeded(stream.[[writable]].[[controller]], e).
	stream.writable.controller.errorIfNeeded(e)

	// 3. Perform ! TransformStreamUnblockWrite(stream).
	stream.unblockWrite()
}

// setBackpressure implements the [specification]'s TransformStreamSetBackpressure abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#transform-stream-set-backpressure
func (stream *TransformStream) setBackpressure(backpressure bool) {
	// 1. Assert: stream.[[backpressure]] is not backpressure.
	if stream.backpressure == backpressure {
		common.Throw(stream.runtime, newError(AssertionError, "backpressure is already set to the given value"))
	}

	previous := stream.backpressureChangePromise

	// 3. Set stream.[[backpressureChangePromise]] to a new promise.
	stream.backpressureChangePromise = newPromiseWrapper(stream.runtime)

	// 4. Set stream.[[backpressure]] to backpressure.
	stream.backpressure = backpressure

	// 2. If stream.[[backpressureChangePromise]] is not undefined, resolve it with undefined.
	//
	// The specification resolves the previous promise before creating the new one and updating
	// [[backpressure]], relying on the promise's reactions running as microtasks (i.e. after all
	// synchronous steps complete). In the k6/Sobek event loop, resolving a promise from Go runs
	// its reactions synchronously, which would re-enter this algorithm before the caller (e.g.
	// TransformStreamDefaultSourcePullAlgorithm) has observed the freshly created promise. We
	// therefore defer the resolution through the transaction mechanism (see withTransaction), so
	// the state is fully updated and the caller has returned before any reaction (such as a
	// pending transform) runs.
	if previous != nil {
		stream.settle(func() { previous.resolveWith(sobek.Undefined()) })
	}
}

// unblockWrite implements the [specification]'s TransformStreamUnblockWrite abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#transform-stream-unblock-write
func (stream *TransformStream) unblockWrite() {
	// 1. If stream.[[backpressure]] is true, perform ! TransformStreamSetBackpressure(stream, false).
	if stream.backpressure {
		stream.setBackpressure(false)
	}
}

// sinkWriteAlgorithm implements the [specification]'s TransformStreamDefaultSinkWriteAlgorithm
// abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#transform-stream-default-sink-write-algorithm
func (stream *TransformStream) sinkWriteAlgorithm(chunk sobek.Value) *sobek.Promise {
	rt := stream.runtime

	// 1. Assert: stream.[[writable]].[[state]] is "writable".
	if stream.writable.state != WritableStreamStateWritable {
		common.Throw(rt, newError(AssertionError, "writable stream is not writable"))
	}

	// 2. Let controller be stream.[[controller]].
	controller := stream.controller

	// 3. If stream.[[backpressure]] is true,
	if stream.backpressure {
		// 3.1. Let backpressureChangePromise be stream.[[backpressureChangePromise]].
		backpressureChangePromise := stream.backpressureChangePromise

		// 3.2. Assert: backpressureChangePromise is not undefined.
		if backpressureChangePromise == nil {
			common.Throw(rt, newError(AssertionError, "backpressureChangePromise is undefined"))
		}

		// 3.3. Return the result of reacting to backpressureChangePromise with fulfillment steps.
		p, err := promiseThenReturn(rt, backpressureChangePromise.promise,
			func(sobek.Value) sobek.Value {
				// 3.3.1. Let writable be stream.[[writable]].
				writable := stream.writable
				// 3.3.2. Let state be writable.[[state]].
				state := writable.state
				// 3.3.3. If state is "erroring", throw writable.[[storedError]].
				if state == WritableStreamStateErroring {
					throw(rt, throwableValue(writable.storedError))
				}
				// 3.3.4. Assert: state is "writable".
				if state != WritableStreamStateWritable {
					common.Throw(rt, newError(AssertionError, "writable stream is not writable"))
				}
				// 3.3.5. Return ! TransformStreamDefaultControllerPerformTransform(controller, chunk).
				return rt.ToValue(controller.performTransform(chunk))
			},
			nil,
		)
		if err != nil {
			common.Throw(rt, err)
		}
		return p
	}

	// 4. Return ! TransformStreamDefaultControllerPerformTransform(controller, chunk).
	return controller.performTransform(chunk)
}

// sinkAbortAlgorithm implements the [specification]'s TransformStreamDefaultSinkAbortAlgorithm
// abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#transform-stream-default-sink-abort-algorithm
func (stream *TransformStream) sinkAbortAlgorithm(reason any) *sobek.Promise {
	rt := stream.runtime

	// 1. Let controller be stream.[[controller]].
	controller := stream.controller

	// 2. If controller.[[finishPromise]] is not undefined, return controller.[[finishPromise]].
	if controller.finishPromise != nil {
		return controller.finishPromise.promise
	}

	// 3. Let readable be stream.[[readable]].
	readable := stream.readable

	// 4. Let controller.[[finishPromise]] be a new promise.
	controller.finishPromise = newPromiseWrapper(rt)

	// 5. Let cancelPromise be the result of performing controller.[[cancelAlgorithm]], passing reason.
	cancelPromise := controller.cancelAlgorithm(reason)

	// 6. Perform ! TransformStreamDefaultControllerClearAlgorithms(controller).
	controller.clearAlgorithms()

	finishPromise := controller.finishPromise
	readableController := stream.readableController()

	// 7. React to cancelPromise.
	_, err := promiseThen(rt, cancelPromise,
		func(sobek.Value) {
			// 7.1. If cancelPromise was fulfilled, then:
			if readable.state == ReadableStreamStateErrored {
				// 7.1.1. If readable.[[state]] is "errored", reject finishPromise with readable.[[storedError]].
				finishPromise.rejectWith(throwableValue(readable.storedError))
			} else {
				// 7.1.2. Otherwise:
				// 7.1.2.1. Perform ! ReadableStreamDefaultControllerError(readable.[[controller]], reason).
				readableController.error(reason)
				// 7.1.2.2. Resolve finishPromise with undefined.
				finishPromise.resolveWith(sobek.Undefined())
			}
		},
		func(r sobek.Value) {
			// 7.2. If cancelPromise was rejected with reason r, then:
			// 7.2.1. Perform ! ReadableStreamDefaultControllerError(readable.[[controller]], r).
			readableController.error(r)
			// 7.2.2. Reject finishPromise with r.
			finishPromise.rejectWith(r)
		},
	)
	if err != nil {
		common.Throw(rt, err)
	}

	// 8. Return controller.[[finishPromise]].
	return controller.finishPromise.promise
}

// sinkCloseAlgorithm implements the [specification]'s TransformStreamDefaultSinkCloseAlgorithm
// abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#transform-stream-default-sink-close-algorithm
func (stream *TransformStream) sinkCloseAlgorithm() *sobek.Promise {
	rt := stream.runtime

	// 1. Let controller be stream.[[controller]].
	controller := stream.controller

	// 2. If controller.[[finishPromise]] is not undefined, return controller.[[finishPromise]].
	if controller.finishPromise != nil {
		return controller.finishPromise.promise
	}

	// 3. Let readable be stream.[[readable]].
	readable := stream.readable

	// 4. Let controller.[[finishPromise]] be a new promise.
	controller.finishPromise = newPromiseWrapper(rt)

	// 5. Let flushPromise be the result of performing controller.[[flushAlgorithm]].
	flushPromise := controller.flushAlgorithm()

	// 6. Perform ! TransformStreamDefaultControllerClearAlgorithms(controller).
	controller.clearAlgorithms()

	finishPromise := controller.finishPromise
	readableController := stream.readableController()

	// 7. React to flushPromise.
	_, err := promiseThen(rt, flushPromise,
		func(sobek.Value) {
			// 7.1. If flushPromise was fulfilled, then:
			if readable.state == ReadableStreamStateErrored {
				// 7.1.1. If readable.[[state]] is "errored", reject finishPromise with readable.[[storedError]].
				finishPromise.rejectWith(throwableValue(readable.storedError))
			} else {
				// 7.1.2. Otherwise:
				// 7.1.2.1. Perform ! ReadableStreamDefaultControllerClose(readable.[[controller]]).
				readableController.close()
				// 7.1.2.2. Resolve finishPromise with undefined.
				finishPromise.resolveWith(sobek.Undefined())
			}
		},
		func(r sobek.Value) {
			// 7.2. If flushPromise was rejected with reason r, then:
			// 7.2.1. Perform ! ReadableStreamDefaultControllerError(readable.[[controller]], r).
			readableController.error(r)
			// 7.2.2. Reject finishPromise with r.
			finishPromise.rejectWith(r)
		},
	)
	if err != nil {
		common.Throw(rt, err)
	}

	// 8. Return controller.[[finishPromise]].
	return controller.finishPromise.promise
}

// sourcePullAlgorithm implements the [specification]'s TransformStreamDefaultSourcePullAlgorithm
// abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#transform-stream-default-source-pull
func (stream *TransformStream) sourcePullAlgorithm() *sobek.Promise {
	rt := stream.runtime

	// 1. Assert: stream.[[backpressure]] is true.
	if !stream.backpressure {
		common.Throw(rt, newError(AssertionError, "backpressure is not true"))
	}

	// 2. Assert: stream.[[backpressureChangePromise]] is not undefined.
	if stream.backpressureChangePromise == nil {
		common.Throw(rt, newError(AssertionError, "backpressureChangePromise is undefined"))
	}

	// 3. Perform ! TransformStreamSetBackpressure(stream, false).
	stream.setBackpressure(false)

	// 4. Return stream.[[backpressureChangePromise]].
	return stream.backpressureChangePromise.promise
}

// sourceCancelAlgorithm implements the [specification]'s TransformStreamDefaultSourceCancelAlgorithm
// abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#transform-stream-default-source-cancel
func (stream *TransformStream) sourceCancelAlgorithm(reason any) *sobek.Promise {
	rt := stream.runtime

	// 1. Let controller be stream.[[controller]].
	controller := stream.controller

	// 2. If controller.[[finishPromise]] is not undefined, return controller.[[finishPromise]].
	if controller.finishPromise != nil {
		return controller.finishPromise.promise
	}

	// 3. Let writable be stream.[[writable]].
	writable := stream.writable

	// 4. Let controller.[[finishPromise]] be a new promise.
	controller.finishPromise = newPromiseWrapper(rt)

	// 5. Let cancelPromise be the result of performing controller.[[cancelAlgorithm]], passing reason.
	cancelPromise := controller.cancelAlgorithm(reason)

	// 6. Perform ! TransformStreamDefaultControllerClearAlgorithms(controller).
	controller.clearAlgorithms()

	finishPromise := controller.finishPromise

	// 7. React to cancelPromise.
	_, err := promiseThen(rt, cancelPromise,
		func(sobek.Value) {
			// 7.1. If cancelPromise was fulfilled, then:
			stream.writable.withTransaction(func() {
				if writable.state == WritableStreamStateErrored {
					// 7.1.1. If writable.[[state]] is "errored", reject finishPromise with writable.[[storedError]].
					finishPromise.rejectWith(throwableValue(writable.storedError))
				} else {
					// 7.1.2. Otherwise:
					// 7.1.2.1. Perform ! WritableStreamDefaultControllerErrorIfNeeded(writable.[[controller]], reason).
					writable.controller.errorIfNeeded(reason)
					// 7.1.2.2. Perform ! TransformStreamUnblockWrite(stream).
					stream.unblockWrite()
					// 7.1.2.3. Resolve finishPromise with undefined.
					finishPromise.resolveWith(sobek.Undefined())
				}
			})
		},
		func(r sobek.Value) {
			// 7.2. If cancelPromise was rejected with reason r, then:
			stream.writable.withTransaction(func() {
				// 7.2.1. Perform ! WritableStreamDefaultControllerErrorIfNeeded(writable.[[controller]], r).
				writable.controller.errorIfNeeded(r)
				// 7.2.2. Perform ! TransformStreamUnblockWrite(stream).
				stream.unblockWrite()
				// 7.2.3. Reject finishPromise with r.
				finishPromise.rejectWith(r)
			})
		},
	)
	if err != nil {
		common.Throw(rt, err)
	}

	// 8. Return controller.[[finishPromise]].
	return controller.finishPromise.promise
}

// setupDefaultController implements the [specification]'s SetUpTransformStreamDefaultController
// abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#set-up-transform-stream-default-controller
func (stream *TransformStream) setupDefaultController(
	controller *TransformStreamDefaultController,
	transformAlgorithm TransformerTransformCallback,
	flushAlgorithm TransformerFlushCallback,
	cancelAlgorithm TransformerCancelCallback,
) {
	// 1. Assert: stream implements TransformStream.
	// 2. Assert: stream.[[controller]] is undefined.
	if stream.controller != nil {
		common.Throw(stream.runtime, newError(AssertionError, "stream already has a controller"))
	}

	// 3. Set controller.[[stream]] to stream.
	controller.stream = stream

	// 4. Set stream.[[controller]] to controller.
	stream.controller = controller

	// 5. Set controller.[[transformAlgorithm]] to transformAlgorithm.
	controller.transformAlgorithm = transformAlgorithm

	// 6. Set controller.[[flushAlgorithm]] to flushAlgorithm.
	controller.flushAlgorithm = flushAlgorithm

	// 7. Set controller.[[cancelAlgorithm]] to cancelAlgorithm.
	controller.cancelAlgorithm = cancelAlgorithm
}

// setupDefaultControllerFromTransformer implements the [specification]'s
// SetUpTransformStreamDefaultControllerFromTransformer abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#set-up-transform-stream-default-controller-from-transformer
func (stream *TransformStream) setupDefaultControllerFromTransformer(
	transformerObj *sobek.Object,
	transformerDict Transformer,
) {
	// 1. Let controller be a new TransformStreamDefaultController.
	controller := &TransformStreamDefaultController{stream: stream}

	// The controller object is created once and reused across start()/transform()/flush().
	controllerObj, err := controller.toObject()
	if err != nil {
		common.Throw(stream.runtime, newError(RuntimeError, err.Error()))
	}
	// 2-7. Derive the transform, flush and cancel algorithms from the transformer, falling back
	// to their default (identity/no-op) implementations when the corresponding method is absent.
	transformAlgorithm := stream.transformAlgorithm(controller, transformerObj, transformerDict)
	flushAlgorithm := stream.flushAlgorithm(controllerObj, transformerObj, transformerDict)
	cancelAlgorithm := stream.cancelAlgorithm(transformerObj, transformerDict)

	// 8. Perform ! SetUpTransformStreamDefaultController(stream, controller, ...).
	stream.setupDefaultController(controller, transformAlgorithm, flushAlgorithm, cancelAlgorithm)
}

// callbackResultToPromise converts the result of invoking a transformer callback into a promise:
// a Go error becomes a rejected promise, a returned promise is passed through, and any other
// value becomes a promise resolved with it.
func (stream *TransformStream) callbackResultToPromise(v sobek.Value, err error) *sobek.Promise {
	if err != nil {
		return newRejectedPromise(stream.vu, exceptionValue(err))
	}
	if p, ok := v.Export().(*sobek.Promise); ok {
		return p
	}
	return newResolvedPromise(stream.vu, v)
}

// transformAlgorithm returns the controller's transform algorithm, as derived from the
// transformer (steps 2 and 5 of SetUpTransformStreamDefaultControllerFromTransformer).
func (stream *TransformStream) transformAlgorithm(
	controller *TransformStreamDefaultController,
	transformerObj *sobek.Object,
	transformerDict Transformer,
) TransformerTransformCallback {
	// 5. If transformerDict["transform"] exists, use it.
	if transformerDict.Transform != nil && !sobek.IsUndefined(transformerDict.Transform) {
		transformFn, ok := sobek.AssertFunction(transformerDict.Transform)
		if !ok {
			throw(stream.runtime, newTypeError(stream.runtime, "transformer.transform must be a function"))
		}
		return func(chunk sobek.Value) *sobek.Promise {
			v, e := transformFn(transformerObj, chunk, controller.object)
			return stream.callbackResultToPromise(v, e)
		}
	}

	// 2. Otherwise, use the default (identity) algorithm.
	return func(chunk sobek.Value) *sobek.Promise {
		// 2.1. Let result be TransformStreamDefaultControllerEnqueue(controller, chunk).
		if e := stream.runtime.Try(func() { controller.enqueue(chunk) }); e != nil {
			// 2.2. If result is an abrupt completion, return a promise rejected with result.[[Value]].
			return newRejectedPromise(stream.vu, e.Value())
		}
		// 2.3. Otherwise, return a promise resolved with undefined.
		return newResolvedPromise(stream.vu, sobek.Undefined())
	}
}

// flushAlgorithm returns the controller's flush algorithm, as derived from the transformer
// (steps 3 and 6 of SetUpTransformStreamDefaultControllerFromTransformer).
func (stream *TransformStream) flushAlgorithm(
	controllerObj *sobek.Object,
	transformerObj *sobek.Object,
	transformerDict Transformer,
) TransformerFlushCallback {
	// 6. If transformerDict["flush"] exists, use it.
	if transformerDict.Flush != nil && !sobek.IsUndefined(transformerDict.Flush) {
		flushFn, ok := sobek.AssertFunction(transformerDict.Flush)
		if !ok {
			throw(stream.runtime, newTypeError(stream.runtime, "transformer.flush must be a function"))
		}
		return func() *sobek.Promise {
			v, e := flushFn(transformerObj, controllerObj)
			return stream.callbackResultToPromise(v, e)
		}
	}

	// 3. Otherwise, return a promise resolved with undefined.
	return func() *sobek.Promise {
		return newResolvedPromise(stream.vu, sobek.Undefined())
	}
}

// cancelAlgorithm returns the controller's cancel algorithm, as derived from the transformer
// (steps 4 and 7 of SetUpTransformStreamDefaultControllerFromTransformer).
func (stream *TransformStream) cancelAlgorithm(
	transformerObj *sobek.Object,
	transformerDict Transformer,
) TransformerCancelCallback {
	// 7. If transformerDict["cancel"] exists, use it.
	if transformerDict.Cancel != nil && !sobek.IsUndefined(transformerDict.Cancel) {
		cancelFn, ok := sobek.AssertFunction(transformerDict.Cancel)
		if !ok {
			throw(stream.runtime, newTypeError(stream.runtime, "transformer.cancel must be a function"))
		}
		return func(reason any) *sobek.Promise {
			v, e := cancelFn(transformerObj, stream.runtime.ToValue(reason))
			return stream.callbackResultToPromise(v, e)
		}
	}

	// 4. Otherwise, return a promise resolved with undefined.
	return func(any) *sobek.Promise {
		return newResolvedPromise(stream.vu, sobek.Undefined())
	}
}

func installTransformStreamPrototype(rt *sobek.Runtime, proto *sobek.Object) error {
	if !hasOwnProperty(proto, "readable") {
		err := proto.DefineAccessorProperty(
			"readable",
			rt.ToValue(func(fc sobek.FunctionCall) sobek.Value {
				stream := transformStreamFromValue(rt, fc.This)
				if stream == nil {
					return sobek.Undefined()
				}
				return stream.readableObj
			}),
			nil,
			sobek.FLAG_TRUE,
			sobek.FLAG_TRUE,
		)
		if err != nil {
			return err
		}
	}

	if hasOwnProperty(proto, "writable") {
		return nil
	}

	return proto.DefineAccessorProperty(
		"writable",
		rt.ToValue(func(fc sobek.FunctionCall) sobek.Value {
			stream := transformStreamFromValue(rt, fc.This)
			if stream == nil {
				return sobek.Undefined()
			}
			return stream.writableObj
		}),
		nil,
		sobek.FLAG_TRUE,
		sobek.FLAG_TRUE,
	)
}

// toObject builds the transform stream's JavaScript object.
//
// It is built as a plain object rather than a reflect-wrapped Go value, because the latter is
// not extensible: the Web Platform Tests' recordingTransformStream helper assigns extra
// properties (such as `controller` and `events`) to the stream. The given proto is used as the
// object's prototype.
func (stream *TransformStream) toObject(proto *sobek.Object) *sobek.Object {
	rt := stream.runtime
	obj := rt.NewObject()

	// We keep a hidden, non-enumerable reference to the Go stream on the object, so that the
	// prototype's `readable` and `writable` getters can retrieve it back.
	if err := obj.DefineDataProperty(
		transformStreamGoRefKey, rt.ToValue(stream), sobek.FLAG_FALSE, sobek.FLAG_FALSE, sobek.FLAG_FALSE,
	); err != nil {
		common.Throw(rt, newError(RuntimeError, err.Error()))
	}

	if err := obj.SetPrototype(proto); err != nil {
		common.Throw(rt, newError(RuntimeError, err.Error()))
	}

	return obj
}
