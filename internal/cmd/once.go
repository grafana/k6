package cmd

import (
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/lib/executor"
)

// Runs before DeriveScenariosFromShortcuts, so the archive gets the same scenario.
func applyOnce(opts lib.Options) lib.Options {
	name := lib.DefaultScenarioName

	once := executor.NewSharedIterationsConfig(name)
	once.VUs = null.IntFrom(1)
	once.Iterations = null.IntFrom(1)

	opts.Scenarios = lib.ScenarioConfigs{name: once}

	return opts
}
