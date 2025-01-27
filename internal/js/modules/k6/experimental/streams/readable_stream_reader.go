package streams

import (
	"github.com/grafana/sobek"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
)

// ReadableStreamReader is the interface implemented by all readable stream readers.
type ReadableStreamReader interface {
	ReadableStreamGenericReader

	// Read returns a [sobek.Promise] providing access to the next chunk in the stream's internal queue.
	Read() *sobek.Promise

	// ReleaseLock releases the reader's lock on the stream.
	ReleaseLock()
}

// ReadableStreamGenericReader defines common internal getters/setters
// and methods that are shared between ReadableStreamDefaultReader and
// ReadableStreamBYOBReader objects.
//
// It implements the [ReadableStreamReaderGeneric] mixin from the specification.
//
// Because we are in the context of Sobek, we cannot really define properties
// the same way as in the spec, so we use getters/setters instead.
//
// [ReadableStreamReaderGeneric]: https://streams.spec.whatwg.org/#readablestreamgenericreader
type ReadableStreamGenericReader interface {
	// GetStream returns the stream that owns this reader.
	GetStream() *ReadableStream

	// SetStream sets the stream that owns this reader.
	SetStream(stream *ReadableStream)

	// GetClosed returns a [sobek.Promise] that resolves when the stream is closed.
	GetClosed() (p *sobek.Promise, resolve, reject func(any) error)

	// SetClosed sets the [sobek.Promise] that resolves when the stream is closed.
	SetClosed(p *sobek.Promise, resolve, reject func(any) error)

	// Cancel returns a [sobek.Promise] that resolves when the stream is canceled.
	Cancel(reason sobek.Value) *sobek.Promise
}

// BaseReadableStreamReader is a base implement
type BaseReadableStreamReader struct {
	closedPromise            *sobek.Promise
	closedPromiseResolveFunc func(resolve any) error
	closedPromiseRejectFunc  func(reason any) error

	// stream is a [ReadableStream] instance that owns this reader
	stream *ReadableStream

	runtime *sobek.Runtime
	vu      modules.VU
}

// Ensure BaseReadableStreamReader implements the ReadableStreamGenericReader interface correctly
var _ ReadableStreamGenericReader = &BaseReadableStreamReader{}

// GetStream returns the stream that owns this reader.
func (reader *BaseReadableStreamReader) GetStream() *ReadableStream {
	return reader.stream
}

// SetStream sets the stream that owns this reader.
func (reader *BaseReadableStreamReader) SetStream(stream *ReadableStream) {
	reader.stream = stream
	reader.runtime = stream.runtime
	reader.vu = stream.vu
}

// GetClosed returns the reader's closed promise as well as its resolve and reject functions.
func (reader *BaseReadableStreamReader) GetClosed() (p *sobek.Promise, resolve, reject func(any) error) {
	return reader.closedPromise, reader.closedPromiseResolveFunc, reader.closedPromiseRejectFunc
}

// SetClosed sets the reader's closed promise as well as its resolve and reject functions.
func (reader *BaseReadableStreamReader) SetClosed(p *sobek.Promise, resolve, reject func(any) error) {
	reader.closedPromise = p
	reader.closedPromiseResolveFunc = resolve
	reader.closedPromiseRejectFunc = reject
}

// Cancel returns a [sobek.Promise] that resolves when the stream is canceled.
func (reader *BaseReadableStreamReader) Cancel(reason sobek.Value) *sobek.Promise {
	return reader.cancel(reason)
}

// cancel implements the [ReadableStreamReaderGenericCancel(reader, reason)] [specification] algorithm.
//
// [specification]: https://streams.spec.whatwg.org/#readable-stream-reader-generic-cancel
func (reader *BaseReadableStreamReader) cancel(reason sobek.Value) *sobek.Promise {
	// 1. Let stream be reader.[[stream]].
	stream := reader.stream

	// 2. Assert: stream is not undefined.
	if stream == nil {
		return newRejectedPromise(reader.vu, newTypeError(reader.runtime, "stream is undefined"))
	}

	// 3. Return ! ReadableStreamCancel(stream, reason).
	return stream.cancel(reason)
}

// release implements the [ReadableStreamReaderGenericRelease(reader)] [specification] algorithm.
//
// [specification]: https://streams.spec.whatwg.org/#readable-stream-reader-generic-release
func (reader *BaseReadableStreamReader) release() {
	// 1. Let stream be reader.[[stream]].
	stream := reader.stream

	// 2. Assert: stream is not undefined.
	if stream == nil {
		common.Throw(reader.vu.Runtime(), newError(AssertionError, "stream is undefined"))
	}

	// 3. Assert: stream.[[reader]] is reader.
	if stream.reader == nil {
		common.Throw(reader.vu.Runtime(), newError(AssertionError, "stream is undefined"))
	}

	var streamReader *BaseReadableStreamReader
	if v, ok := stream.reader.(*ReadableStreamDefaultReader); ok {
		streamReader = &v.BaseReadableStreamReader
	}

	if reader != streamReader {
		common.Throw(reader.vu.Runtime(), newError(AssertionError, "stream reader isn't reader"))
	}

	// 4. If stream.[[state]] is "readable", reject reader.[[closedPromise]] with a TypeError exception.
	if stream.state == ReadableStreamStateReadable {
		err := reader.closedPromiseRejectFunc(newTypeError(reader.runtime, "stream is readable").Err())
		if err != nil {
			panic(err)
		}
	} else { // 5. Otherwise, set reader.[[closedPromise]] to a promise rejected with a TypeError exception.
		reader.closedPromise = newRejectedPromise(stream.vu, newTypeError(reader.runtime, "stream is not readable").Err())
	}

	// 6. Set reader.[[closedPromise]].[[PromiseIsHandled]] to true.
	// FIXME: See https://github.com/dop251/goja/issues/565
	var (
		err       error
		doNothing = func(sobek.Value) {}
	)
	_, err = promiseThen(stream.vu.Runtime(), reader.closedPromise, doNothing, doNothing)
	if err != nil {
		common.Throw(stream.vu.Runtime(), newError(RuntimeError, err.Error()))
	}

	// 7. Perform ! stream.[[controller]].[[ReleaseSteps]]().
	stream.controller.releaseSteps()

	// 8. Set stream.[[reader]] to undefined.
	stream.reader = nil
	stream.Locked = false

	// 9. Set reader.[[stream]] to undefined.
	reader.stream = nil
}

// ReadRequest is a struct containing three algorithms to perform in reaction to filling the readable stream's
// internal queue or changing its state
type ReadRequest struct {
	// chunkSteps is an algorithm taking a chunk, called when a chunk is available for reading.
	chunkSteps func(chunk any)

	// closeSteps is an algorithm taking no arguments, called when no chunks are available because
	// the stream is closed.
	closeSteps func()

	// errorSteps is an algorithm taking a JavaScript value, called when no chunks are available because
	// the stream is errored.
	errorSteps func(e any)
}

// ReadableStreamReaderGenericInitialize implements the [specification] ReadableStreamReaderGenericInitialize algorithm.
//
// [specification]: https://streams.spec.whatwg.org/#readable-stream-reader-generic-initialize
func ReadableStreamReaderGenericInitialize(reader ReadableStreamGenericReader, stream *ReadableStream) {
	// 1. Set reader.[[stream]] to stream.
	reader.SetStream(stream)

	// 2. Set stream.[[reader]] to reader.
	stream.reader = reader
	stream.Locked = true

	promise, resolve, reject := stream.runtime.NewPromise()

	switch stream.state {
	// 3. If stream.[[state]] is "readable",
	case ReadableStreamStateReadable:
		// 3.1 Set reader.[[closedPromise]] to a new promise.
		// Set later, as we need to set the resolve/reject functions as well.
	// 4. Otherwise, if stream.[[state]] is "closed",
	case ReadableStreamStateClosed:
		// 4.1 Set reader.[[closedPromise]] to a promise resolved with undefined.
		err := resolve(sobek.Undefined())
		if err != nil {
			panic(err) // TODO(@mstoykov): probably better to move them out as errors
		}
	// 5. Otherwise,
	default:
		// 5.1 Assert: stream.[[state]] is "errored".
		if stream.state != ReadableStreamStateErrored {
			common.Throw(stream.vu.Runtime(), newError(AssertionError, "stream.state is not \"errored\""))
		}

		// 5.2 Set reader.[[closedPromise]] to a promise rejected with stream.[[storedError]].
		if jsErr, ok := stream.storedError.(*jsError); ok {
			err := reject(jsErr.Err())
			if err != nil {
				panic(err)
			}
		} else {
			err := reject(errToObj(stream.runtime, stream.storedError))
			if err != nil {
				panic(err)
			}
		}

		// 5.3 Set reader.[[closedPromise]].[[PromiseIsHandled]] to true.
		// See https://github.com/dop251/goja/issues/565
		var (
			err       error
			doNothing = func(sobek.Value) {}
		)
		_, err = promiseThen(stream.vu.Runtime(), promise, doNothing, doNothing)
		if err != nil {
			common.Throw(stream.vu.Runtime(), newError(RuntimeError, err.Error()))
		}
	}

	reader.SetClosed(promise, resolve, reject)
}
