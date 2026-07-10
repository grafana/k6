package streams

import (
	"github.com/grafana/sobek"
	"go.k6.io/k6/v2/js/common"
)

// UnderlyingSink represents the underlying sink of a WritableStream, and defines how
// the underlying data is written to the sink.
//
// [specification]: https://streams.spec.whatwg.org/#dictdef-underlyingsink
type UnderlyingSink struct {
	// Start is called immediately during the creation of a WritableStream.
	//
	// Typically, this is used to acquire access to the underlying sink resource.
	// If the setup process is asynchronous, it can return a promise to signal success
	// or failure; a rejected promise will error the stream.
	//
	// Any thrown exceptions will be re-thrown by the WritableStream constructor.
	Start sobek.Value `json:"start"`

	// Write is called when a new chunk of data is ready to be written to the underlying
	// sink.
	//
	// The stream implementation guarantees that this function will be called only after
	// previous writes have succeeded, and never before start() has succeeded or after
	// close() or abort() have been called.
	//
	// If the process of writing is asynchronous, it can return a promise to signal success
	// or failure; the stream will apply backpressure until the returned promise settles.
	// Throwing an exception is treated the same as returning a rejected promise.
	Write sobek.Value `json:"write"`

	// Close is called after the producer signals, via the writer, that they are done
	// writing chunks to the stream, and subsequently all queued-up writes have
	// successfully completed.
	//
	// If the shutdown process is asynchronous, it can return a promise to signal success
	// or failure; the result will be communicated via the return value of the close()
	// method that was called. Throwing an exception is treated the same as returning a
	// rejected promise.
	Close sobek.Value `json:"close"`

	// Abort is called after the producer signals, via the writer, that they wish to
	// abruptly close the stream and put it in an errored state.
	//
	// It takes as its argument the same value as was passed to the writer's or stream's
	// abort() method.
	//
	// If the shutdown process is asynchronous, it can return a promise to signal success
	// or failure; the result will be communicated via the return value of the abort()
	// method that was called. Throwing an exception is treated the same as returning a
	// rejected promise.
	Abort sobek.Value `json:"abort"`

	// Type is reserved for future use, and must be absent. When it exists, the
	// WritableStream constructor throws a RangeError exception.
	Type sobek.Value `json:"type"`
}

// UnderlyingSinkStartCallback is called immediately during the creation of a WritableStream.
type UnderlyingSinkStartCallback func(controller *sobek.Object) sobek.Value

// UnderlyingSinkWriteCallback is called when a new chunk of data is ready to be written to
// the underlying sink.
type UnderlyingSinkWriteCallback func(chunk sobek.Value, controller *sobek.Object) *sobek.Promise

// UnderlyingSinkCloseCallback is called after the producer signals that they are done
// writing chunks to the stream.
type UnderlyingSinkCloseCallback func() *sobek.Promise

// UnderlyingSinkAbortCallback is called after the producer signals that they wish to
// abruptly close the stream and put it in an errored state.
type UnderlyingSinkAbortCallback func(reason any) *sobek.Promise

// NewUnderlyingSinkFromObject creates a new UnderlyingSink from a sobek.Object.
func NewUnderlyingSinkFromObject(rt *sobek.Runtime, obj *sobek.Object) (UnderlyingSink, error) {
	var underlyingSink UnderlyingSink

	if common.IsNullish(obj) {
		// If the user didn't provide an underlying sink, use the default one.
		return underlyingSink, nil
	}

	if err := rt.ExportTo(obj, &underlyingSink); err != nil {
		return underlyingSink, newTypeError(rt, "invalid underlying sink object")
	}

	return underlyingSink, nil
}

func isDictionaryMemberPresent(value sobek.Value) bool {
	return value != nil && !sobek.IsUndefined(value)
}
