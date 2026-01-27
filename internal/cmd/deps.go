package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/ext"
	"go.k6.io/k6/internal/build"
)

type depsCmd struct {
	gs     *state.GlobalState
	isJSON bool
}

func getCmdDeps(gs *state.GlobalState) *cobra.Command {
	depsCmd := &depsCmd{gs: gs}

	cmd := &cobra.Command{
		Use:   "deps",
		Short: "Resolve and list the dependencies of a test",
		Long: "deps command provides users a clear overview of all dependencies needed for running a script. " +
			"By analyzing imports (excluding `require` calls), the command resolves and lists the test's dependencies." +
			" Additionally, it tells whether a custom build is required to run the script.",
		Args:    exactArgsWithMsg(1, "arg should either be \"-\", if reading script from stdin, or a path to a script file"),
		Example: getExampleText(gs, `  {{.}} deps script.js`),
		RunE:    depsCmd.run,
	}

	cmd.Flags().BoolVar(&depsCmd.isJSON, "json", false, "if set, output dependencies information will be in JSON format")
	cmd.Flags().SortFlags = false
	cmd.Flags().AddFlagSet(runtimeOptionFlagSet(false))

	return cmd
}

func (c *depsCmd) run(cmd *cobra.Command, args []string) error {
	test, err := loadLocalTestWithoutRunner(c.gs, cmd, args)
	if err != nil {
		var unsatisfiedErr binaryIsNotSatisfyingDependenciesError
		if !errors.As(err, &unsatisfiedErr) {
			return err
		}
	}

	deps := test.Dependencies()
	depsMap := map[string]string{}
	for name, constraint := range test.Dependencies() {
		if constraint == nil {
			depsMap[name] = "*"
			continue
		}
		depsMap[name] = constraint.String()
	}
	imports := test.Imports()
	slices.Sort(imports)

	customBuildRequired := isCustomBuildRequired(deps, build.Version, ext.GetAll())

	if c.isJSON {
		return c.outputJSON(depsMap, imports, customBuildRequired)
	}

	return c.outputHumanReadable(depsMap, imports, customBuildRequired)
}

func (c *depsCmd) outputJSON(depsMap map[string]string, imports []string, customBuildRequired bool) error {
	result := struct {
		BuildDependencies   map[string]string `json:"buildDependencies"`
		Imports             []string          `json:"imports"`
		CustomBuildRequired bool              `json:"customBuildRequired"`
	}{
		BuildDependencies:   depsMap,
		Imports:             imports,
		CustomBuildRequired: customBuildRequired,
	}

	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		return err
	}

	printToStdout(c.gs, buf.String())
	return nil
}

func (c *depsCmd) outputHumanReadable(depsMap map[string]string, imports []string, customBuildRequired bool) error {
	var output strings.Builder

	// Build Dependencies section
	if len(depsMap) > 0 {
		output.WriteString("Build Dependencies:\n")
		// Sort dependencies for consistent output
		depNames := make([]string, 0, len(depsMap))
		for name := range depsMap {
			depNames = append(depNames, name)
		}
		slices.Sort(depNames)

		for _, name := range depNames {
			constraint := depsMap[name]
			fmt.Fprintf(&output, "  %s: %s\n", name, constraint)
		}
		output.WriteString("\n")
	} else {
		output.WriteString("Build Dependencies: (none)\n\n")
	}

	// Imports section
	if len(imports) > 0 {
		output.WriteString("Imports:\n")
		for _, imp := range imports {
			fmt.Fprintf(&output, "  %s\n", imp)
		}
		output.WriteString("\n")
	} else {
		output.WriteString("Imports: (none)\n\n")
	}

	// Custom Build Required section
	if customBuildRequired {
		output.WriteString("Custom Build Required: yes\n")
	} else {
		output.WriteString("Custom Build Required: no\n")
	}

	printToStdout(c.gs, output.String())
	return nil
}
