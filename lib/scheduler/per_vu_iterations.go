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

const perVUIterationsType = "per-vu-iterations"

func init() {
	RegisterConfigType(perVUIterationsType, func(name string, rawJSON []byte) (Config, error) {
		config := NewPerVUIterationsConfig(name)
		err := strictJSONUnmarshal(rawJSON, &config)
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
		VUs:         null.NewInt(1, false),
		Iterations:  null.NewInt(1, false),
		MaxDuration: types.NewNullDuration(1*time.Hour, false),
	}
}

// Make sure we implement the Config interface
var _ Config = &PerVUIteationsConfig{}

// Validate makes sure all options are configured and valid
func (pvic PerVUIteationsConfig) Validate() []error {
	errors := pvic.BaseConfig.Validate()
	if pvic.VUs.Int64 <= 0 {
		errors = append(errors, fmt.Errorf("the number of VUs should be more than 0"))
	}

	if pvic.Iterations.Int64 <= 0 {
		errors = append(errors, fmt.Errorf("the number of iterations should be more than 0"))
	}

	if time.Duration(pvic.MaxDuration.Duration) < minDuration {
		errors = append(errors, fmt.Errorf(
			"the maxDuration should be at least %s, but is %s", minDuration, pvic.MaxDuration,
		))
	}

	return errors
}

// GetMaxVUs returns the absolute maximum number of possible concurrently running VUs
func (pvic PerVUIteationsConfig) GetMaxVUs() int64 {
	return pvic.VUs.Int64
}

// GetMaxDuration returns the maximum duration time for this scheduler, including
// the specified iterationTimeout, if the iterations are uninterruptible
func (pvic PerVUIteationsConfig) GetMaxDuration() time.Duration {
	maxDuration := pvic.MaxDuration.Duration
	if !pvic.Interruptible.Bool {
		maxDuration += pvic.IterationTimeout.Duration
	}
	return time.Duration(maxDuration)
}
