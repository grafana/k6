package scheduler

import (
	"fmt"
	"time"

	"github.com/loadimpact/k6/lib/types"
	null "gopkg.in/guregu/null.v3"
)

const minPercentage = 0.01

// The maximum time k6 will wait after an iteration is supposed to be done
const maxIterationTimeout = 300 * time.Second

// BaseConfig contains the common config fields for all schedulers
type BaseConfig struct {
	Name             string             `json:"-"` // set via the JS object key
	Type             string             `json:"type"`
	StartTime        types.NullDuration `json:"startTime"`
	Interruptible    null.Bool          `json:"interruptible"`
	IterationTimeout types.NullDuration `json:"iterationTimeout"`
	Env              map[string]string  `json:"env"`
	Exec             string             `json:"exec"` // function name, externally validated
	Percentage       float64            `json:"-"`    // 100, unless Split() was called

	// Future extensions: tags, distribution, others?
}

// Make sure we implement the Config interface, even with the BaseConfig!
var _ Config = &BaseConfig{}

// NewBaseConfig returns a default base config with the default values
func NewBaseConfig(name, configType string, interruptible bool) BaseConfig {
	return BaseConfig{
		Name:             name,
		Type:             configType,
		Interruptible:    null.NewBool(interruptible, false),
		IterationTimeout: types.NewNullDuration(30*time.Second, false),
		Percentage:       100,
	}
}

// Validate checks some basic things like present name, type, and a positive start time
func (bc BaseConfig) Validate() (errors []error) {
	// Some just-in-case checks, since those things are likely checked in other places or
	// even assigned by us:
	if bc.Name == "" {
		errors = append(errors, fmt.Errorf("scheduler name shouldn't be empty"))
	}
	if bc.Type == "" {
		errors = append(errors, fmt.Errorf("missing or empty type field"))
	}
	if bc.Percentage < minPercentage || bc.Percentage > 100 {
		errors = append(errors, fmt.Errorf(
			"percentage should be between %f and 100, but is %f", minPercentage, bc.Percentage,
		))
	}
	// The actually reasonable checks:
	if bc.StartTime.Valid && bc.StartTime.Duration < 0 {
		errors = append(errors, fmt.Errorf("scheduler start time should be positive"))
	}
	iterTimeout := time.Duration(bc.IterationTimeout.Duration)
	if iterTimeout < 0 || iterTimeout > maxIterationTimeout {
		errors = append(errors, fmt.Errorf(
			"the iteration timeout should be between 0 and %s, but is %s", maxIterationTimeout, iterTimeout,
		))
	}
	return errors
}

// GetBaseConfig just returns itself
func (bc BaseConfig) GetBaseConfig() BaseConfig {
	return bc
}

// CopyWithPercentage is a helper function that just sets the percentage to
// the specified amount.
func (bc BaseConfig) CopyWithPercentage(percentage float64) *BaseConfig {
	c := bc
	c.Percentage = percentage
	return &c
}

// Split splits the BaseConfig with the accurate percentages
func (bc BaseConfig) Split(percentages []float64) ([]Config, error) {
	if err := checkPercentagesSum(percentages); err != nil {
		return nil, err
	}
	configs := make([]Config, len(percentages))
	for i, p := range percentages {
		configs[i] = bc.CopyWithPercentage(p)
	}
	return configs, nil
}
