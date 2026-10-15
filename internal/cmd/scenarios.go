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
	"go.k6.io/k6/v2/metrics"
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
	selected := make(lib.ScenarioConfigs, len(names))
	for _, name := range names {
		if sc, ok := opts.Scenarios[name]; ok {
			selected[name] = sc
			continue
		}

		available := strings.Join(slices.Sorted(maps.Keys(opts.Scenarios)), ", ")
		err := fmt.Errorf("scenario %q not found; available scenarios: %s", name, available)
		if len(opts.Scenarios) == 0 {
			err = fmt.Errorf("scenario %q not found; the script does not have any named scenario", name)
		}

		return opts, errext.WithExitCodeIfNone(err, exitcodes.InvalidConfig)
	}

	opts.Scenarios = selected

	return opts, nil
}

func dropScenarioThresholds(logger logrus.FieldLogger, opts *lib.Options, configured lib.ScenarioConfigs) {
	opts.Thresholds = maps.Clone(opts.Thresholds)
	var dropped []string
	for metricName := range opts.Thresholds {
		_, tags, err := metrics.ParseMetricName(metricName)
		if err != nil {
			// Only --no-thresholds permits malformed filters past threshold validation.
			continue
		}
		// Match AddSubmetric's normalization and last-value-wins handling of repeated tags.
		for _, tag := range slices.Backward(tags) {
			key, value, _ := strings.Cut(tag, ":")
			if strings.Trim(strings.TrimSpace(key), `"'`) != "scenario" {
				continue
			}
			name := strings.Trim(strings.TrimSpace(value), `"'`)
			_, exists := configured[name]
			_, selected := opts.Scenarios[name]
			if exists && !selected {
				delete(opts.Thresholds, metricName)
				dropped = append(dropped, metricName)
			}
			break
		}
	}
	if len(dropped) > 0 {
		slices.Sort(dropped)
		logger.Warnf("--scenario skipped thresholds for excluded scenarios: %s; "+
			"these thresholds remain skipped even if another scenario emits matching tags", strings.Join(dropped, "; "))
	}
}
