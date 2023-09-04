// Package exitcodes contains the constants representing possible k6 exit error codes.
//
//nolint:golint
package exitcodes

// ExitCode is just a type representing a process exit code for k6
type ExitCode uint8

// list of exit codes used by k6
const (
	// CloudTestRunFailed indicates that the cloud test run failed.
	// Its value used to be 99 before k6 v0.33.0.
	CloudTestRunFailed ExitCode = 97 // This used to be 99 before k6 v0.33.0

	// CloudFailedToGetProgress indicates that k6 was unable to synchronize the
	// test progress with the cloud.
	CloudFailedToGetProgress ExitCode = 98

	// ThresholdsHaveFailed indicates that one or more thresholds have failed.
	ThresholdsHaveFailed ExitCode = 99

	// SetupTimeout indicates the execution of the test setup function timed out.
	SetupTimeout ExitCode = 100

	// TeardownTimeout indicates the execution of the test teardown function timed out.
	TeardownTimeout ExitCode = 101

	// GenericTimeout indicates a timeout with an unspecified reason.
	GenericTimeout ExitCode = 102 // TODO: remove?

	// ScriptStoppedFromRESTAPI indicates the execution has been
	// stopped by a call to the k6's REST API.
	ScriptStoppedFromRESTAPI ExitCode = 103

	// InvalidConfig indicates an invalid configuration.
	InvalidConfig ExitCode = 104

	// ExternalAbort indicates the test was aborted by an external signal
	// (e.g. SIGINT, SIGTERM, etc.) and should be considered aborted rather
	// than a failure.
	ExternalAbort ExitCode = 105

	// CannotStartRESTAPI indicates the k6's REST API server could not be started.
	CannotStartRESTAPI ExitCode = 106

	// ScriptException indicates an exception was thrown during the
	// test script's execution.
	ScriptException ExitCode = 107

	// ScriptAborted indicates the script was aborted by a call to the
	// k6 execution module's `test.abort()` function.
	ScriptAborted ExitCode = 108

	// GoPanic indicates the script was aborted by a panic in the Go runtime.
	GoPanic ExitCode = 109
)
