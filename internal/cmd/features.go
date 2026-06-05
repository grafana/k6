package cmd

import (
	"fmt"
	"maps"
	"strings"

	"github.com/spf13/cobra"

	"go.k6.io/k6/v2/cmd/state"
	"go.k6.io/k6/v2/errext"
	"go.k6.io/k6/v2/errext/exitcodes"
	"go.k6.io/k6/v2/internal/features"
	"go.k6.io/k6/v2/lib"
)

func resolveFeatureFlags(gs *state.GlobalState, cmd *cobra.Command, piState *lib.TestPreInitState) error {
	var cli features.Source
	if cmd.Flags().Lookup("features") != nil {
		cli.Supplied = cmd.Flags().Changed("features")
		vals, err := cmd.Flags().GetStringArray("features")
		if err != nil {
			return fmt.Errorf("reading --features flag: %w", err)
		}
		cli.Values = vals
	}

	fileConf, err := readDiskConfig(gs)
	if err != nil {
		return errext.WithExitCodeIfNone(
			fmt.Errorf("failed to load the configuration file from the local file system: %w", err),
			exitcodes.InvalidConfig,
		)
	}
	jsonSurface := features.Source{Values: fileConf.Features, Supplied: fileConf.Features != nil}

	envSurface := maps.Clone(gs.Env)
	if envSurface == nil {
		envSurface = make(map[string]string)
	}
	maps.Copy(envSurface, piState.RuntimeOptions.Env)

	flags, err := features.Init(gs.Logger, cli, jsonSurface, envSurface)
	if err != nil {
		return fmt.Errorf("initializing feature flags: %w", err)
	}
	piState.FeatureFlags = flags

	for _, name := range flags.Activated() {
		if err := gs.Usage.Strings("features", name); err != nil {
			gs.Logger.WithError(err).Debugf("reporting feature usage for %q", name)
		}
	}

	propagateFeatureEnv(piState.RuntimeOptions.Env, flags.Activated())
	return nil
}

// applyFeatureRunTags rewrites RunTags via a fresh map (consolidated shares the original, must not mutate).
func applyFeatureRunTags(opts *lib.Options, tags map[string]string) {
	reconciled := make(map[string]string, len(opts.RunTags)+len(tags))
	for k, v := range opts.RunTags {
		if !strings.HasPrefix(k, "k6_feature_") {
			reconciled[k] = v
		}
	}
	maps.Copy(reconciled, tags)

	if len(reconciled) == 0 {
		opts.RunTags = nil
		return
	}
	opts.RunTags = reconciled
}

func propagateFeatureEnv(env map[string]string, activated []string) {
	if len(activated) == 0 {
		delete(env, "K6_FEATURES")
		return
	}
	env["K6_FEATURES"] = strings.Join(activated, ",")
}
