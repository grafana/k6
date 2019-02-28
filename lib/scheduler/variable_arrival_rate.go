/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package scheduler

import (
	"fmt"
	"time"

	"github.com/loadimpact/k6/lib/types"
	null "gopkg.in/guregu/null.v3"
)

const variableArrivalRateType = "variable-arrival-rate"

func init() {
	RegisterConfigType(variableArrivalRateType, func(name string, rawJSON []byte) (Config, error) {
		config := NewVariableArrivalRateConfig(name)
		err := strictJSONUnmarshal(rawJSON, &config)
		return config, err
	})
}

// VariableArrivalRateConfig stores config for the variable arrival-rate scheduler
type VariableArrivalRateConfig struct {
	BaseConfig
	StartRate null.Int           `json:"startRate"`
	TimeUnit  types.NullDuration `json:"timeUnit"`
	Stages    []Stage            `json:"stages"`

	// Initialize `PreAllocatedVUs` number of VUs, and if more than that are needed,
	// they will be dynamically allocated, until `MaxVUs` is reached, which is an
	// absolutely hard limit on the number of VUs the scheduler will use
	PreAllocatedVUs null.Int `json:"preAllocatedVUs"`
	MaxVUs          null.Int `json:"maxVUs"`
}

// NewVariableArrivalRateConfig returns a VariableArrivalRateConfig with default values
func NewVariableArrivalRateConfig(name string) VariableArrivalRateConfig {
	return VariableArrivalRateConfig{
		BaseConfig: NewBaseConfig(name, variableArrivalRateType, false),
		TimeUnit:   types.NewNullDuration(1*time.Second, false),
	}
}

// Make sure we implement the Config interface
var _ Config = &VariableArrivalRateConfig{}

// Validate makes sure all options are configured and valid
func (varc VariableArrivalRateConfig) Validate() []error {
	errors := varc.BaseConfig.Validate()

	if varc.StartRate.Int64 < 0 {
		errors = append(errors, fmt.Errorf("the startRate value shouldn't be negative"))
	}

	if time.Duration(varc.TimeUnit.Duration) < 0 {
		errors = append(errors, fmt.Errorf("the timeUnit should be more than 0"))
	}

	errors = append(errors, validateStages(varc.Stages)...)

	if !varc.PreAllocatedVUs.Valid {
		errors = append(errors, fmt.Errorf("the number of preAllocatedVUs isn't specified"))
	} else if varc.PreAllocatedVUs.Int64 < 0 {
		errors = append(errors, fmt.Errorf("the number of preAllocatedVUs shouldn't be negative"))
	}

	if !varc.MaxVUs.Valid {
		errors = append(errors, fmt.Errorf("the number of maxVUs isn't specified"))
	} else if varc.MaxVUs.Int64 < varc.PreAllocatedVUs.Int64 {
		errors = append(errors, fmt.Errorf("maxVUs shouldn't be less than preAllocatedVUs"))
	}

	return errors
}

// GetMaxVUs returns the absolute maximum number of possible concurrently running VUs
func (varc VariableArrivalRateConfig) GetMaxVUs() int64 {
	return varc.MaxVUs.Int64
}

// GetMaxDuration returns the maximum duration time for this scheduler, including
// the specified iterationTimeout, if the iterations are uninterruptible
func (varc VariableArrivalRateConfig) GetMaxDuration() time.Duration {
	var maxDuration types.Duration
	for _, s := range varc.Stages {
		maxDuration += s.Duration.Duration
	}
	if !varc.Interruptible.Bool {
		maxDuration += varc.IterationTimeout.Duration
	}
	return time.Duration(maxDuration)
}
