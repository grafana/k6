package scheduler

import (
	"fmt"
	"time"

	"github.com/loadimpact/k6/lib/types"
	null "gopkg.in/guregu/null.v3"
)

const variableLoopingVUsType = "variable-looping-vus"

func init() {
	RegisterConfigType(variableLoopingVUsType, func(name string, rawJSON []byte) (Config, error) {
		config := NewVariableLoopingVUsConfig(name)
		err := strictJSONUnmarshal(rawJSON, &config)
		return config, err
	})
}

// Stage contains
type Stage struct {
	Duration types.NullDuration `json:"duration"`
	Target   null.Int           `json:"target"` // TODO: maybe rename this to endVUs? something else?
}

// VariableLoopingVUsConfig stores the configuration for the stages scheduler
type VariableLoopingVUsConfig struct {
	BaseConfig
	StartVUs null.Int `json:"startVUs"`
	Stages   []Stage  `json:"stages"`
}

// NewVariableLoopingVUsConfig returns a VariableLoopingVUsConfig with its default values
func NewVariableLoopingVUsConfig(name string) VariableLoopingVUsConfig {
	return VariableLoopingVUsConfig{BaseConfig: NewBaseConfig(name, variableLoopingVUsType, false)}
}

// Make sure we implement the Config interface
var _ Config = &VariableLoopingVUsConfig{}

// Validate makes sure all options are configured and valid
func (vlvc VariableLoopingVUsConfig) Validate() []error {
	errors := vlvc.BaseConfig.Validate()
	if vlvc.StartVUs.Int64 < 0 {
		errors = append(errors, fmt.Errorf("the number of start VUs shouldn't be negative"))
	}

	return append(errors, validateStages(vlvc.Stages)...)
}

// GetMaxVUs returns the absolute maximum number of possible concurrently running VUs
func (vlvc VariableLoopingVUsConfig) GetMaxVUs() int64 {
	maxVUs := vlvc.StartVUs.Int64
	for _, s := range vlvc.Stages {
		if s.Target.Int64 > maxVUs {
			maxVUs = s.Target.Int64
		}
	}
	return maxVUs
}

// GetMaxDuration returns the maximum duration time for this scheduler, including
// the specified iterationTimeout, if the iterations are uninterruptible
func (vlvc VariableLoopingVUsConfig) GetMaxDuration() time.Duration {
	var maxDuration types.Duration
	for _, s := range vlvc.Stages {
		maxDuration += s.Duration.Duration
	}
	if !vlvc.Interruptible.Bool {
		maxDuration += vlvc.IterationTimeout.Duration
	}
	return time.Duration(maxDuration)
}
