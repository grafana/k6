package streams

import (
	"github.com/grafana/sobek"

	"go.k6.io/k6/js/common"
)

// ReadableStreamDefaultReader represents a default reader designed to be vended by a [ReadableStream].
type ReadableStreamDefaultReader struct {
	BaseReadableStreamReader

	// readRequests holds a list of read requests, used when a consumer requests
	// chunks sooner than they are available.
	readRequests []ReadRequest
}

// NewReadableStreamDefaultReaderObject creates a new sobek.Object from a [ReadableStreamDefaultReader] instance.
func NewReadableStreamDefaultReaderObject(reader *ReadableStreamDefaultReader) (*sobek.Object, error) {
	rt := reader.stream.runtime
	obj := rt.NewObject()
	objName := "ReadableStreamDefaultReader"

	err := obj.DefineAccessorProperty("closed", rt.ToValue(func() *sobek.Promise {
		p, _, _ := reader.GetClosed()
		return p
	}), nil, sobek.FLAG_FALSE, sobek.FLAG_TRUE)
	if err != nil {
		return nil, err
	}

	if err := setReadOnlyPropertyOf(obj, objName, "cancel", rt.ToValue(reader.Cancel)); err != nil {
		return nil, err
	}

	// Exposing the properties of the [ReadableStreamDefaultReader] interface
	if err := setReadOnlyPropertyOf(obj, objName, "read", rt.ToValue(reader.Read)); err != nil {
		return nil, err
	}

	if err := setReadOnlyPropertyOf(obj, objName, "releaseLock", rt.ToValue(reader.ReleaseLock)); err != nil {
		return nil, err
	}

	return obj, nil
}

// Ensure the ReadableStreamReader interface is implemented correctly
var _ ReadableStreamReader = &ReadableStreamDefaultReader{}

// Read returns a [sobek.Promise] providing access to the next chunk in the stream's internal queue.
func (reader *ReadableStreamDefaultReader) Read() *sobek.Promise {
	stream := reader.GetStream()

	// 1. If this.[[stream]] is undefined, return a promise rejected with a TypeError exception.
	if stream == nil {
		return newRejectedPromise(reader.vu, newTypeError(reader.runtime, "stream is undefined").Err())
	}

	// 2. Let promise be a new promise.
	promise, resolve, reject := stream.vu.Runtime().NewPromise()

	// 3. Let readRequest be a new read request with the following items:
	readRequest := ReadRequest{
		chunkSteps: func(chunk any) {
			// Resolve promise with «[ "value" → chunk, "done" → false ]».
			// TODO(@mstoykov): propagate as error?
			err := resolve(map[string]any{"value": chunk, "done": false})
			if err != nil {
				panic(err)
			}
		},
		closeSteps: func() {
			// Resolve promise with «[ "value" → undefined, "done" → true ]».
			err := resolve(map[string]any{"value": sobek.Undefined(), "done": true})
			if err != nil {
				panic(err)
			}
		},
		errorSteps: func(e any) {
			// Reject promise with e.
			err := reject(e)
			if err != nil {
				panic(err)
			}
		},
	}

	// 4. Perform ! ReadableStreamDefaultReaderRead(this, readRequest).
	reader.read(readRequest)

	// 5. Return promise.
	return promise
}

// Cancel returns a [sobek.Promise] that resolves when the stream is canceled.
//
// Calling this method signals a loss of interest in the stream by a consumer. The
// supplied reason argument will be given to the underlying source, which may or
// may not use it.
//
// The `reason` argument is optional, and should hold a human-readable reason for
// the cancellation. This value may or may not be used.
//
// [SetUpReadableStreamDefaultReader]: https://streams.spec.whatwg.org/#set-up-readable-stream-default-reader
func (reader *ReadableStreamDefaultReader) Cancel(reason sobek.Value) *sobek.Promise {
	// 1. If this.[[stream]] is undefined, return a promise rejected with a TypeError exception.
	if reader.stream == nil {
		return newRejectedPromise(reader.vu, newTypeError(reader.runtime, "stream is undefined").Err())
	}

	// 2. Return ! ReadableStreamReaderGenericCancel(this, reason).
	return reader.BaseReadableStreamReader.Cancel(reason)
}

// ReadResult is the result of a read operation
//
// It contains the value read from the stream and a boolean indicating whether or not the stream is done.
// An undefined value indicates that the stream has been closed.
type ReadResult struct {
	Value sobek.Value
	Done  bool
}

// ReleaseLock releases the reader's lock on the stream.
//
// If the associated stream is errored when the lock is released, the
// reader will appear errored in that same way subsequently; otherwise, the
// reader will appear closed.
func (reader *ReadableStreamDefaultReader) ReleaseLock() {
	// 1. If this.[[stream]] is undefined, return.
	if reader.stream == nil {
		return
	}

	// 2. Perform ! ReadableStreamDefaultReaderRelease(this).
	reader.release()
}

// release implements the [ReadableStreamDefaultReaderRelease] algorithm.
//
// [ReadableStreamDefaultReaderRelease]:
// https://streams.spec.whatwg.org/#abstract-opdef-readablestreamdefaultreaderrelease
func (reader *ReadableStreamDefaultReader) release() {
	// 1. Perform ! ReadableStreamReaderGenericRelease(reader).
	reader.BaseReadableStreamReader.release()

	// 2. Let e be a new TypeError exception.
	e := newTypeError(reader.runtime, "reader released")

	// 3. Perform ! ReadableStreamDefaultReaderErrorReadRequests(reader, e).
	reader.errorReadRequests(e.Err())
}

// setup implements the [SetUpReadableStreamDefaultReader] algorithm.
//
// [SetUpReadableStreamDefaultReader]: https://streams.spec.whatwg.org/#set-up-readable-stream-default-reader
func (reader *ReadableStreamDefaultReader) setup(stream *ReadableStream) {
	rt := stream.vu.Runtime()

	// 1. If ! IsReadableStreamLocked(stream) is true, throw a TypeError exception.
	if stream.isLocked() {
		throw(rt, newTypeError(rt, "stream is locked"))
	}

	// 2. Perform ! ReadableStreamReaderGenericInitialize(reader, stream).
	ReadableStreamReaderGenericInitialize(reader, stream)

	// 3. Set reader.[[readRequests]] to a new empty list.
	reader.readRequests = []ReadRequest{}
}

// Implements the [specification]'s ReadableStreamDefaultReaderErrorReadRequests algorithm.
//
// [specification]: https://streams.spec.whatwg.org/#abstract-opdef-readablestreamdefaultreadererrorreadrequests
func (reader *ReadableStreamDefaultReader) errorReadRequests(e any) {
	// 1. Let readRequests be reader.[[readRequests]].
	readRequests := reader.readRequests

	// 2. Set reader.[[readRequests]] to a new empty list.
	reader.readRequests = []ReadRequest{}

	// 3. For each readRequest of readRequests,
	for _, request := range readRequests {
		// 3.1. Perform readRequest’s error steps, given e.
		request.errorSteps(e)
	}
}

// read implements the [ReadableStreamDefaultReaderRead] algorithm.
//
// [ReadableStreamDefaultReaderRead]: https://streams.spec.whatwg.org/#readable-stream-default-reader-read
func (reader *ReadableStreamDefaultReader) read(readRequest ReadRequest) {
	// 1. Let stream be reader.[[stream]].
	stream := reader.GetStream()

	// 2. Assert: stream is not undefined.
	if stream == nil {
		common.Throw(stream.vu.Runtime(), newError(AssertionError, "stream is undefined"))
	}

	// 3. Set stream.[[disturbed]] to true.
	stream.disturbed = true

	switch stream.state {
	case ReadableStreamStateClosed:
		// 4. If stream.[[state]] is "closed", perform readRequest’s close steps.
		readRequest.closeSteps()
	case ReadableStreamStateErrored:
		// 5. Otherwise, if stream.[[state]] is "errored", perform readRequest’s error steps given stream.[[storedError]].
		if jsErr, ok := stream.storedError.(*jsError); ok {
			readRequest.errorSteps(jsErr.Err())
		} else {
			readRequest.errorSteps(stream.storedError)
		}

	default:
		// 6. Otherwise,
		// 6.1. Assert: stream.[[state]] is "readable".
		if stream.state != ReadableStreamStateReadable {
			common.Throw(stream.vu.Runtime(), newError(AssertionError, "stream.state is not readable"))
		}

		// 6.2. Perform ! stream.[[controller]].[[PullSteps]](readRequest).
		stream.controller.pullSteps(readRequest)
	}
}
