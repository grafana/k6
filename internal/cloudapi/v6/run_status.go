package cloudapi

// RunStatus values are used to tell the cloud output how a local test run
// ended, and to get that information from the cloud for cloud tests.
type RunStatus string

// Possible run status values; iota isn't used intentionally
const (
	RunStatusCreated           RunStatus = "created"
	RunStatusQueued            RunStatus = "queued"
	RunStatusInitializing      RunStatus = "initializing"
	RunStatusRunning           RunStatus = "running"
	RunStatusProcessingMetrics RunStatus = "processing_metrics"
	RunStatusCompleted         RunStatus = "completed"
	RunStatusAborted           RunStatus = "aborted"
)
