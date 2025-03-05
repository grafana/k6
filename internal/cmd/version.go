package cmd

import (
	"encoding/json"
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/ext"
	"go.k6.io/k6/internal/build"
)

const (
	commitKey      = "commit"
	commitDirtyKey = "commit_dirty"
)

// fullVersion returns the maximally full version and build information for
// the currently running k6 executable.
func fullVersion() string {
	details := versionDetails()

	goVersionArch := fmt.Sprintf("%s, %s/%s", details["go_version"], details["go_os"], details["go_arch"])

	k6version := fmt.Sprintf("%s", details["version"])
	// for the fallback case when the version is not in the expected format
	// cobra adds a "v" prefix to the version
	k6version = strings.TrimLeft(k6version, "v")

	commit, ok := details[commitKey].(string)
	if !ok || commit == "" {
		return fmt.Sprintf("%s (%s)", k6version, goVersionArch)
	}

	isDirty, ok := details[commitDirtyKey].(bool)
	if ok && isDirty {
		commit += "-dirty"
	}

	return fmt.Sprintf("%s (commit/%s, %s)", k6version, commit, goVersionArch)
}

// versionDetails returns the structured details about version
func versionDetails() map[string]interface{} {
	v := build.Version
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}

	details := map[string]interface{}{
		"version":    v,
		"go_version": runtime.Version(),
		"go_os":      runtime.GOOS,
		"go_arch":    runtime.GOARCH,
	}

	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return details
	}

	var (
		commit string
		dirty  bool
	)
	for _, s := range buildInfo.Settings {
		switch s.Key {
		case "vcs.revision":
			commitLen := 10
			if len(s.Value) < commitLen {
				commitLen = len(s.Value)
			}
			commit = s.Value[:commitLen]
		case "vcs.modified":
			if s.Value == "true" {
				dirty = true
			}
		default:
		}
	}

	if commit == "" {
		return details
	}

	details[commitKey] = commit
	if dirty {
		details[commitDirtyKey] = true
	}

	return details
}

func versionString() string {
	v := fullVersion()

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

	details := versionDetails()
	if exts := ext.GetAll(); len(exts) > 0 {
		type extInfo struct {
			Module  string   `json:"module"`
			Version string   `json:"version"`
			Imports []string `json:"imports"`
		}

		ext := make(map[string]extInfo)
		for _, e := range exts {
			key := e.Path + "@" + e.Version

			if v, ok := ext[key]; ok {
				v.Imports = append(v.Imports, e.Name)
				ext[key] = v
				continue
			}

			ext[key] = extInfo{
				Module:  e.Path,
				Version: e.Version,
				Imports: []string{e.Name},
			}
		}

		list := make([]extInfo, 0, len(ext))
		for _, v := range ext {
			list = append(list, v)
		}

		details["extensions"] = list
	}

	if err := json.NewEncoder(c.gs.Stdout).Encode(details); err != nil {
		return fmt.Errorf("failed to encode/output version details: %w", err)
	}

	return nil
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
