package scheduler

import "time"

// Config is an interface that should be implemented by all scheduler config types
type Config interface {
	GetBaseConfig() BaseConfig
	Validate() []error
	GetMaxVUs() int64
	GetMaxDuration() time.Duration // includes max timeouts, to allow us to share VUs between schedulers in the future
	//TODO: Split(percentages []float64) ([]Config, error)
	//TODO: String() method that could be used for priting descriptions of the currently running schedulers for the UI?
}
