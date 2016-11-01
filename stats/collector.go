package stats

import (
	"context"
)

// A Collector abstracts away the details of a storage backend from the application.
type Collector interface {
	// Run is called in a goroutine and starts the collector. Should commit samples to the backend
	// at regular intervals and when the context is terminated.
	Run(ctx context.Context)

	// Buffer returns a buffer belonging to this collector. The collector should track issued
	// buffers in some way.
	Buffer() Buffer
}

// A Buffer is a container for Samples. They are to be drained by a running Collector at regular
// intervals.
type Buffer interface {
	// Adds a set of samples to the buffer.
	Add(samples ...Sample)

	// Drain empties the buffer and returns the previous contents.
	Drain() []Sample
}
