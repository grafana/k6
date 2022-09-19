package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/lib/consts"
)

func getCmdVersion(globalState *globalState) *cobra.Command {
	// versionCmd represents the version command.
	var showModules bool

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Show application version",
		Long:  `Show the application version and exit.`,
		Run: func(_ *cobra.Command, _ []string) {
			printToStdout(globalState, fmt.Sprintf("k6 v%s\n", consts.FullVersion()))

			if showModules {
				mods := modules.GetJSModules()
				if len(mods) > 0 {
					printToStdout(globalState, "modules:\n")
					for moduleName, _ := range mods {
						printToStdout(globalState, fmt.Sprintf("\t- %s\n", moduleName))
					}
				}
			}
		},
	}

	versionCmd.Flags().BoolVarP(&showModules,
		"modules",
		"m",
		false,
		"show built-in modules")

	return versionCmd
}
