package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"slices"

	"github.com/spf13/cobra"

	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/ext"
	"go.k6.io/k6/internal/build"
)

func getCmdDeps(gs *state.GlobalState) *cobra.Command {
	depsCmd := &cobra.Command{
		Use:   "deps",
		Short: "Resolve and list the dependencies of a test",
		Long: `deps command provides users a clear overview of all dependencies needed for running a script. By analyzing imports (excluding `require` calls), the command ensures to accurately evaluate project's dependencies.` +
			` Additionally, it tells whether a custom build is required to run the script.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			test, err := loadLocalTestWithoutRunner(gs, cmd, args)
			if err != nil {
				var unsatisfiedErr binaryIsNotSatisfyingDependenciesError
				if !errors.As(err, &unsatisfiedErr) {
					return err
				}
			}

			deps := test.Dependencies()
			depsMap := map[string]string{}
			for name, constraint := range deps {
				if constraint == nil {
					depsMap[name] = "*"
					continue
				}
				depsMap[name] = constraint.String()
			}
			imports := test.Imports()
			slices.Sort(imports)

			result := struct {
				BuildDependencies   map[string]string `json:"buildDependencies"`
				Imports             []string          `json:"imports"`
				CustomBuildRequired bool              `json:"customBuildRequired"`
			}{
				BuildDependencies:   depsMap,
				Imports:             imports,
				CustomBuildRequired: isCustomBuildRequired(deps, build.Version, ext.GetAll()),
			}

			buf := &bytes.Buffer{}
			enc := json.NewEncoder(buf)
			enc.SetEscapeHTML(false)
			enc.SetIndent("", "  ")
			if err := enc.Encode(result); err != nil {
				return err
			}

			printToStdout(gs, buf.String())
			return nil
		},
	}

	depsCmd.Flags().SortFlags = false
	depsCmd.Flags().AddFlagSet(runtimeOptionFlagSet(false))

	return depsCmd
}
