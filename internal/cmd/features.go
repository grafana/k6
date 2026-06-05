package cmd

import (
	"encoding/json"
	"fmt"
	"maps"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"go.k6.io/k6/v2/cmd/state"
	"go.k6.io/k6/v2/errext"
	"go.k6.io/k6/v2/errext/exitcodes"
	"go.k6.io/k6/v2/internal/features"
	"go.k6.io/k6/v2/lib"
)

type featuresCmd struct {
	gs     *state.GlobalState
	isJSON bool
}

func getCmdFeatures(gs *state.GlobalState) *cobra.Command {
	c := &featuresCmd{gs: gs}

	cmd := &cobra.Command{
		Use:   "features",
		Short: "List available feature flags",
		Long:  `List all available feature flags with their lifecycle status and description.`,
		Args:  cobra.NoArgs,
		RunE:  c.run,
	}

	cmd.Flags().BoolVar(&c.isJSON, "json", false, "output in JSON format")

	return cmd
}

func (c *featuresCmd) run(_ *cobra.Command, _ []string) error {
	all, err := features.All()
	if err != nil {
		return fmt.Errorf("loading feature definitions: %w", err)
	}

	if c.isJSON {
		return c.printJSON(all)
	}
	return c.printTable(all)
}

func (c *featuresCmd) printTable(flags []features.Flag) error {
	if len(flags) == 0 {
		return nil
	}

	w := tabwriter.NewWriter(c.gs.Stdout, 0, 0, 3, ' ', 0)
	if _, err := fmt.Fprintln(w, "FEATURE\tLIFECYCLE\tDESCRIPTION"); err != nil {
		return err
	}
	for _, f := range flags {
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\n", f.Name, f.Lifecycle.String(), f.Description); err != nil {
			return err
		}
	}
	return w.Flush()
}

func (c *featuresCmd) printJSON(flags []features.Flag) error {
	type jsonFlag struct {
		Feature     string `json:"feature"`
		Lifecycle   string `json:"lifecycle"`
		Description string `json:"description"`
	}

	out := make([]jsonFlag, len(flags))
	for i, f := range flags {
		out[i] = jsonFlag{
			Feature:     f.Name,
			Lifecycle:   f.Lifecycle.String(),
			Description: f.Description,
		}
	}

	enc := json.NewEncoder(c.gs.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

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
