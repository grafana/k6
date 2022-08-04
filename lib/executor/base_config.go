package executor

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/lib/consts"
	"go.k6.io/k6/lib/types"
)

// DefaultGracefulStopValue is the graceful top value for all executors, unless
// it's manually changed by the gracefulStop in each one.
// TODO?: Discard? Or make this actually user-configurable somehow? hello #883...
var DefaultGracefulStopValue = 30 * time.Second //nolint:gochecknoglobals

var executorNameWhitelist = regexp.MustCompile(`^[0-9a-zA-Z_-]+$`) //nolint:gochecknoglobals
const executorNameErr = "the executor name should contain only numbers, latin letters, underscores, and dashes"

// BaseConfig contains the common config fields for all executors
type BaseConfig struct {
	Name         string             `json:"-"` // set via the JS object key
	Type         string             `json:"executor"`
	StartTime    types.NullDuration `json:"startTime"`
	GracefulStop types.NullDuration `json:"gracefulStop"`
	Env          map[string]string  `json:"env"`
	Exec         null.String        `json:"exec"` // function name, externally validated
	Tags         map[string]string  `json:"tags"`

	// TODO: future extensions like distribution, others?
}

// NewBaseConfig returns a default base config with the default values
func NewBaseConfig(name, configType string) BaseConfig {
	return BaseConfig{
		Name:         name,
		Type:         configType,
		GracefulStop: types.NewNullDuration(DefaultGracefulStopValue, false),
	}
}

// Validate checks some basic things like present name, type, and a positive start time
func (bc BaseConfig) Validate() (errors []error) {
	// Some just-in-case checks, since those things are likely checked in other places or
	// even assigned by us:
	if bc.Name == "" {
		errors = append(errors, fmt.Errorf("executor name can't be empty"))
	}
	if !executorNameWhitelist.MatchString(bc.Name) {
		errors = append(errors, fmt.Errorf(executorNameErr))
	}
	if bc.Exec.Valid && bc.Exec.String == "" {
		errors = append(errors, fmt.Errorf("exec value cannot be empty"))
	}
	if bc.Type == "" {
		errors = append(errors, fmt.Errorf("missing or empty type field"))
	}
	// The actually reasonable checks:
	if bc.StartTime.Duration < 0 {
		errors = append(errors, fmt.Errorf("the startTime can't be negative"))
	}
	if bc.GracefulStop.Duration < 0 {
		errors = append(errors, fmt.Errorf("the gracefulStop timeout can't be negative"))
	}
	return errors
}

// GetName returns the name of the executor.
func (bc BaseConfig) GetName() string {
	return bc.Name
}

// GetType returns the executor's type as a string ID.
func (bc BaseConfig) GetType() string {
	return bc.Type
}

// GetStartTime returns the starting time, relative to the beginning of the
// actual test, that this executor is supposed to execute.
func (bc BaseConfig) GetStartTime() time.Duration {
	return bc.StartTime.TimeDuration()
}

// GetGracefulStop returns how long k6 is supposed to wait for any still
// running iterations to finish executing at the end of the normal executor
// duration, before it actually kills them.
//
// Of course, that doesn't count when the user manually interrupts the test,
// then iterations are immediately stopped.
func (bc BaseConfig) GetGracefulStop() time.Duration {
	return bc.GracefulStop.TimeDuration()
}

// GetEnv returns any specific environment key=value pairs that
// are configured for the executor.
func (bc BaseConfig) GetEnv() map[string]string {
	return bc.Env
}

// GetExec returns the configured custom exec value, if any.
func (bc BaseConfig) GetExec() string {
	exec := bc.Exec.ValueOrZero()
	if exec == "" {
		exec = consts.DefaultFn
	}
	return exec
}

// GetTags returns any custom tags configured for the executor.
func (bc BaseConfig) GetTags() map[string]string {
	return bc.Tags
}

// IsDistributable returns true since by default all executors could be run in
// a distributed manner.
func (bc BaseConfig) IsDistributable() bool {
	return true
}

// getBaseInfo is a helper method for the "parent" String methods.
func (bc BaseConfig) getBaseInfo(facts ...string) string {
	if bc.Exec.Valid {
		facts = append(facts, fmt.Sprintf("exec: %s", bc.Exec.String))
	}
	if bc.StartTime.Duration > 0 {
		facts = append(facts, fmt.Sprintf("startTime: %s", bc.StartTime.Duration))
	}
	if bc.GracefulStop.Duration > 0 {
		facts = append(facts, fmt.Sprintf("gracefulStop: %s", bc.GracefulStop.Duration))
	}
	if len(facts) == 0 {
		return ""
	}
	return " (" + strings.Join(facts, ", ") + ")"
}
