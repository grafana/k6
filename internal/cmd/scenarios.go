package cmd

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"go.k6.io/k6/v2/errext"
	"go.k6.io/k6/v2/errext/exitcodes"
	"go.k6.io/k6/v2/lib"
)

func selectScenarios(opts lib.Options, names []string) (lib.Options, error) {
	for _, name := range names {
		if _, ok := opts.Scenarios[name]; ok {
			continue
		}

		available := strings.Join(slices.Sorted(maps.Keys(opts.Scenarios)), ", ")
		err := fmt.Errorf("scenario %q not found; available scenarios: %s", name, available)

		return opts, errext.WithExitCodeIfNone(err, exitcodes.InvalidConfig)
	}

	opts.Scenarios = maps.Clone(opts.Scenarios)
	maps.DeleteFunc(opts.Scenarios, func(name string, _ lib.ExecutorConfig) bool {
		return !slices.Contains(names, name)
	})

	return opts, nil
}
