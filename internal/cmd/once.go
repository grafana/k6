package cmd

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/v2/errext"
	"go.k6.io/k6/v2/errext/exitcodes"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/lib/executor"
)

// Bare --once never guesses which scenario runs, so several scenarios are ambiguous.
func rejectAmbiguousOnce(scenarios lib.ScenarioConfigs) error {
	if len(scenarios) <= 1 {
		return nil
	}
	names := slices.Sorted(maps.Keys(scenarios))
	return errext.WithExitCodeIfNone(
		fmt.Errorf("--once can run only with a single scenario, but got: %s",
			strings.Join(names, ", ")), exitcodes.InvalidConfig)
}

// Runs before DeriveScenariosFromShortcuts, so the archive gets the same scenario.
func applyOnce(opts lib.Options) lib.Options {
	name := lib.DefaultScenarioName

	once := executor.NewSharedIterationsConfig(name)
	once.VUs = null.IntFrom(1)
	once.Iterations = null.IntFrom(1)

	opts.Scenarios = lib.ScenarioConfigs{name: once}

	return opts
}
