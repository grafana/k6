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

const sharedIterationsType = "shared-iterations"

func init() {
	RegisterConfigType(sharedIterationsType, func(name string, rawJSON []byte) (Config, error) {
		config := NewSharedIterationsConfig(name)
		err := strictJSONUnmarshal(rawJSON, &config)
		return config, err
	})
}

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
		VUs:         null.NewInt(1, false),
		Iterations:  null.NewInt(1, false),
		MaxDuration: types.NewNullDuration(1*time.Hour, false),
	}
}

// Make sure we implement the Config interface
var _ Config = &SharedIteationsConfig{}

// Validate makes sure all options are configured and valid
func (sic SharedIteationsConfig) Validate() []error {
	errors := sic.BaseConfig.Validate()
	if sic.VUs.Int64 <= 0 {
		errors = append(errors, fmt.Errorf("the number of VUs should be more than 0"))
	}

	if sic.Iterations.Int64 < sic.VUs.Int64 {
		errors = append(errors, fmt.Errorf(
			"the number of iterations (%d) shouldn't be less than the number of VUs (%d)",
			sic.Iterations.Int64, sic.VUs.Int64,
		))
	}

	if time.Duration(sic.MaxDuration.Duration) < minDuration {
		errors = append(errors, fmt.Errorf(
			"the maxDuration should be at least %s, but is %s", minDuration, sic.MaxDuration,
		))
	}

	return errors
}

// GetMaxVUs returns the absolute maximum number of possible concurrently running VUs
func (sic SharedIteationsConfig) GetMaxVUs() int64 {
	return sic.VUs.Int64
}

// GetMaxDuration returns the maximum duration time for this scheduler, including
// the specified iterationTimeout, if the iterations are uninterruptible
func (sic SharedIteationsConfig) GetMaxDuration() time.Duration {
	maxDuration := sic.MaxDuration.Duration
	if !sic.Interruptible.Bool {
		maxDuration += sic.IterationTimeout.Duration
	}
	return time.Duration(maxDuration)
}
