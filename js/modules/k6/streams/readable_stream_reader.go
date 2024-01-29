package streams

import (
	"github.com/dop251/goja"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/promises"
)

// ReadableStreamReader is the interface implemented by all readable stream readers.
type ReadableStreamReader interface {
	ReadableStreamGenericReader

	// Read returns a [goja.Promise] providing access to the next chunk in the stream's internal queue.
	Read() *goja.Promise

	// ReleaseLock releases the reader's lock on the stream.
	ReleaseLock()
}

// ReadableStreamGenericReader defines common internal getters/setters
// and methods that are shared between ReadableStreamDefaultReader and
// ReadableStreamBYOBReader objects.
//
// It implements the [ReadableStreamReaderGeneric] mixin from the specification.
//
// Because we are in the context of Goja, we cannot really define properties
// the same way as in the spec, so we use getters/setters instead.
//
// [ReadableStreamReaderGeneric]: https://streams.spec.whatwg.org/#readablestreamgenericreader
type ReadableStreamGenericReader interface {
	// GetStream returns the stream that owns this reader.
	GetStream() *ReadableStream

	// SetStream sets the stream that owns this reader.
	SetStream(stream *ReadableStream)

	// GetClosed returns a [goja.Promise] that resolves when the stream is closed.
	GetClosed() (p *goja.Promise, resolve func(any), reject func(any))

	// SetClosed sets the [goja.Promise] that resolves when the stream is closed.
	SetClosed(p *goja.Promise, resolve func(any), reject func(any))

	// Cancel returns a [goja.Promise] that resolves when the stream is canceled.
	Cancel(reason goja.Value) *goja.Promise
}

// BaseReadableStreamReader is a base implement
type BaseReadableStreamReader struct {
	closedPromise            *goja.Promise
	closedPromiseResolveFunc func(resolve any)
	closedPromiseRejectFunc  func(reason any)

	// stream is a [ReadableStream] instance that owns this reader
	stream *ReadableStream
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
}

// GetClosed returns the reader's closed promise as well as its resolve and reject functions.
func (reader *BaseReadableStreamReader) GetClosed() (p *goja.Promise, resolve func(any), reject func(any)) {
	return reader.closedPromise, reader.closedPromiseResolveFunc, reader.closedPromiseRejectFunc
}

// SetClosed sets the reader's closed promise as well as its resolve and reject functions.
func (reader *BaseReadableStreamReader) SetClosed(p *goja.Promise, resolve func(any), reject func(any)) {
	reader.closedPromise = p
	reader.closedPromiseResolveFunc = resolve
	reader.closedPromiseRejectFunc = reject
}

// Cancel returns a [goja.Promise] that resolves when the stream is canceled.
func (reader *BaseReadableStreamReader) Cancel(reason goja.Value) *goja.Promise {
	if reader.stream == nil {
		return newRejectedPromise(reader.stream.vu, newError(TypeError, "stream is undefined"))
	}

	return reader.cancel(reason)
}

// cancel implements the [ReadableStreamReaderGenericCancel(reader, reason)] [specification] algorithm.
//
// [specification]: https://streams.spec.whatwg.org/#readable-stream-reader-generic-cancel
func (reader *BaseReadableStreamReader) cancel(reason goja.Value) *goja.Promise {
	// 1.
	stream := reader.stream

	// 2.
	if stream == nil {
		common.Throw(reader.stream.vu.Runtime(), newError(AssertionError, "stream is not undefined"))
	}

	// 3.
	return stream.cancel(reason)
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
	// 1.
	reader.SetStream(stream)

	// 2.
	stream.reader = reader

	// 3.
	promise, resolve, reject := promises.New(stream.vu)
	switch stream.state {
	case ReadableStreamStateReadable:
		break
	case ReadableStreamStateClosed:
		go func() {
			resolve(goja.Undefined())
		}()
	default:
		if stream.state != ReadableStreamStateErrored {
			common.Throw(stream.vu.Runtime(), newError(AssertionError, "stream.state is not \"errored\""))
		}

		go func() {
			reject(stream.storedError)
		}()
	}

	reader.SetClosed(promise, resolve, reject)
}
