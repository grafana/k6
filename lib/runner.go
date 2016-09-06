package lib

import (
	"context"
	"github.com/loadimpact/speedboat/stats"
)

// A Runner is a factory for VUs.
type Runner interface {
	// Creates a new VU. As much as possible should be precomputed here, to allow a pool
	// of prepared VUs to be used to quickly scale up and down.
	NewVU() (VU, error)
}

// A VU is a Virtual User.
type VU interface {
	// Runs the VU once. An iteration should be completely self-contained, and no state
	// or open connections should carry over from one iteration to the next.
	RunOnce(ctx context.Context) ([]stats.Sample, error)

	// Called when the VU's identity changes.
	Reconfigure(id int64) error
}
