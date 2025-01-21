package streams

import "github.com/grafana/sobek"

// ReadableStreamController is the interface implemented by all readable stream controllers.
//
// It defines both the specification's shared controller and private methods.
type ReadableStreamController interface {
	Close()
	Enqueue(chunk sobek.Value)
	Error(err sobek.Value)

	// cancelSteps performs the controller’s steps that run in reaction to
	// the stream being canceled, used to clean up the state stored in the
	// controller and inform the underlying source.
	cancelSteps(reason any) *sobek.Promise

	// pullSteps performs the controller’s steps that run when a default reader
	// is read from, used to pull from the controller any queued chunks, or
	// pull from the underlying source to get more chunks.
	pullSteps(readRequest ReadRequest)

	// releaseSteps performs the controller’s steps that run when a reader is
	// released, used to clean up reader-specific resources stored in the controller.
	releaseSteps()

	// toObject returns a [*sobek.Object] that represents the controller.
	toObject() (*sobek.Object, error)
}

// SizeAlgorithm is a function that returns the size of a chunk.
// type SizeAlgorithm func(chunk sobek.Value) (float64, error)
type SizeAlgorithm = sobek.Callable
