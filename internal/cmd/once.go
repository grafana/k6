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

// Runs before DeriveScenariosFromShortcuts, so the archive gets the same scenarios.
func applyOnce(opts lib.Options) lib.Options {
	scenarios := opts.Scenarios
	if len(scenarios) == 0 {
		scenarios = lib.ScenarioConfigs{lib.DefaultScenarioName: nil}
	}

	rewritten := make(lib.ScenarioConfigs, len(scenarios))
	for name := range scenarios {
		rewritten[name] = onceScenario(name)
	}
	opts.Scenarios = rewritten

	return opts
}

func onceScenario(name string) lib.ExecutorConfig {
	once := executor.NewSharedIterationsConfig(name)
	once.VUs = null.IntFrom(1)
	once.Iterations = null.IntFrom(1)
	return once
}
