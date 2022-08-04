// Package exitcodes contains the constants representing possible k6 exit error codes.
//nolint:golint
package exitcodes

// ExitCode is just a type representing a process exit code for k6
type ExitCode uint8

// list of exit codes used by k6
const (
	CloudTestRunFailed       ExitCode = 97 // This used to be 99 before k6 v0.33.0
	CloudFailedToGetProgress ExitCode = 98
	ThresholdsHaveFailed     ExitCode = 99
	SetupTimeout             ExitCode = 100
	TeardownTimeout          ExitCode = 101
	GenericTimeout           ExitCode = 102 // TODO: remove?
	GenericEngine            ExitCode = 103
	InvalidConfig            ExitCode = 104
	ExternalAbort            ExitCode = 105
	CannotStartRESTAPI       ExitCode = 106
	ScriptException          ExitCode = 107
	ScriptAborted            ExitCode = 108
)
