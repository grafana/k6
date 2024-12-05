package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/ext"
	"go.k6.io/k6/lib/consts"
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

type versionCmd struct {
	gs     *state.GlobalState
	isJSON bool
}

func (c *versionCmd) run(cmd *cobra.Command, _ []string) error {
	if !c.isJSON {
		root := cmd.Root()
		root.SetArgs([]string{"--version"})
		_ = root.Execute()
		return nil
	}

	details := consts.VersionDetails()
	if exts := ext.GetAll(); len(exts) > 0 {
		ext := make([]map[string]string, 0, len(exts))
		for _, e := range exts {
			ext = append(ext, map[string]string{
				"name":    e.Name,
				"type":    e.Type.String(),
				"version": e.Version,
				"path":    e.Path,
			})
		}

		details["extensions"] = ext
	}

	jsonDetails, err := json.Marshal(details)
	if err != nil {
		return fmt.Errorf("failed produce a JSON version details: %w", err)
	}

	_, err = fmt.Fprintln(c.gs.Stdout, string(jsonDetails))
	return err
}

func getCmdVersion(gs *state.GlobalState) *cobra.Command {
	versionCmd := &versionCmd{gs: gs}

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show application version",
		Long:  `Show the application version and exit.`,
		RunE:  versionCmd.run,
	}

	cmd.Flags().BoolVar(&versionCmd.isJSON, "json", false, "if set, output version information will be in JSON format")

	return cmd
}
