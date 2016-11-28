package stats

import (
	"context"
)

// A Collector abstracts away the details of a storage backend from the application.
type Collector interface {
	// Run is called in a goroutine and starts the collector. Should commit samples to the backend
	// at regular intervals and when the context is terminated.
	Run(ctx context.Context)

	// Collect receives a set of samples. This method is never called concurrently, and only while
	// the context for Run() is valid, but should defer as much work as possible to Run().
	Collect(samples []Sample)
}
