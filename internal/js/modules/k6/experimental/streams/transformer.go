package streams

import (
	"github.com/grafana/sobek"
	"go.k6.io/k6/v2/js/common"
)

// Transformer represents the set of algorithms that a TransformStream uses to
// transform the chunks written to its writable side into chunks readable from
// its readable side.
//
// [specification]: https://streams.spec.whatwg.org/#dictdef-transformer
type Transformer struct {
	// Start is called immediately during the creation of a TransformStream.
	//
	// Typically, this is used to enqueue prefix chunks, using controller.enqueue().
	// If the setup process is asynchronous, it can return a promise to signal success
	// or failure; a rejected promise will error the stream. Any thrown exceptions will
	// be re-thrown by the TransformStream constructor.
	Start sobek.Value `js:"start"`

	// Transform is called when a new chunk originally written to the writable side is
	// ready to be transformed.
	//
	// If no transform function is supplied, the identity transform is used, which
	// enqueues chunks unchanged from the writable side to the readable side.
	//
	// If the process of transforming is asynchronous, it can return a promise to signal
	// success or failure; a rejected promise will error both the readable and writable
	// sides of the transform stream.
	Transform sobek.Value `js:"transform"`

	// Flush is called after all chunks written to the writable side have been transformed
	// by successfully passing through transform(), and the writable side is about to be
	// closed.
	//
	// Typically, this is used to enqueue suffix chunks to the readable side, before that
	// too becomes closed.
	Flush sobek.Value `js:"flush"`

	// Cancel is called when the readable side is cancelled, or when the writable side is
	// aborted.
	//
	// Typically, this is used to clean up underlying transformer resources when the stream
	// is aborted or cancelled.
	Cancel sobek.Value `js:"cancel"`

	// ReadableType is reserved for future use, and must be absent. When it exists, the
	// TransformStream constructor throws a RangeError exception.
	ReadableType sobek.Value `js:"readableType"`

	// WritableType is reserved for future use, and must be absent. When it exists, the
	// TransformStream constructor throws a RangeError exception.
	WritableType sobek.Value `js:"writableType"`
}

// TransformerTransformCallback is the promise-returning algorithm, taking one argument (the
// chunk to transform), which requests the transformer perform its transformation.
type TransformerTransformCallback func(chunk sobek.Value) *sobek.Promise

// TransformerFlushCallback is the promise-returning algorithm which communicates a requested
// close to the transformer.
type TransformerFlushCallback func() *sobek.Promise

// TransformerCancelCallback is the promise-returning algorithm, taking one argument (the reason
// for cancellation), which communicates a requested cancellation to the transformer.
type TransformerCancelCallback func(reason any) *sobek.Promise

// NewTransformerFromObject creates a new Transformer from a sobek.Object.
func NewTransformerFromObject(rt *sobek.Runtime, obj *sobek.Object) (Transformer, error) {
	var transformer Transformer

	if common.IsNullish(obj) {
		// If the user didn't provide a transformer, use the default (identity) one.
		return transformer, nil
	}

	if err := rt.ExportTo(obj, &transformer); err != nil {
		return transformer, newTypeError(rt, "invalid transformer object")
	}

	return transformer, nil
}
