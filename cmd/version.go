package cmd

import (
	"fmt"
	"strings"

	"github.com/liuxd6825/k6server/cmd/state"
	"github.com/liuxd6825/k6server/ext"
	"github.com/liuxd6825/k6server/lib/consts"
	"github.com/spf13/cobra"
)

func versionString() string {
	v := consts.FullVersion()

	if exts := ext.GetAll(); len(exts) > 0 {
		extsDesc := make([]string, 0, len(exts))
		for _, e := range exts {
			extsDesc = append(extsDesc, fmt.Sprintf("  %s", e.String()))
		}
		v += fmt.Sprintf("\nExtensions:\n%s\n",
			strings.Join(extsDesc, "\n"))
	}
	return v
}

func getCmdVersion(_ *state.GlobalState) *cobra.Command {
	// versionCmd represents the version command.
	return &cobra.Command{
		Use:   "version",
		Short: "Show application version",
		Long:  `Show the application version and exit.`,
		Run: func(cmd *cobra.Command, _ []string) {
			root := cmd.Root()
			root.SetArgs([]string{"--version"})
			_ = root.Execute()
		},
	}
}
