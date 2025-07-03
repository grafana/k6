package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/fsext"
	"go.k6.io/k6/lib/types"
)

// TODO: split apart like `k6 run` and `k6 archive`
func getCmdInspect(gs *state.GlobalState) *cobra.Command {
	var addExecReqs bool
	var policyPath string

	// inspectCmd represents the inspect command
	inspectCmd := &cobra.Command{
		Use:   "inspect [file]",
		Short: "Inspect a script or archive",
		Long: `Inspect a script or archive.

If a k6policy.json file is found in the same directory as the script, 
policy validation will be performed automatically. Use --policy to 
specify a different policy file.

Policy validation checks for:
- Required thresholds
- Required tags  
- Disallowed strings (exact matches) in script content
- Disallowed regex patterns in script content

Exit codes:
- 0: Inspection successful, no policy violations
- 1: Policy violations found (when policy checking is enabled)
- 2: Other errors (file not found, parsing errors, etc.)`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			test, err := loadLocalTest(gs, cmd, args)
			if err != nil {
				return err
			}

			// Read script content for policy checking
			scriptPath := args[0]
			scriptContent, err := fsext.ReadFile(gs.FS, scriptPath)
			if err != nil {
				return fmt.Errorf("failed to read script content: %w", err)
			}

			// Perform policy checking
			policyChecker := NewPolicyChecker(gs.FS)
			policyResult, err := policyChecker.CheckPolicy(scriptPath, policyPath, test.initRunner.GetOptions(), string(scriptContent))
			if err != nil {
				return err
			}

			// Print policy results first
			if err := PrintPolicyResult(policyResult, gs.Stdout); err != nil {
				return err
			}

			// Print separator if policy was used
			if policyResult.Used {
				printToStdout(gs, "") // Empty line for separation
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

			// Exit with non-zero code if policy violations found
			if policyResult.Used && len(policyResult.Violations) > 0 {
				os.Exit(1)
			}

			return nil
		},
	}

	inspectCmd.Flags().SortFlags = false
	inspectCmd.Flags().AddFlagSet(runtimeOptionFlagSet(false))
	inspectCmd.Flags().BoolVar(&addExecReqs,
		"execution-requirements",
		false,
		"include calculations of execution requirements for the test")
	inspectCmd.Flags().StringVar(&policyPath,
		"policy",
		"",
		"path to policy file (k6policy.json) for validation. If not specified, will look for k6policy.json in script directory")

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
