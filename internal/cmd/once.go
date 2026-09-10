package cmd

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/v2/errext"
	"go.k6.io/k6/v2/errext/exitcodes"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/lib/executor"
)

// --once writes the load itself, so a CLI shortcut would fight the scenario it writes.
func checkOnceConflicts(flags *pflag.FlagSet) error {
	if !getNullBool(flags, "once").Bool {
		return nil
	}
	for _, name := range []string{
		"vus", "duration", "iterations", "stage",
		"execution-segment", "execution-segment-sequence",
	} {
		if flags.Changed(name) {
			return errext.WithExitCodeIfNone(
				fmt.Errorf("--once cannot be combined with --%s", name), exitcodes.InvalidConfig)
		}
	}
	return nil
}

// A shortcut left in a lower layer drops or rewrites the scenario applyOnce writes.
func dropOnceShortcuts(logger logrus.FieldLogger, layers map[string]*lib.Options) error {
	var unset lib.Options

	// k6 merges the layers in this order, and the warning follows it.
	for _, name := range []string{"config", "script", "environment"} {
		o, ok := layers[name]
		if !ok {
			return fmt.Errorf("missing %q layer", name)
		}

		var dropped []string
		drop := func(opt string, set bool) {
			if set {
				dropped = append(dropped, opt)
			}
		}
		drop("vus", o.VUs.Valid)
		drop("duration", o.Duration.Valid)
		drop("iterations", o.Iterations.Valid)
		drop("stages", o.Stages != nil)
		drop("executionSegment", o.ExecutionSegment != nil)
		drop("executionSegmentSequence", o.ExecutionSegmentSequence != nil)

		// Clearing an unset shortcut changes nothing, so only the naming above needs a check.
		o.VUs = unset.VUs
		o.Duration = unset.Duration
		o.Iterations = unset.Iterations
		o.Stages = unset.Stages
		o.ExecutionSegment = unset.ExecutionSegment
		o.ExecutionSegmentSequence = unset.ExecutionSegmentSequence

		if len(dropped) > 0 {
			logger.Warnf("--once overrode %s in %q configuration", strings.Join(dropped, ", "), name)
		}
	}

	return nil
}

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
	once.Options = src.GetScenarioOptions()

	return once, nil
}
