package cmd

import (
	"encoding/json"

	"github.com/spf13/cobra"

	"go.k6.io/k6/v2/cmd/state"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/lib/types"
)

// TODO: split apart like `k6 run` and `k6 archive`
func getCmdInspect(gs *state.GlobalState) *cobra.Command {
	var addExecReqs bool

	// inspectCmd represents the inspect command
	inspectCmd := &cobra.Command{
		Use:   "inspect [file]",
		Short: "Inspect a script or archive",
		Long:  `Inspect a script or archive.`,
		Args: func(cmd *cobra.Command, args []string) error {
			// The options file mode does not read the script, so the argument is optional.
			if _, ok := optionsFilePath(cmd.Flags()); ok {
				return cobra.RangeArgs(0, 1)(cmd, args)
			}
			return cobra.ExactArgs(1)(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var inspectOutput any
			var err error
			if path, ok := optionsFilePath(cmd.Flags()); ok {
				inspectOutput, err = inspectOptionsFile(gs, cmd, path, addExecReqs)
			} else {
				inspectOutput, err = inspectTest(gs, cmd, args, addExecReqs)
			}
			if err != nil {
				return err
			}

			data, err := json.MarshalIndent(inspectOutput, "", "  ")
			if err != nil {
				return err
			}
			printToStdout(gs, string(data))

			return nil
		},
	}

	inspectCmd.Flags().SortFlags = false
	inspectCmd.Flags().AddFlagSet(runtimeOptionFlagSet(false))
	inspectCmd.Flags().AddFlagSet(optionsFileFlagSet())
	inspectCmd.Flags().BoolVar(&addExecReqs,
		"execution-requirements",
		false,
		"include calculations of execution requirements for the test")

	return inspectCmd
}

func inspectTest(gs *state.GlobalState, cmd *cobra.Command, args []string, addExecReqs bool) (any, error) {
	test, err := loadLocalTest(gs, cmd, args)
	if err != nil {
		return nil, err
	}

	// At the moment, `k6 inspect` output can take 2 forms: standard
	// (equal to the lib.Options struct) and extended, with additional
	// fields with execution requirements.
	if !addExecReqs {
		return test.initRunner.GetOptions(), nil
	}

	// If --execution-requirements is enabled, this will consolidate the config,
	// derive the value of `scenarios` and calculate the max test duration and VUs.
	// we don't actually support CLI flags here, so we pass nil as the getter
	configuredTest, err := test.consolidateDeriveAndValidateConfig(gs, cmd, nil)
	if err != nil {
		return nil, err
	}
	return execRequirementsOutput(configuredTest.derivedConfig.Options)
}

// inspectOptionsFile inspects the options from --options-file without loading the test, so
// the script is not read and no JS runs. Without init, it cannot check that a scenario's
// exec is a callable export, or that a threshold's metric exists, so it does not.
func inspectOptionsFile(gs *state.GlobalState, cmd *cobra.Command, path string, addExecReqs bool) (any, error) {
	opts, err := readOptionsFile(gs, path)
	if err != nil {
		return nil, err
	}

	runtimeOptions, err := getRuntimeOptions(cmd.Flags(), gs.Env)
	if err != nil {
		return nil, err
	}
	featureFlags, err := initFeatureFlags(gs, cmd, runtimeOptions.Env)
	if err != nil {
		return nil, err
	}
	warnOnOptionsFeatures(gs.Logger, opts, optionsFileLayerName)

	if !addExecReqs {
		return opts, nil
	}

	// As above, CLI option flags are not supported, so there is no getter.
	_, derivedConfig, err := consolidateDeriveAndValidate(gs, cmd, nil, testConfigInputs{
		options:      opts,
		layerName:    optionsFileLayerName,
		isExecutable: func(string) bool { return true },
		registry:     nil,
		noThresholds: runtimeOptions.NoThresholds.Bool,
		featureFlags: featureFlags,
	})
	if err != nil {
		return nil, err
	}
	return execRequirementsOutput(derivedConfig.Options)
}

// execRequirementsOutput adds the max test duration and VUs to the derived options.
func execRequirementsOutput(derived lib.Options) (any, error) {
	et, err := lib.NewExecutionTuple(derived.ExecutionSegment, derived.ExecutionSegmentSequence)
	if err != nil {
		return nil, err
	}

	executionPlan := derived.Scenarios.GetFullExecutionRequirements(et)
	duration, _ := lib.GetEndOffset(executionPlan)

	return struct {
		lib.Options
		TotalDuration types.NullDuration `json:"totalDuration"`
		MaxVUs        uint64             `json:"maxVUs"`
	}{
		derived,
		types.NewNullDuration(duration, true),
		lib.GetMaxPossibleVUs(executionPlan),
	}, nil
}
