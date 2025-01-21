package event

// Type represents the different event types emitted by k6.
//
//go:generate enumer -type=Type -trimprefix Type -output type_gen.go
type Type uint8

const (
	// Init is emitted when k6 starts initializing outputs, VUs and executors.
	Init Type = iota + 1
	// TestStart is emitted when the execution scheduler starts running the test.
	TestStart
	// TestEnd is emitted when the test execution ends.
	TestEnd
	// IterStart is emitted when a VU starts an iteration.
	IterStart
	// IterEnd is emitted when a VU ends an iteration.
	IterEnd
	// Exit is emitted when the k6 process is about to exit.
	Exit
)

//nolint:gochecknoglobals
var (
	// GlobalEvents are emitted once per test run.
	GlobalEvents = []Type{Init, TestStart, TestEnd, Exit}
	// VUEvents are emitted multiple times per each VU.
	VUEvents = []Type{IterStart, IterEnd}
)

// ExitData is the data sent in the Exit event. Error is the error returned by
// the run command.
type ExitData struct {
	Error error
}

// IterData is the data sent in the IterStart and IterEnd events.
type IterData struct {
	Iteration    int64
	VUID         uint64
	ScenarioName string
	Error        error
}
