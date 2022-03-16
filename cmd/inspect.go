/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package cmd

import (
	"encoding/json"

	"github.com/spf13/cobra"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/types"
)

// TODO: split apart like `k6 run` and `k6 archive`
func getCmdInspect(gs *globalState) *cobra.Command {
	var addExecReqs bool

	// inspectCmd represents the inspect command
	inspectCmd := &cobra.Command{
		Use:   "inspect [file]",
		Short: "Inspect a script or archive",
		Long:  `Inspect a script or archive.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			test, err := loadAndConfigureTest(gs, cmd, args, nil)
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
func inspectOutputWithExecRequirements(gs *globalState, cmd *cobra.Command, test *loadedTest) (interface{}, error) {
	// we don't actually support CLI flags here, so we pass nil as the getter
	if err := test.consolidateDeriveAndValidateConfig(gs, cmd, nil); err != nil {
		return nil, err
	}

	et, err := lib.NewExecutionTuple(test.derivedConfig.ExecutionSegment, test.derivedConfig.ExecutionSegmentSequence)
	if err != nil {
		return nil, err
	}

	executionPlan := test.derivedConfig.Scenarios.GetFullExecutionRequirements(et)
	duration, _ := lib.GetEndOffset(executionPlan)

	return struct {
		lib.Options
		TotalDuration types.NullDuration `json:"totalDuration"`
		MaxVUs        uint64             `json:"maxVUs"`
	}{
		test.derivedConfig.Options,
		types.NewNullDuration(duration, true),
		lib.GetMaxPossibleVUs(executionPlan),
	}, nil
}
