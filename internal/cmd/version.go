package cmd

import (
	"encoding/json"
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"

	"go.k6.io/k6/v2/cmd/state"
	"go.k6.io/k6/v2/ext"
	"go.k6.io/k6/v2/internal/build"
)

const (
	commitKey      = "commit"
	commitDirtyKey = "commit_dirty"
	mainK6Path     = "go.k6.io/k6/v2"
)

// k6VersionInfo holds the version and VCS metadata for the running k6 binary,
// as read from Go build info.
type k6VersionInfo struct {
	version string // effective version string, always with "v" prefix
	commit  string // short VCS hash, "devel" for plain go-build, "" if unknown
	dirty   bool   // true when the working tree was modified at build time
}

// readK6VersionInfo reads Go build info once and returns everything needed by
// both the version display and the dependency constraint check. This is the
// single source of truth so that what k6 prints and what it compares against
// are always the same string.
func readK6VersionInfo() k6VersionInfo {
	v := build.Version
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}

	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return k6VersionInfo{version: v}
	}

	if bi.Main.Path == mainK6Path {
		return readMainK6VersionInfo(bi, v)
	}
	return readLibK6VersionInfo(bi, v)
}

func readMainK6VersionInfo(bi *debug.BuildInfo, fallback string) k6VersionInfo {
	var info k6VersionInfo
	if bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		info.version = bi.Main.Version
	} else {
		info.version = fallback
		if bi.Main.Version == "(devel)" {
			info.commit = "devel"
		}
	}
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			commitLen := min(len(s.Value), 10)
			info.commit = s.Value[:commitLen]
		case "vcs.modified":
			info.dirty = s.Value == "true"
		}
	}
	return info
}

// readLibK6VersionInfo handles the case where k6 is used as a library inside another binary.
func readLibK6VersionInfo(bi *debug.BuildInfo, fallback string) k6VersionInfo {
	var info k6VersionInfo
	for _, dep := range bi.Deps {
		if dep.Path == mainK6Path {
			if dep.Replace != nil {
				info.version = dep.Replace.Version
			} else {
				info.version = dep.Version
			}
			break
		}
	}
	if info.version == "" {
		info.version = fallback
	}
	return info
}

// runtimeK6Version returns the effective k6 version for dependency constraint
// checking. It is guaranteed to match details["version"] from versionDetails().
func runtimeK6Version() string {
	return readK6VersionInfo().version
}

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
func versionDetails() map[string]any {
	info := readK6VersionInfo()

	details := map[string]any{
		"version":    info.version,
		"go_version": runtime.Version(),
		"go_os":      runtime.GOOS,
		"go_arch":    runtime.GOARCH,
	}

	if info.commit != "" {
		details[commitKey] = info.commit
	}
	if info.dirty {
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

// versionDetailsWithExtensions returns the structured details about version including extensions
// returns error if there are unhandled extension types
func versionDetailsWithExtensions(exts []*ext.Extension) (map[string]any, error) {
	details := versionDetails()

	if len(exts) == 0 {
		return details, nil
	}

	// extInfo represents the JSON structure for an extension in the version details
	// modeled after k6 extension registry structure
	type extInfo struct {
		Module  string   `json:"module"`
		Version string   `json:"version"`
		Imports []string `json:"imports,omitempty"`
		Outputs []string `json:"outputs,omitempty"`
	}

	infoList := make([]*extInfo, 0, len(exts))
	infoMap := make(map[string]*extInfo)

	for _, e := range exts {
		key := e.Path + "@" + e.Version

		info, found := infoMap[key]
		if !found {
			info = &extInfo{
				Module:  e.Path,
				Version: e.Version,
			}

			infoMap[key] = info
			infoList = append(infoList, info)
		}

		switch e.Type {
		case ext.OutputExtension:
			info.Outputs = append(info.Outputs, e.Name)
		case ext.JSExtension:
			info.Imports = append(info.Imports, e.Name)
		case ext.SecretSourceExtension:
			// currently, no special handling is needed for secret source extensions
		case ext.SubcommandExtension:
			// currently, no special handling is needed for subcommand extensions
		default:
			// report unhandled extension type for future proofing
			return details, fmt.Errorf("unhandled extension type: %s", e.Type)
		}
	}

	details["extensions"] = infoList

	return details, nil
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

	details, err := versionDetailsWithExtensions(ext.GetAll())
	if err != nil {
		return fmt.Errorf("failed to get version details with extensions: %w", err)
	}

	if err := json.NewEncoder(c.gs.Stdout).Encode(details); err != nil {
		return fmt.Errorf("failed to encode/output version details: %w", err)
	}

	return nil
}

func getCmdVersion(gs *state.GlobalState) *cobra.Command {
	versionCmd := &versionCmd{gs: gs}

	cmd := &cobra.Command{
		Use:    "version",
		Short:  "Show application version",
		Long:   `Show the application version and exit.`,
		Hidden: true,
		RunE:   versionCmd.run,
	}

	cmd.Flags().BoolVar(&versionCmd.isJSON, "json", false, "if set, output version information will be in JSON format")

	return cmd
}
