package cloudapi

// RunStatus values are used to tell the cloud output how a local test run
// ended, and to get that information from the cloud for cloud tests.
type RunStatus int

// Possible run status values; iota isn't used intentionally
const (
	RunStatusCreated            RunStatus = -2
	RunStatusValidated          RunStatus = -1
	RunStatusQueued             RunStatus = 0
	RunStatusInitializing       RunStatus = 1
	RunStatusRunning            RunStatus = 2
	RunStatusFinished           RunStatus = 3
	RunStatusTimedOut           RunStatus = 4
	RunStatusAbortedUser        RunStatus = 5
	RunStatusAbortedSystem      RunStatus = 6
	RunStatusAbortedScriptError RunStatus = 7
	RunStatusAbortedThreshold   RunStatus = 8
	RunStatusAbortedLimit       RunStatus = 9
	RunStatusArchived           RunStatus = 10
)
