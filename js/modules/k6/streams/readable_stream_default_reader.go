package streams

import (
	"github.com/dop251/goja"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/promises"
)

// ReadableStreamDefaultReader represents a default reader designed to be vended by a [ReadableStream].
type ReadableStreamDefaultReader struct {
	BaseReadableStreamReader

	// readRequests holds a list of read requests, used when a consumer requests
	// chunks sooner than they are available.
	readRequests []ReadRequest
}

// Ensure the ReadableStreamReader interface is implemented correctly
var _ ReadableStreamReader = &ReadableStreamDefaultReader{}

// Read returns a [goja.Promise] providing access to the next chunk in the stream's internal queue.
func (reader *ReadableStreamDefaultReader) Read() *goja.Promise {
	stream := reader.GetStream()

	// 1.
	if stream == nil {
		return newRejectedPromise(stream.vu, newError(TypeError, "stream is undefined"))
	}

	// 2.
	promise, resolve, reject := promises.New(stream.vu)

	// 3.
	// TODO: should this be wrapped in a goroutine? I assumed not, considering
	// the call the callbacks is deferred to a later point in time, but I'm not sure.
	readRequest := ReadRequest{
		chunkSteps: func(chunk any) {
			go func() {
				resolve(map[string]any{"value": chunk, "done": false})
			}()
		},
		closeSteps: func() {
			go func() {
				resolve(map[string]any{"value": goja.Undefined(), "done": true})
			}()
		},
		errorSteps: func(e any) {
			go func() {
				reject(e)
			}()
		},
	}

	// 4.
	reader.read(readRequest)

	// 5.
	return promise
}

// Closed returns a [goja.Promise] that fulfills when the stream closes, or
// rejects if the stream throws an error or the reader's lock is released.
//
// This property enables you to write code that responds to an end to the streaming process.
func (reader *ReadableStreamDefaultReader) Closed() *goja.Promise {
	// FIXME: should be exposed as a property instead of a method
	// Implement logic to return a promise that fulfills or rejects based on the reader's state
	// The promise should fulfill when the reader is closed and reject if the reader is errored
	return nil
}

// Cancel returns a [goja.Promise] that resolves when the stream is canceled.
//
// Calling this method signals a loss of interest in the stream by a consumer. The
// supplied reason argument will be given to the underlying source, which may or
// may not use it.
//
// The `reason` argument is optional, and should hold A human-readable reason for
// the cancellation. This value may or may not be used.
// FIXME: implement according to specification.
func (reader *ReadableStreamDefaultReader) Cancel(_ goja.Value) *goja.Promise {
	// Implement logic to return a promise that fulfills or rejects based on the reader's state
	// The promise should fulfill when the reader is closed and reject if the reader is errored
	return nil
}

// ReadResult is the result of a read operation
//
// It contains the value read from the stream and a boolean indicating whether or not the stream is done.
// An undefined value indicates that the stream has been closed.
type ReadResult struct {
	Value goja.Value
	Done  bool
}

// ReleaseLock releases the reader's lock on the stream.
//
// If the associated stream is errored when the lock is released, the
// reader will appear errored in that same way subsequently; otherwise, the
// reader will appear closed.
func (reader *ReadableStreamDefaultReader) ReleaseLock() {
	// Implement the logic to release the lock on the stream
	// This might involve changing the state of the stream and handling any queued read requests
}

// setup implements the [SetUpReadableStreamDefaultReader] algorithm.
//
// [SetUpReadableStreamDefaultReader]: https://streams.spec.whatwg.org/#set-up-readable-stream-default-reader
func (reader *ReadableStreamDefaultReader) setup(stream *ReadableStream) {
	// 1.
	if stream.isLocked() {
		common.Throw(reader.GetStream().vu.Runtime(), newError(TypeError, "stream is locked"))
	}

	// 2.
	ReadableStreamReaderGenericInitialize(reader, stream)

	// 3.
	reader.readRequests = []ReadRequest{}
}

// Implements the [specification]'s ReadableStreamDefaultReaderErrorReadRequests algorithm.
//
// [specification]: https://streams.spec.whatwg.org/#abstract-opdef-readablestreamdefaultreadererrorreadrequests
func (reader *ReadableStreamDefaultReader) errorReadRequests(e any) {
	// 1.
	readRequests := reader.readRequests

	// 2.
	reader.readRequests = []ReadRequest{}

	// 3.
	for _, request := range readRequests {
		// 3.1.
		request.errorSteps(e)
	}
}

// read implements the [ReadableStreamDefaultReaderRead] algorithm.
//
// [ReadableStreamDefaultReaderRead]: https://streams.spec.whatwg.org/#abstract-opdef-readablestreamdefaultreaderread
func (reader *ReadableStreamDefaultReader) read(readRequest ReadRequest) {
	// 1.
	stream := reader.GetStream()

	// 2.
	if stream == nil {
		common.Throw(stream.vu.Runtime(), newError(AssertionError, "stream is undefined"))
	}

	// 3.
	stream.disturbed = true

	switch stream.state {
	case ReadableStreamStateClosed:
		// 4.
		readRequest.closeSteps()
	case ReadableStreamStateErrored:
		// 5.
		readRequest.errorSteps(stream.storedError)
	default:
		// 6.
		if stream.state != ReadableStreamStateReadable {
			common.Throw(stream.vu.Runtime(), newError(AssertionError, "stream.state is not readable"))
		}

		stream.controller.pullSteps(readRequest)
	}
}
