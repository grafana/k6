package scheduler

import (
	"fmt"
	"time"

	"github.com/loadimpact/k6/lib/types"
	null "gopkg.in/guregu/null.v3"
)

const sharedIterationsType = "shared-iterations"

// SharedIteationsConfig stores the number of VUs iterations, as well as maxDuration settings
type SharedIteationsConfig struct {
	BaseConfig
	VUs         null.Int           `json:"vus"`
	Iterations  null.Int           `json:"iterations"`
	MaxDuration types.NullDuration `json:"maxDuration"`
}

// NewSharedIterationsConfig returns a SharedIteationsConfig with default values
func NewSharedIterationsConfig(name string) SharedIteationsConfig {
	return SharedIteationsConfig{
		BaseConfig:  NewBaseConfig(name, sharedIterationsType, false),
		MaxDuration: types.NewNullDuration(1*time.Hour, false),
	}
}

// Make sure we implement the Config interface
var _ Config = &SharedIteationsConfig{}

// Validate makes sure all options are configured and valid
func (sic SharedIteationsConfig) Validate() []error {
	errors := sic.BaseConfig.Validate()
	if !sic.VUs.Valid {
		errors = append(errors, fmt.Errorf("the number of VUs isn't specified"))
	} else if sic.VUs.Int64 <= 0 {
		errors = append(errors, fmt.Errorf("the number of VUs should be more than 0"))
	}

	if !sic.Iterations.Valid {
		errors = append(errors, fmt.Errorf("the number of iterations isn't specified"))
	} else if sic.Iterations.Int64 < sic.VUs.Int64 {
		errors = append(errors, fmt.Errorf("the number of iterations shouldn't be less than the number of VUs"))
	}

	if time.Duration(sic.MaxDuration.Duration) < minDuration {
		errors = append(errors, fmt.Errorf(
			"the maxDuration should be at least %s, but is %s", minDuration, sic.MaxDuration,
		))
	}

	return errors
}
