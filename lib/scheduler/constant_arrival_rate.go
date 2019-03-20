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

const constantArrivalRateType = "constant-arrival-rate"

func init() {
	RegisterConfigType(constantArrivalRateType, func(name string, rawJSON []byte) (Config, error) {
		config := NewConstantArrivalRateConfig(name)
		err := strictJSONUnmarshal(rawJSON, &config)
		return config, err
	})
}

// ConstantArrivalRateConfig stores config for the constant arrival-rate scheduler
type ConstantArrivalRateConfig struct {
	BaseConfig
	Rate     null.Int           `json:"rate"`
	TimeUnit types.NullDuration `json:"timeUnit"`
	Duration types.NullDuration `json:"duration"`

	// Initialize `PreAllocatedVUs` number of VUs, and if more than that are needed,
	// they will be dynamically allocated, until `MaxVUs` is reached, which is an
	// absolutely hard limit on the number of VUs the scheduler will use
	PreAllocatedVUs null.Int `json:"preAllocatedVUs"`
	MaxVUs          null.Int `json:"maxVUs"`
}

// NewConstantArrivalRateConfig returns a ConstantArrivalRateConfig with default values
func NewConstantArrivalRateConfig(name string) ConstantArrivalRateConfig {
	return ConstantArrivalRateConfig{
		BaseConfig: NewBaseConfig(name, constantArrivalRateType, false),
		TimeUnit:   types.NewNullDuration(1*time.Second, false),
	}
}

// Make sure we implement the Config interface
var _ Config = &ConstantArrivalRateConfig{}

// Validate makes sure all options are configured and valid
func (carc ConstantArrivalRateConfig) Validate() []error {
	errors := carc.BaseConfig.Validate()
	if !carc.Rate.Valid {
		errors = append(errors, fmt.Errorf("the iteration rate isn't specified"))
	} else if carc.Rate.Int64 <= 0 {
		errors = append(errors, fmt.Errorf("the iteration rate should be more than 0"))
	}

	if time.Duration(carc.TimeUnit.Duration) <= 0 {
		errors = append(errors, fmt.Errorf("the timeUnit should be more than 0"))
	}

	if !carc.Duration.Valid {
		errors = append(errors, fmt.Errorf("the duration is unspecified"))
	} else if time.Duration(carc.Duration.Duration) < minDuration {
		errors = append(errors, fmt.Errorf(
			"the duration should be at least %s, but is %s", minDuration, carc.Duration,
		))
	}

	if !carc.PreAllocatedVUs.Valid {
		errors = append(errors, fmt.Errorf("the number of preAllocatedVUs isn't specified"))
	} else if carc.PreAllocatedVUs.Int64 < 0 {
		errors = append(errors, fmt.Errorf("the number of preAllocatedVUs shouldn't be negative"))
	}

	if !carc.MaxVUs.Valid {
		errors = append(errors, fmt.Errorf("the number of maxVUs isn't specified"))
	} else if carc.MaxVUs.Int64 < carc.PreAllocatedVUs.Int64 {
		errors = append(errors, fmt.Errorf("maxVUs shouldn't be less than preAllocatedVUs"))
	}

	return errors
}

// GetMaxVUs returns the absolute maximum number of possible concurrently running VUs
func (carc ConstantArrivalRateConfig) GetMaxVUs() int64 {
	return carc.MaxVUs.Int64
}

// GetMaxDuration returns the maximum duration time for this scheduler, including
// the specified iterationTimeout, if the iterations are uninterruptible
func (carc ConstantArrivalRateConfig) GetMaxDuration() time.Duration {
	maxDuration := carc.Duration.Duration
	if !carc.Interruptible.Bool {
		maxDuration += carc.IterationTimeout.Duration
	}
	return time.Duration(maxDuration)
}
