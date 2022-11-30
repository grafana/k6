package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"go.k6.io/k6/ext"
	"go.k6.io/k6/lib/consts"
)

func getCmdVersion(gs *globalState) *cobra.Command {
	// versionCmd represents the version command.
	return &cobra.Command{
		Use:   "version",
		Short: "Show application version",
		Long:  `Show the application version and exit.`,
		Run: func(_ *cobra.Command, _ []string) {
			gs.console.Printf("k6 v%s\n", consts.FullVersion())

			if exts := ext.GetAll(); len(exts) > 0 {
				extsDesc := make([]string, 0, len(exts))
				for _, e := range exts {
					extsDesc = append(extsDesc, fmt.Sprintf("  %s", e.String()))
				}
				gs.console.Printf("Extensions:\n%s\n", strings.Join(extsDesc, "\n"))
			}
		},
	}
}
