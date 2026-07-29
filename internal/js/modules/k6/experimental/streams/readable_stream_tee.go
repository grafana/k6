package streams

import (
	"github.com/grafana/sobek"

	"go.k6.io/k6/v2/js/common"
)

// readableStreamTeeState holds the shared state of the two branches created by default tee.
// Default tee follows the faster consumer. Consequently, chunks may accumulate without bound in
// a slower branch until that branch catches up or is canceled.
type readableStreamTeeState struct {
	source *ReadableStream
	reader *ReadableStreamDefaultReader

	branch1 *ReadableStream
	branch2 *ReadableStream

	reading   bool
	readAgain bool

	canceled1 bool
	canceled2 bool
	reason1   sobek.Value
	reason2   sobek.Value

	cancelPromise *promiseWrapper
}

// tee implements the specification's ReadableStreamTee and ReadableStreamDefaultTee abstract
// operations. Readable byte streams are not constructable yet, so every supported stream uses the
// default tee algorithm.
//
// [ReadableStreamTee]: https://streams.spec.whatwg.org/#readable-stream-tee
// [ReadableStreamDefaultTee]: https://streams.spec.whatwg.org/#readable-stream-default-tee
func (stream *ReadableStream) tee() [2]*ReadableStream {
	rt := stream.runtime
	intrinsics := readableIntrinsicsForRuntime(rt)
	if intrinsics == nil {
		throw(rt, newError(RuntimeError, "ReadableStream intrinsics are not initialized"))
	}

	if _, ok := stream.controller.(*ReadableStreamDefaultController); !ok {
		throw(rt, newError(NotSupportedError, "teeing readable byte streams is not supported yet"))
	}

	state := &readableStreamTeeState{
		source:        stream,
		reader:        stream.acquireDefaultReader(),
		reason1:       sobek.Undefined(),
		reason2:       sobek.Undefined(),
		cancelPromise: newPromiseWrapper(rt),
	}

	startAlgorithm := func(*sobek.Object) sobek.Value { return sobek.Undefined() }
	pullAlgorithm := func(*sobek.Object) *sobek.Promise { return state.pull() }
	cancel1Algorithm := func(reason any) sobek.Value { return state.cancel(1, reason) }
	cancel2Algorithm := func(reason any) sobek.Value { return state.cancel(2, reason) }
	sizeAlgorithm, ok := sobek.AssertFunction(rt.ToValue(defaultSizeFunc))
	if !ok {
		throw(rt, newError(AssertionError, "default readable size algorithm is not callable"))
	}

	state.branch1 = createReadableStream(
		stream.vu, startAlgorithm, pullAlgorithm, cancel1Algorithm, 1, sizeAlgorithm,
	)
	state.branch2 = createReadableStream(
		stream.vu, startAlgorithm, pullAlgorithm, cancel2Algorithm, 1, sizeAlgorithm,
	)
	state.branch1.readerPrototype = intrinsics.readerPrototype
	state.branch2.readerPrototype = intrinsics.readerPrototype

	// A source error is observed through the reader's closed promise. The successful read path
	// queues chunk delivery in a microtask, so this reaction wins if both become observable in the
	// same turn.
	if _, err := promiseThen(rt, state.reader.getClosed().promise,
		func(sobek.Value) {},
		func(reason sobek.Value) { state.sourceErrored(reason) },
	); err != nil {
		common.Throw(rt, err)
	}

	return [2]*ReadableStream{state.branch1, state.branch2}
}

// pull is the shared pull algorithm of both branches. Only one read from the original stream can
// be active; a concurrent branch pull records that another read is needed once it completes.
func (tee *readableStreamTeeState) pull() *sobek.Promise {
	if tee.canceled1 && tee.canceled2 {
		return newResolvedPromise(tee.source.vu, sobek.Undefined())
	}
	if tee.reading {
		tee.readAgain = true
		return newResolvedPromise(tee.source.vu, sobek.Undefined())
	}

	tee.reading = true
	tee.reader.read(ReadRequest{
		chunkSteps: func(chunk any) {
			chunkValue := tee.source.runtime.ToValue(chunk)
			queueStreamMicrotask(tee.source.runtime, func() { tee.distributeChunk(chunkValue) })
		},
		closeSteps: func() { tee.sourceClosed() },
		errorSteps: func(any) {
			// reader.closed performs the error propagation to both branches.
			tee.reading = false
			tee.readAgain = false
		},
	})

	return newResolvedPromise(tee.source.vu, sobek.Undefined())
}

func (tee *readableStreamTeeState) distributeChunk(chunk sobek.Value) {
	tee.readAgain = false

	// Default tee deliberately shares the same chunk object between both branches. Byte stream
	// teeing will need the specification's cloning behavior when byte streams are implemented.
	if !tee.canceled1 {
		if err := tee.branch1Controller().enqueue(chunk); err != nil {
			common.Throw(tee.source.runtime, err)
		}
	}
	if !tee.canceled2 {
		if err := tee.branch2Controller().enqueue(chunk); err != nil {
			common.Throw(tee.source.runtime, err)
		}
	}

	tee.reading = false
	if tee.readAgain {
		tee.pull()
	}
}

func (tee *readableStreamTeeState) sourceClosed() {
	tee.reading = false
	tee.readAgain = false
	if !tee.canceled1 {
		tee.branch1Controller().close()
	}
	if !tee.canceled2 {
		tee.branch2Controller().close()
	}
	if !tee.canceled1 || !tee.canceled2 {
		tee.resolveCancel(sobek.Undefined())
	}
}

func (tee *readableStreamTeeState) sourceErrored(reason sobek.Value) {
	tee.reading = false
	tee.readAgain = false
	if !tee.canceled1 {
		tee.branch1Controller().error(reason)
	}
	if !tee.canceled2 {
		tee.branch2Controller().error(reason)
	}
	if !tee.canceled1 || !tee.canceled2 {
		// A branch that canceled before the source errored is no longer interested in source
		// errors. Its pending cancellation therefore fulfills once the source terminates.
		tee.resolveCancel(sobek.Undefined())
	}
}

func (tee *readableStreamTeeState) cancel(branch int, reason any) sobek.Value {
	reasonValue := tee.source.runtime.ToValue(reason)
	if reasonValue == nil {
		reasonValue = sobek.Undefined()
	}

	if branch == 1 {
		tee.canceled1 = true
		tee.reason1 = reasonValue
	} else {
		tee.canceled2 = true
		tee.reason2 = reasonValue
	}

	if tee.canceled1 && tee.canceled2 {
		compositeReason := tee.source.runtime.NewArray(tee.reason1, tee.reason2)
		cancelResult := tee.source.cancel(compositeReason)
		if _, err := promiseThen(tee.source.runtime, cancelResult,
			func(sobek.Value) { tee.resolveCancel(sobek.Undefined()) },
			func(rejection sobek.Value) { tee.rejectCancel(rejection) },
		); err != nil {
			common.Throw(tee.source.runtime, err)
		}
	}

	return tee.source.runtime.ToValue(tee.cancelPromise.promise)
}

func (tee *readableStreamTeeState) resolveCancel(value sobek.Value) {
	if tee.cancelPromise.isPending() {
		tee.cancelPromise.resolveWith(value)
	}
}

func (tee *readableStreamTeeState) rejectCancel(reason sobek.Value) {
	if tee.cancelPromise.isPending() {
		tee.cancelPromise.rejectWith(reason)
	}
}

func (tee *readableStreamTeeState) branch1Controller() *ReadableStreamDefaultController {
	controller, ok := tee.branch1.controller.(*ReadableStreamDefaultController)
	if !ok {
		common.Throw(tee.source.runtime, newError(AssertionError, "tee branch 1 has no default controller"))
	}
	return controller
}

func (tee *readableStreamTeeState) branch2Controller() *ReadableStreamDefaultController {
	controller, ok := tee.branch2.controller.(*ReadableStreamDefaultController)
	if !ok {
		common.Throw(tee.source.runtime, newError(AssertionError, "tee branch 2 has no default controller"))
	}
	return controller
}
