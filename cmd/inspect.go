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
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"go.k6.io/k6/core/local"
	"go.k6.io/k6/js"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/metrics"
	"go.k6.io/k6/lib/types"
)

func getInspectCmd(logger *logrus.Logger) *cobra.Command {
	var addExecReqs bool

	// inspectCmd represents the inspect command
	inspectCmd := &cobra.Command{
		Use:   "inspect [file]",
		Short: "Inspect a script or archive",
		Long:  `Inspect a script or archive.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, filesystems, err := readSource(args[0], logger)
			if err != nil {
				return err
			}

			runtimeOptions, err := getRuntimeOptions(cmd.Flags(), buildEnvMap(os.Environ()))
			if err != nil {
				return err
			}
			registry := metrics.NewRegistry()
			builtinMetrics := metrics.RegisterBuiltinMetrics(registry)

			var b *js.Bundle
			switch getRunType(src) {
			// this is an exhaustive list
			case typeArchive:
				var arc *lib.Archive
				arc, err = lib.ReadArchive(bytes.NewBuffer(src.Data))
				if err != nil {
					return err
				}
				b, err = js.NewBundleFromArchive(logger, arc, runtimeOptions, registry)

			case typeJS:
				b, err = js.NewBundle(logger, src, filesystems, runtimeOptions, registry)
			}
			if err != nil {
				return err
			}

			// ATM, output can take 2 forms: standard (equal to lib.Options struct) and extended, with additional fields.
			inspectOutput := interface{}(b.Options)

			if addExecReqs {
				inspectOutput, err = addExecRequirements(b, builtinMetrics, registry, logger)
				if err != nil {
					return err
				}
			}

			data, err := json.MarshalIndent(inspectOutput, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))

			return nil
		},
	}

	inspectCmd.Flags().SortFlags = false
	inspectCmd.Flags().AddFlagSet(runtimeOptionFlagSet(false))
	inspectCmd.Flags().StringVarP(&runType, "type", "t", runType, "override file `type`, \"js\" or \"archive\"")
	inspectCmd.Flags().BoolVar(&addExecReqs,
		"execution-requirements",
		false,
		"include calculations of execution requirements for the test")

	return inspectCmd
}

func addExecRequirements(b *js.Bundle,
	builtinMetrics *metrics.BuiltinMetrics, registry *metrics.Registry,
	logger *logrus.Logger) (interface{}, error) {

	// TODO: after #1048 issue, consider rewriting this without a Runner:
	// just creating ExecutionPlan directly from validated options

	runner, err := js.NewFromBundle(logger, b, builtinMetrics, registry)
	if err != nil {
		return nil, err
	}

	conf, err := getConsolidatedConfig(afero.NewOsFs(), Config{}, runner.GetOptions())
	if err != nil {
		return nil, err
	}

	conf, err = deriveAndValidateConfig(conf, runner.IsExecutable)
	if err != nil {
		return nil, err
	}

	if err = runner.SetOptions(conf.Options); err != nil {
		return nil, err
	}
	execScheduler, err := local.NewExecutionScheduler(runner, logger)
	if err != nil {
		return nil, err
	}

	executionPlan := execScheduler.GetExecutionPlan()
	duration, _ := lib.GetEndOffset(executionPlan)

	return struct {
		lib.Options
		TotalDuration types.NullDuration `json:"totalDuration"`
		MaxVUs        uint64             `json:"maxVUs"`
	}{
		conf.Options,
		types.NewNullDuration(duration, true),
		lib.GetMaxPossibleVUs(executionPlan),
	}, nil
}
