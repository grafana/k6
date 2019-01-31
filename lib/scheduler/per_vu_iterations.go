package scheduler

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/loadimpact/k6/lib/types"
	null "gopkg.in/guregu/null.v3"
)

const perVUIterationsType = "per-vu-iterations"

func init() {
	RegisterConfigType(perVUIterationsType, func(name string, rawJSON []byte) (Config, error) {
		config := NewPerVUIterationsConfig(name)
		err := json.Unmarshal(rawJSON, &config)
		return config, err
	})
}

// PerVUIteationsConfig stores the number of VUs iterations, as well as maxDuration settings
type PerVUIteationsConfig struct {
	BaseConfig
	VUs         null.Int           `json:"vus"`
	Iterations  null.Int           `json:"iterations"`
	MaxDuration types.NullDuration `json:"maxDuration"`
}

// NewPerVUIterationsConfig returns a PerVUIteationsConfig with default values
func NewPerVUIterationsConfig(name string) PerVUIteationsConfig {
	return PerVUIteationsConfig{
		BaseConfig:  NewBaseConfig(name, perVUIterationsType, false),
		MaxDuration: types.NewNullDuration(1*time.Hour, false),
	}
}

// Make sure we implement the Config interface
var _ Config = &PerVUIteationsConfig{}

// Validate makes sure all options are configured and valid
func (pvic PerVUIteationsConfig) Validate() []error {
	errors := pvic.BaseConfig.Validate()
	if !pvic.VUs.Valid {
		errors = append(errors, fmt.Errorf("the number of VUs isn't specified"))
	} else if pvic.VUs.Int64 <= 0 {
		errors = append(errors, fmt.Errorf("the number of VUs should be more than 0"))
	}

	if !pvic.Iterations.Valid {
		errors = append(errors, fmt.Errorf("the number of iterations isn't specified"))
	} else if pvic.Iterations.Int64 <= 0 {
		errors = append(errors, fmt.Errorf("the number of iterations should be more than 0"))
	}

	if time.Duration(pvic.MaxDuration.Duration) < minDuration {
		errors = append(errors, fmt.Errorf(
			"the maxDuration should be at least %s, but is %s", minDuration, pvic.MaxDuration,
		))
	}

	return errors
}
