package streams

import (
	"github.com/dop251/goja"
	"go.k6.io/k6/js/common"
)

// UnderlyingSource represents the underlying source of a ReadableStream, and defines how
// the underlying data is pulled from the source.
//
// [specification]: https://streams.spec.whatwg.org/#dictdef-underlyingsource
type UnderlyingSource struct {
	// StartFunc is called immediately during the creation of a ReadableStream.
	//
	// Typically this is used to a adapt a push source by setting up relevant event listeners.
	// If the setup process is asynchronous, it can return a Promise to signal success or
	// failure; a rejected promise will error the stream.
	Start UnderlyingSourceStartCallback `json:"start"`

	// PullFunc is  a function that is called whenever the stream's internal queue of chunks
	// becomes not full, i.e. whenever the queue's desired size becomes positive.
	//
	// Generally it will be called repeatedly until the queue reaches its high watermark.
	//
	// This function will not be called until `start()` successfully completes. Additionally,
	// it will only be called repeatedly if it enqueues at least one chunk or fulfills a
	// BYOB request; a no-op `pull` implementation will not be continually called.
	Pull UnderlyingSourcePullCallback `json:"pull"`

	// CancelFunc is a function that is called when the stream's or reader's `cancel()` method is
	// called.
	//
	// It takes as its argument the same value as was passed to those methods by the consumer.
	//
	// For all streams, this is generally used to release access to the underlying resource.
	//
	// If the shutdown process is asynchronous, it can return a promise to signal success or
	// failure; the result will be communicated via the return value of the cancel() method
	// that was called. Throwing an exception is treated the same as returning a rejected promise.
	Cancel UnderlyingSourceCancelCallback `json:"cancel"`

	// Type is a string indicating the type of the underlying source.
	Type ReadableStreamType `json:"type"`

	// AutoAllocateChunkSize (optional) is a non-negative integer indicating the size of
	// chunks to allocate when auto-allocating chunks.
	//
	// Can be set to a positive integer to cause the implementation to automatically
	// allocate buffers for the underlying source code to write into. In this case, when
	// a consumer is using a default reader, the stream implementation will automatically
	// allocate an ArrayBuffer of the given size, so that `controller.byobRequest` is always
	// present, as if the consumer was using a BYOB reader.
	AutoAllocateChunkSize uint

	// startSet is true if the start function was set by the user.
	startSet bool

	// pullSet is true if the pull function was set by the user.
	pullSet bool

	// cancelSet is true if the cancel function was set by the user.
	cancelSet bool
}

// UnderlyingSourceStartCallback is a function that is called immediately during the creation of a ReadableStream.
type UnderlyingSourceStartCallback func(controller ReadableStreamController) goja.Value

// UnderlyingSourcePullCallback is a function that is called whenever the stream's internal queue of chunks
// becomes not full, i.e. whenever the queue's desired size becomes positive.
type UnderlyingSourcePullCallback func(controller ReadableStreamController) *goja.Promise

// UnderlyingSourceCancelCallback is a function that is called when the stream's or reader's `cancel()` method is
// called.
type UnderlyingSourceCancelCallback func(reason any) *goja.Promise

// NewUnderlyingSourceFromObject creates a new UnderlyingSource from a goja.Object.
func NewUnderlyingSourceFromObject(rt *goja.Runtime, obj *goja.Object) (*UnderlyingSource, error) {
	var underlyingSource *UnderlyingSource

	if common.IsNullish(obj) {
		// If the user didn't provide a underlying source, use the default one.
		return underlyingSource, nil
	}

	if err := rt.ExportTo(obj, &underlyingSource); err != nil {
		return underlyingSource, newError(TypeError, "invalid underlying source object")
	}

	startProperty := obj.Get("start")
	if !common.IsNullish(startProperty) {
		underlyingSource.startSet = true
	}

	if !common.IsNullish(obj.Get("pull")) {
		underlyingSource.pullSet = true
	}

	if !common.IsNullish(obj.Get("cancel")) {
		underlyingSource.cancelSet = true
	}

	return underlyingSource, nil
}