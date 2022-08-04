package executor

import (
	"github.com/sirupsen/logrus"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/types"
)

// ExecutionConflictError is a custom error type used for all of the errors in
// the DeriveScenariosFromShortcuts() function.
type ExecutionConflictError string

func (e ExecutionConflictError) Error() string {
	return string(e)
}

var _ error = ExecutionConflictError("")

func getConstantVUsScenario(duration types.NullDuration, vus null.Int) lib.ScenarioConfigs {
	ds := NewConstantVUsConfig(lib.DefaultScenarioName)
	ds.VUs = vus
	ds.Duration = duration
	return lib.ScenarioConfigs{lib.DefaultScenarioName: ds}
}

func getRampingVUsScenario(stages []lib.Stage, startVUs null.Int) lib.ScenarioConfigs {
	ds := NewRampingVUsConfig(lib.DefaultScenarioName)
	ds.StartVUs = startVUs
	for _, s := range stages {
		if s.Duration.Valid {
			ds.Stages = append(ds.Stages, Stage{Duration: s.Duration, Target: s.Target})
		}
	}
	return lib.ScenarioConfigs{lib.DefaultScenarioName: ds}
}

func getSharedIterationsScenario(iters null.Int, duration types.NullDuration, vus null.Int) lib.ScenarioConfigs {
	ds := NewSharedIterationsConfig(lib.DefaultScenarioName)
	ds.VUs = vus
	ds.Iterations = iters
	if duration.Valid {
		ds.MaxDuration = duration
	}
	return lib.ScenarioConfigs{lib.DefaultScenarioName: ds}
}

// DeriveScenariosFromShortcuts checks for conflicting options and turns any
// shortcut options (i.e. duration, iterations, stages) into the proper
// long-form scenario/executor configuration in the scenarios property.
func DeriveScenariosFromShortcuts(opts lib.Options, logger logrus.FieldLogger) (lib.Options, error) {
	result := opts

	switch {
	case opts.Iterations.Valid:
		if len(opts.Stages) > 0 { // stages isn't nil (not set) and isn't explicitly set to empty
			return result, ExecutionConflictError(
				"using multiple execution config shortcuts (`iterations` and `stages`) simultaneously is not allowed",
			)
		}
		if opts.Scenarios != nil {
			return opts, ExecutionConflictError(
				"using an execution configuration shortcut (`iterations`) and `scenarios` simultaneously is not allowed",
			)
		}
		result.Scenarios = getSharedIterationsScenario(opts.Iterations, opts.Duration, opts.VUs)

	case opts.Duration.Valid:
		if len(opts.Stages) > 0 { // stages isn't nil (not set) and isn't explicitly set to empty
			return result, ExecutionConflictError(
				"using multiple execution config shortcuts (`duration` and `stages`) simultaneously is not allowed",
			)
		}
		if opts.Scenarios != nil {
			return result, ExecutionConflictError(
				"using an execution configuration shortcut (`duration`) and `scenarios` simultaneously is not allowed",
			)
		}
		if opts.Duration.Duration <= 0 {
			// TODO: move this validation to Validate()?
			return result, ExecutionConflictError(
				"`duration` should be more than 0, for infinite duration use the externally-controlled executor",
			)
		}
		result.Scenarios = getConstantVUsScenario(opts.Duration, opts.VUs)

	case len(opts.Stages) > 0: // stages isn't nil (not set) and isn't explicitly set to empty
		if opts.Scenarios != nil {
			return opts, ExecutionConflictError(
				"using an execution configuration shortcut (`stages`) and `scenarios` simultaneously is not allowed",
			)
		}
		result.Scenarios = getRampingVUsScenario(opts.Stages, opts.VUs)

	case len(opts.Scenarios) > 0:
		// Do nothing, scenarios was explicitly specified

	default:
		// Check if we should emit some warnings
		if opts.VUs.Valid && opts.VUs.Int64 != 1 {
			logger.Warnf(
				"the `vus=%d` option will be ignored, it only works in conjunction with `iterations`, `duration`, or `stages`",
				opts.VUs.Int64,
			)
		}
		if opts.Stages != nil && len(opts.Stages) == 0 {
			// No someone explicitly set stages to empty
			logger.Warnf("`stages` was explicitly set to an empty value, running the script with 1 iteration in 1 VU")
		}
		if opts.Scenarios != nil && len(opts.Scenarios) == 0 {
			// No shortcut, and someone explicitly set execution to empty
			logger.Warnf("`scenarios` was explicitly set to an empty value, running the script with 1 iteration in 1 VU")
		}
		// No execution parameters whatsoever were specified, so we'll create a per-VU iterations config
		// with 1 VU and 1 iteration.
		result.Scenarios = lib.ScenarioConfigs{
			lib.DefaultScenarioName: NewPerVUIterationsConfig(lib.DefaultScenarioName),
		}
	}

	// TODO: validate the config; questions:
	// - separately validate the duration, iterations and stages for better error messages?
	// - or reuse the execution validation somehow, at the end? or something mixed?
	// - here or in getConsolidatedConfig() or somewhere else?

	return result, nil
}
