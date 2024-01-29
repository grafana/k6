package streams

import (
	"github.com/dop251/goja"
)

// ReadableStreamBYOBReader is a [readable stream BYOB reader] object.
//
// [readable stream BYOB reader]: https://streams.spec.whatwg.org/#rs-byob-reader
// FIXME: Needs a constructor constructor(ReadableStream stream)
type ReadableStreamBYOBReader struct {
	ReadableStreamGenericReader

	// FIXME: "A list of read-into requests, used when a consumer requests chunks sooner than they are available"
	readIntoRequests []ReadIntoRequest
}

// Read implements the [read] algorithm.
//
// [read]: https://streams.spec.whatwg.org/#readable-stream-byob-reader-read
func (rsbr *ReadableStreamBYOBReader) Read(_ goja.Value, _ ReadableStreamBYOBReaderReadOptions) *goja.Promise {
	// Implement logic to return a promise that fulfills or rejects based on the reader's state
	// The promise should fulfill when the reader is closed and reject if the reader is errored
	return nil
}

// ReleaseLock implements the [release lock] algorithm.
//
// [release lock]: https://streams.spec.whatwg.org/#release-lock
func (rsbr *ReadableStreamBYOBReader) ReleaseLock() {
	stream := rsbr.GetStream()

	if stream == nil {
		return
	}

	// TODO: implement the following
	// 1. Perform ! ReadableStreamReaderGenericRelease(reader).
	// 2. Let e be a new TypeError exception.
	// 3. Perform ! ReadableStreamBYOBReaderErrorReadIntoRequests(reader, e).
}

// ReadableStreamBYOBReaderReadOptions implements the [read options] dictionary.
//
// [read options]: https://streams.spec.whatwg.org/#readable-stream-byob-reader-read-options
// TODO: implement
type ReadableStreamBYOBReaderReadOptions struct {
	// FIXME: default should be 1
	// FIXME: it's marked [EnforcedRange], what does it mean?
	Min uint
}

// errorReadIntoRequests implements the specification ReadableStreamBYOBReaderErrorReadIntoRequests algorithm.
//
// [specification]: https://streams.spec.whatwg.org/#readable-stream-byob-reader-error-read-into-requests
func (rsbr *ReadableStreamBYOBReader) errorReadIntoRequests(e any) {
	// 1. Let readIntoRequests be reader.[[readIntoRequests]].
	readIntoRequests := rsbr.readIntoRequests

	// 2. Set reader.[[readIntoRequests]] to a new empty list.
	rsbr.readIntoRequests = []ReadIntoRequest{}

	// 3. For each readIntoRequest of readIntoRequests,
	for _, readIntoRequest := range readIntoRequests {
		// 3.1. Perform readIntoRequest’s error steps, given e.
		readIntoRequest.errorSteps(e)
	}
}

// ReadIntoRequest implements the [read-into request] struct.
//
// [read-into request]: https://streams.spec.whatwg.org/#read-into-request
type ReadIntoRequest struct {
	// chunkSteps is an algorithm taking a chunk, called when a chunk is available for reading
	chunkSteps func(any)

	// closeSteps is an algorithm taking a chunk or undefined, called when no chunks are available because the stream
	// is closed
	closeSteps func(*any)

	// errorSteps taking a Javascript value, called when no chunks are available because the stream is errored
	errorSteps func(any)
}
