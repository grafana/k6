package lib

// TODO: move to some other package - types? models?

// RunStatus values can be used by k6 to denote how a script run ends
// and by the cloud executor and collector so that k6 knows the current
// status of a particular script run.
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
)
