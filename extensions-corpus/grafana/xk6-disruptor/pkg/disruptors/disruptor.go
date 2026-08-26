package disruptors

import "context"

// Disruptor defines the generic interface implemented by all disruptors
type Disruptor interface {
	// Targets returns the names of the targets for the disruptor
	Targets(ctx context.Context) ([]string, error)
}
