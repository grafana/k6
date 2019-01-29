package scheduler

import (
	"fmt"
	"time"

	"github.com/loadimpact/k6/lib/types"
	null "gopkg.in/guregu/null.v3"
)

const constantLoopingVUsType = "constant-looping-vus"

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
	//TODO: decide if we want interruptible or uninterruptible iterations here?
	return ConstantLoopingVUsConfig{BaseConfig: NewBaseConfig(name, constantLoopingVUsType, false)}
}

// Make sure we implement the Config interface
var _ Config = &ConstantLoopingVUsConfig{}

// Validate makes sure all options are configured and valid
func (lcv ConstantLoopingVUsConfig) Validate() []error {
	errors := lcv.BaseConfig.Validate()
	if !lcv.VUs.Valid {
		errors = append(errors, fmt.Errorf("the number of VUs isn't specified"))
	} else if lcv.VUs.Int64 < 0 {
		errors = append(errors, fmt.Errorf("the number of VUs shouldn't be negative"))
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
