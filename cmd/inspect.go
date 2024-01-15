package cmd

import (
	"encoding/json"

	"github.com/spf13/cobra"

	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/types"
)

// TODO: split apart like `k6 run` and `k6 archive`
func getCmdInspect(gs *state.GlobalState) *cobra.Command {
	var addExecReqs bool

	// inspectCmd represents the inspect command
	inspectCmd := &cobra.Command{
		Use:   "inspect [file]",
		Short: "Inspect a script or archive",
		Long:  `Inspect a script or archive.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			test, err := loadLocalTest(gs, cmd, args)
			if err != nil {
				return err
			}

			// At the moment, `k6 inspect` output can take 2 forms: standard
			// (equal to the lib.Options struct) and extended, with additional
			// fields with execution requirements.
			var inspectOutput interface{}
			if addExecReqs {
				inspectOutput, err = inspectOutputWithExecRequirements(gs, cmd, test)
				if err != nil {
					return err
				}
			} else {
				inspectOutput = test.initRunner.GetOptions()
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
	inspectCmd.Flags().BoolVar(&addExecReqs,
		"execution-requirements",
		false,
		"include calculations of execution requirements for the test")

	return inspectCmd
}

// If --execution-requirements is enabled, this will consolidate the config,
// derive the value of `scenarios` and calculate the max test duration and VUs.
func inspectOutputWithExecRequirements(
	gs *state.GlobalState, cmd *cobra.Command, test *loadedTest,
) (interface{}, error) {
	// we don't actually support CLI flags here, so we pass nil as the getter
	configuredTest, err := test.consolidateDeriveAndValidateConfig(gs, cmd, nil)
	if err != nil {
		return nil, err
	}

	et, err := lib.NewExecutionTuple(
		configuredTest.derivedConfig.ExecutionSegment,
		configuredTest.derivedConfig.ExecutionSegmentSequence,
	)
	if err != nil {
		return nil, err
	}

	executionPlan := configuredTest.derivedConfig.Scenarios.GetFullExecutionRequirements(et)
	duration, _ := lib.GetEndOffset(executionPlan)

	return struct {
		lib.Options
		TotalDuration types.NullDuration `json:"totalDuration"`
		MaxVUs        uint64             `json:"maxVUs"`
	}{
		configuredTest.derivedConfig.Options,
		types.NewNullDuration(duration, true),
		lib.GetMaxPossibleVUs(executionPlan),
	}, nil
}
