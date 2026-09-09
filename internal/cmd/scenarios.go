package cmd

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"

	"go.k6.io/k6/v2/errext"
	"go.k6.io/k6/v2/errext/exitcodes"
	"go.k6.io/k6/v2/lib"
)

func checkScenarioConflicts(flags *pflag.FlagSet) error {
	if !flags.Changed("scenario") {
		return nil
	}
	for _, name := range []string{"vus", "duration", "iterations", "stage"} {
		if flags.Changed(name) {
			return errext.WithExitCodeIfNone(
				fmt.Errorf("--scenario cannot be combined with --%s", name), exitcodes.InvalidConfig)
		}
	}
	return nil
}

func dropScenarioShortcuts(logger logrus.FieldLogger, layers map[string]*lib.Options) error {
	var unset lib.Options
	for _, name := range []string{"config", "script", "environment"} {
		opts, ok := layers[name]
		if !ok {
			return fmt.Errorf("missing %q layer", name)
		}

		var dropped []string
		if opts.VUs.Valid {
			dropped = append(dropped, "vus")
		}
		if opts.Duration.Valid {
			dropped = append(dropped, "duration")
		}
		if opts.Iterations.Valid {
			dropped = append(dropped, "iterations")
		}
		if opts.Stages != nil {
			dropped = append(dropped, "stages")
		}
		opts.VUs = unset.VUs
		opts.Duration = unset.Duration
		opts.Iterations = unset.Iterations
		opts.Stages = unset.Stages
		if len(dropped) > 0 {
			logger.Warnf("--scenario overrode %s in %q configuration", strings.Join(dropped, ", "), name)
		}
	}
	return nil
}

func selectScenarios(opts lib.Options, names []string) (lib.Options, error) {
	for _, name := range names {
		if _, ok := opts.Scenarios[name]; ok {
			continue
		}

		available := strings.Join(slices.Sorted(maps.Keys(opts.Scenarios)), ", ")
		err := fmt.Errorf("scenario %q not found; available scenarios: %s", name, available)
		if len(opts.Scenarios) == 0 {
			err = fmt.Errorf("scenario %q not found; the script does not have any named scenario", name)
		}

		return opts, errext.WithExitCodeIfNone(err, exitcodes.InvalidConfig)
	}

	opts.Scenarios = maps.Clone(opts.Scenarios)
	maps.DeleteFunc(opts.Scenarios, func(name string, _ lib.ExecutorConfig) bool {
		return !slices.Contains(names, name)
	})

	return opts, nil
}
