package streams

import (
	"github.com/grafana/sobek"
	"go.k6.io/k6/v2/js/common"
)

// TransformStreamDefaultController is the default controller for a TransformStream. It has
// the ability to enqueue chunks to the readable side, or to terminate or error the stream.
//
// For more details, see the [specification].
//
// [specification]: https://streams.spec.whatwg.org/#ts-default-controller-class
type TransformStreamDefaultController struct {
	// cancelAlgorithm is a promise-returning algorithm, taking one argument (the reason for
	// cancellation), which communicates a requested cancellation to the transformer.
	cancelAlgorithm TransformerCancelCallback

	// finishPromise is a promise which resolves on completion of either the cancelAlgorithm
	// or the flushAlgorithm. If this field is nil, then neither of those algorithms have
	// been invoked yet.
	finishPromise *promiseWrapper

	// flushAlgorithm is a promise-returning algorithm which communicates a requested close to
	// the transformer.
	flushAlgorithm TransformerFlushCallback

	// object is the controller's JavaScript object, created once and reused across the
	// transformer's start(), transform() and flush() invocations so that its identity is
	// stable (as observed by user code that stores the controller).
	object *sobek.Object

	// stream is the transform stream that this controller controls.
	stream *TransformStream

	// transformAlgorithm is a promise-returning algorithm, taking one argument (the chunk to
	// transform), which requests the transformer perform its transformation.
	transformAlgorithm TransformerTransformCallback
}

// NewTransformStreamDefaultControllerObject creates a new [sobek.Object] from a
// [TransformStreamDefaultController] instance.
func NewTransformStreamDefaultControllerObject(
	controller *TransformStreamDefaultController,
) (*sobek.Object, error) {
	rt := controller.stream.runtime
	obj := rt.NewObject()
	objName := "TransformStreamDefaultController"

	// The controller is not constructable: invoking its constructor must throw a TypeError.
	// Exposing a plain Go function (which is not a constructor) achieves this, as calling it
	// with `new` throws a TypeError.
	if err := setReadOnlyPropertyOf(obj, objName, "constructor", rt.ToValue(func() sobek.Value {
		throw(rt, newTypeError(rt, "TransformStreamDefaultController is not constructable"))
		return sobek.Undefined()
	})); err != nil {
		return nil, err
	}

	// desiredSize getter
	err := obj.DefineAccessorProperty("desiredSize", rt.ToValue(func() sobek.Value {
		return controller.DesiredSize()
	}), nil, sobek.FLAG_FALSE, sobek.FLAG_TRUE)
	if err != nil {
		return nil, err
	}

	if err := setReadOnlyPropertyOf(
		obj,
		objName,
		"enqueue",
		rt.ToValue(controller.Enqueue),
	); err != nil {
		return nil, err
	}

	if err := setReadOnlyPropertyOf(obj, objName, "error", rt.ToValue(controller.Error)); err != nil {
		return nil, err
	}

	if err := setReadOnlyPropertyOf(
		obj,
		objName,
		"terminate",
		rt.ToValue(controller.Terminate),
	); err != nil {
		return nil, err
	}

	return rt.CreateObject(obj), nil
}

// DesiredSize returns the desired size to fill the readable side's internal queue.
//
// It implements the TransformStreamDefaultController.desiredSize [specification] getter.
//
// [specification]: https://streams.spec.whatwg.org/#ts-default-controller-desired-size
func (controller *TransformStreamDefaultController) DesiredSize() sobek.Value {
	rt := controller.stream.runtime

	// 1. Let readableController be this.[[stream]].[[readable]].[[controller]].
	readableController := controller.readableController()

	// 2. Return ! ReadableStreamDefaultControllerGetDesiredSize(readableController).
	desiredSize := readableController.getDesiredSize()
	if !desiredSize.Valid {
		return sobek.Null()
	}
	return rt.ToValue(desiredSize.Float64)
}

// Enqueue enqueues the given chunk in the readable side of the controlled transform stream.
//
// It implements the TransformStreamDefaultController.enqueue(chunk) [specification] algorithm.
//
// [specification]: https://streams.spec.whatwg.org/#ts-default-controller-enqueue
func (controller *TransformStreamDefaultController) Enqueue(chunk sobek.Value) {
	if chunk == nil {
		chunk = sobek.Undefined()
	}

	// 1. Perform ? TransformStreamDefaultControllerEnqueue(this, chunk).
	controller.enqueue(chunk)
}

// Error errors both the readable and writable sides of the controlled transform stream.
//
// It implements the TransformStreamDefaultController.error(e) [specification] algorithm.
//
// [specification]: https://streams.spec.whatwg.org/#ts-default-controller-error
func (controller *TransformStreamDefaultController) Error(e sobek.Value) {
	if e == nil {
		e = sobek.Undefined()
	}

	// 1. Perform ? TransformStreamDefaultControllerError(this, e).
	controller.stream.writable.withTransaction(func() {
		controller.error(e)
	})
}

// Terminate closes the readable side and errors the writable side of the controlled transform
// stream.
//
// It implements the TransformStreamDefaultController.terminate() [specification] algorithm.
//
// [specification]: https://streams.spec.whatwg.org/#ts-default-controller-terminate
func (controller *TransformStreamDefaultController) Terminate() {
	// 1. Perform ? TransformStreamDefaultControllerTerminate(this).
	controller.stream.writable.withTransaction(func() {
		controller.terminate()
	})
}

// readableController returns the [ReadableStreamDefaultController] of the transform stream's
// readable side.
func (controller *TransformStreamDefaultController) readableController() *ReadableStreamDefaultController {
	return controller.stream.readableController()
}

// clearAlgorithms implements the [specification]'s TransformStreamDefaultControllerClearAlgorithms
// abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#transform-stream-default-controller-clear-algorithms
func (controller *TransformStreamDefaultController) clearAlgorithms() {
	// 1. Set controller.[[transformAlgorithm]] to undefined.
	controller.transformAlgorithm = nil

	// 2. Set controller.[[flushAlgorithm]] to undefined.
	controller.flushAlgorithm = nil

	// 3. Set controller.[[cancelAlgorithm]] to undefined.
	controller.cancelAlgorithm = nil
}

// enqueue implements the [specification]'s TransformStreamDefaultControllerEnqueue abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#transform-stream-default-controller-enqueue
func (controller *TransformStreamDefaultController) enqueue(chunk sobek.Value) {
	// 1. Let stream be controller.[[stream]].
	stream := controller.stream
	rt := stream.runtime

	// 2. Let readableController be stream.[[readable]].[[controller]].
	readableController := controller.readableController()

	// 3. If ! ReadableStreamDefaultControllerCanCloseOrEnqueue(readableController) is false,
	// throw a TypeError exception.
	if !readableController.canCloseOrEnqueue() {
		throw(rt, newTypeError(rt, "readable side is not in a state that permits enqueuing"))
	}

	// 4. Let enqueueResult be ReadableStreamDefaultControllerEnqueue(readableController, chunk).
	err := readableController.enqueue(chunk)
	// 5. If enqueueResult is an abrupt completion,
	if err != nil {
		// 5.1. Perform ! TransformStreamErrorWritableAndUnblockWrite(stream, enqueueResult.[[Value]]).
		stream.errorWritableAndUnblockWrite(exceptionValue(err))

		// 5.2. Throw stream.[[readable]].[[storedError]].
		throw(rt, throwableValue(stream.readable.storedError))
	}

	// 6. Let backpressure be ! ReadableStreamDefaultControllerHasBackpressure(readableController).
	backpressure := readableController.hasBackpressure()

	// 7. If backpressure is not stream.[[backpressure]],
	if stream.backpressure != backpressure {
		// 7.1. Assert: backpressure is true.
		if !backpressure {
			common.Throw(rt, newError(AssertionError, "backpressure is not true"))
		}

		// 7.2. Perform ! TransformStreamSetBackpressure(stream, true).
		stream.setBackpressure(true)
	}
}

// error implements the [specification]'s TransformStreamDefaultControllerError abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#transform-stream-default-controller-error
func (controller *TransformStreamDefaultController) error(e any) {
	// 1. Perform ! TransformStreamError(controller.[[stream]], e).
	controller.stream.error(e)
}

// performTransform implements the [specification]'s
// TransformStreamDefaultControllerPerformTransform abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#transform-stream-default-controller-perform-transform
func (controller *TransformStreamDefaultController) performTransform(chunk sobek.Value) *sobek.Promise {
	rt := controller.stream.runtime

	// 1. Let transformPromise be the result of performing controller.[[transformAlgorithm]], passing chunk.
	transformPromise := controller.transformAlgorithm(chunk)

	// 2. Return the result of reacting to transformPromise with the following rejection steps given r:
	p, err := promiseThen(
		rt, transformPromise,
		func(sobek.Value) {},
		func(r sobek.Value) {
			// 2.1. Perform ! TransformStreamError(controller.[[stream]], r).
			controller.stream.error(r)
			// 2.2. Throw r.
			throw(rt, r)
		},
	)
	if err != nil {
		common.Throw(rt, err)
	}
	return p
}

// terminate implements the [specification]'s TransformStreamDefaultControllerTerminate
// abstract operation.
//
// [specification]: https://streams.spec.whatwg.org/#transform-stream-default-controller-terminate
func (controller *TransformStreamDefaultController) terminate() {
	// 1. Let stream be controller.[[stream]].
	stream := controller.stream
	rt := stream.runtime

	// 2. Let readableController be stream.[[readable]].[[controller]].
	readableController := controller.readableController()

	// 3. Perform ! ReadableStreamDefaultControllerClose(readableController).
	readableController.close()

	// 4. Let error be a TypeError exception indicating that the stream has been terminated.
	terminatedError := newTypeError(rt, "the stream has been terminated")

	// 5. Perform ! TransformStreamErrorWritableAndUnblockWrite(stream, error).
	stream.errorWritableAndUnblockWrite(terminatedError)
}

func (controller *TransformStreamDefaultController) toObject() (*sobek.Object, error) {
	if controller.object != nil {
		return controller.object, nil
	}

	object, err := NewTransformStreamDefaultControllerObject(controller)
	if err != nil {
		return nil, err
	}

	controller.object = object
	return object, nil
}
