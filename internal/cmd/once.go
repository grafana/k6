package cmd

import (
	"encoding/json"
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
func applyOnce(opts lib.Options) (lib.Options, error) {
	scenarios := opts.Scenarios
	if len(scenarios) == 0 {
		scenarios = lib.ScenarioConfigs{lib.DefaultScenarioName: nil}
	}

	rewritten := make(lib.ScenarioConfigs, len(scenarios))
	for name, src := range scenarios {
		sc, err := onceScenario(name, src)
		if err != nil {
			return opts, err
		}
		rewritten[name] = sc
	}
	opts.Scenarios = rewritten

	return opts, nil
}

func onceScenario(name string, src lib.ExecutorConfig) (lib.ExecutorConfig, error) {
	once := executor.NewSharedIterationsConfig(name)
	once.VUs = null.IntFrom(1)
	once.Iterations = null.IntFrom(1)
	if src == nil {
		return once, nil
	}

	// GetExec collapses an unset exec and an explicit "default" to the same "default",
	// and the interface exposes no raw getter, so read the null.String through JSON.
	data, err := json.Marshal(src)
	if err != nil {
		return nil, fmt.Errorf("reading the %q scenario: %w", src.GetName(), err)
	}
	var raw struct {
		Exec null.String `json:"exec"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("reading the exec of the %q scenario: %w", src.GetName(), err)
	}
	once.Exec = raw.Exec
	once.Env = src.GetEnv()
	once.Tags = src.GetTags()

	return once, nil
}
