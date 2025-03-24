package lib

import (
	"context"
	"io"

	"go.k6.io/k6/internal/lib/summary"
	"go.k6.io/k6/metrics"
)

// ActiveVU represents an actively running virtual user.
type ActiveVU interface {
	// RunOnce runs the configured exported function in the VU once.
	// The only way to interrupt the execution is to cancel the context given
	// to InitializedVU.Activate()
	RunOnce() error
}

// InitializedVU represents a virtual user ready for work. It needs to be
// activated (i.e. given a context) before it can actually be used. Activation
// also requires a callback function, which will be called when the supplied
// context is done. That way, VUs can be returned to a pool and reused.
type InitializedVU interface {
	// Activate fully activate the VU so it will be able to run code.
	Activate(*VUActivationParams) ActiveVU

	// GetID returns the unique VU ID.
	GetID() uint64
}

// VUActivationParams are supplied by each executor when it retrieves a VU from
// the buffer pool and activates it for use.
type VUActivationParams struct {
	RunContext               context.Context
	DeactivateCallback       func(InitializedVU)
	Env, Tags                map[string]string
	Exec, Scenario           string
	GetNextIterationCounters func() (uint64, uint64)
}

// A Runner is a factory for VUs. It should precompute as much as possible upon
// creation (parse ASTs, load files into memory, etc.), so that spawning VUs
// becomes as fast as possible. The Runner doesn't actually *do* anything in
// itself, the ExecutionScheduler is responsible for wrapping and scheduling
// these VUs for execution.
//
// TODO: Rename this to something more obvious? This name made sense a very long
// time ago.
//
// interfacebloat: We may evaluate in the future to move out some methods;
// but considering how central it is, it would require a huge effort.
type Runner interface {
	// MakeArchive creates an Archive of the runner. There should be a corresponding NewFromArchive() function
	// that will restore the runner from the archive.
	MakeArchive() *Archive

	// NewVU spawns a new VU. It's fine to make this function rather heavy, if it means a performance
	// improvement at runtime. Remember, this is called once per VU and normally only at the start
	// of a test - RunOnce() may be called hundreds of thousands of times, and must be fast.
	NewVU(ctx context.Context, idLocal, idGlobal uint64, out chan<- metrics.SampleContainer) (InitializedVU, error)

	// Setup runs pre-test setup, if applicable.
	Setup(ctx context.Context, out chan<- metrics.SampleContainer) error

	// GetSetupData returns json representation of the setup data if setup() is specified and run, nil otherwise
	GetSetupData() []byte

	// SetSetupData saves the externally supplied setup data as json in the runner
	SetSetupData([]byte)

	// Teardown runs post-test teardown, if applicable.
	Teardown(ctx context.Context, out chan<- metrics.SampleContainer) error

	// Get and set options. The initial value will be whatever the script specifies (for JS,
	// `export let options = {}`); cmd/run.go will mix this in with CLI-, config- and env-provided
	// values and write it back to the runner.
	GetOptions() Options
	SetOptions(opts Options) error

	// IsExecutable returns whether the given name is an exported and executable
	// function in the script.
	IsExecutable(string) bool

	HandleSummary(context.Context, *LegacySummary, *summary.Summary) (map[string]io.Reader, error)
}

// UIState describes the state of the UI, which might influence what
// handleSummary() returns.
type UIState struct {
	IsStdOutTTY bool
	IsStdErrTTY bool
}
