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

const constantLoopingVUsType = "constant-looping-vus"

func init() {
	RegisterConfigType(constantLoopingVUsType, func(name string, rawJSON []byte) (Config, error) {
		config := NewConstantLoopingVUsConfig(name)
		err := strictJSONUnmarshal(rawJSON, &config)
		return config, err
	})
}

// The minimum duration we'll allow users to schedule. This doesn't affect the stages
// configuration, where 0-duration virtual stages are allowed for instantaneous VU jumps
const minDuration = 1 * time.Second

// ConstantLoopingVUsConfig stores VUs and duration
type ConstantLoopingVUsConfig struct {
	BaseConfig
	VUs      null.Int           `json:"vus"`
	Duration types.NullDuration `json:"duration"`
}

// NewConstantLoopingVUsConfig returns a ConstantLoopingVUsConfig with default values
func NewConstantLoopingVUsConfig(name string) ConstantLoopingVUsConfig {
	return ConstantLoopingVUsConfig{
		BaseConfig: NewBaseConfig(name, constantLoopingVUsType, false),
		VUs:        null.NewInt(1, false),
	}
}

// Make sure we implement the Config interface
var _ Config = &ConstantLoopingVUsConfig{}

// Validate makes sure all options are configured and valid
func (lcv ConstantLoopingVUsConfig) Validate() []error {
	errors := lcv.BaseConfig.Validate()
	if lcv.VUs.Int64 <= 0 {
		errors = append(errors, fmt.Errorf("the number of VUs should be more than 0"))
	}

	if !lcv.Duration.Valid {
		errors = append(errors, fmt.Errorf("the duration is unspecified"))
	} else if time.Duration(lcv.Duration.Duration) < minDuration {
		errors = append(errors, fmt.Errorf(
			"the duration should be at least %s, but is %s", minDuration, lcv.Duration,
		))
	}

	return errors
}

// GetMaxVUs returns the absolute maximum number of possible concurrently running VUs
func (lcv ConstantLoopingVUsConfig) GetMaxVUs() int64 {
	return lcv.VUs.Int64
}

// GetMaxDuration returns the maximum duration time for this scheduler, including
// the specified iterationTimeout, if the iterations are uninterruptible
func (lcv ConstantLoopingVUsConfig) GetMaxDuration() time.Duration {
	maxDuration := lcv.Duration.Duration
	if !lcv.Interruptible.Bool {
		maxDuration += lcv.IterationTimeout.Duration
	}
	return time.Duration(maxDuration)
}

// Split divides the VUS as best it can, but keeps the same duration
func (lcv ConstantLoopingVUsConfig) Split(percentages []float64) ([]Config, error) {
	if err := checkPercentagesSum(percentages); err != nil {
		return nil, err
	}
	configs := make([]Config, len(percentages))
	for i, p := range percentages {
		//TODO: figure out a better approach for the proportional distribution
		// of the VUs (which are indivisible items)...
		// Some sort of "pick closest match to percentage and adjust remaining"?
		configs[i] = &ConstantLoopingVUsConfig{
			BaseConfig: *lcv.BaseConfig.CopyWithPercentage(p),
			VUs:        null.IntFrom(int64(float64(lcv.VUs.Int64) / p)),
			Duration:   lcv.Duration,
		}
	}
	return configs, nil
}
