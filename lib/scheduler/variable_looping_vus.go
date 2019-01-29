package scheduler

import (
	"fmt"

	"github.com/loadimpact/k6/lib/types"
	null "gopkg.in/guregu/null.v3"
)

const variableLoopingVUsType = "variable-looping-vus"

// Stage contains
type Stage struct {
	Duration types.NullDuration `json:"duration"`
	Target   null.Int           `json:"target"` // TODO: maybe rename this to endVUs?
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
func (ls VariableLoopingVUsConfig) Validate() []error {
	errors := ls.BaseConfig.Validate()
	if ls.StartVUs.Int64 < 0 {
		errors = append(errors, fmt.Errorf("the number of start VUs shouldn't be negative"))
	}

	if len(ls.Stages) == 0 {
		errors = append(errors, fmt.Errorf("at least one stage has to be specified"))
	} else {
		for i, s := range ls.Stages {
			stageNum := i + 1
			if !s.Duration.Valid {
				errors = append(errors, fmt.Errorf("stage %d doesn't have a duration", stageNum))
			} else if s.Duration.Duration < 0 {
				errors = append(errors, fmt.Errorf("the duration for stage %d shouldn't be negative", stageNum))
			}
			if !s.Target.Valid {
				errors = append(errors, fmt.Errorf("stage %d doesn't have a target", stageNum))
			} else if s.Target.Int64 < 0 {
				errors = append(errors, fmt.Errorf("the target for stage %d shouldn't be negative", stageNum))
			}
		}
	}

	return errors
}
