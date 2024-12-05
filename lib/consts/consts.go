// Package consts houses some constants needed across k6
package consts

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
)

// Version contains the current semantic version of k6.
const Version = "0.55.0"

const (
	commitKey      = "commit"
	commitDirtyKey = "commit_dirty"
)

// FullVersion returns the maximally full version and build information for
// the currently running k6 executable.
func FullVersion() string {
	details := VersionDetails()

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

// VersionDetails returns the structured details about version
func VersionDetails() map[string]interface{} {
	v := Version
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

// Banner returns the ASCII-art banner with the k6 logo
func Banner() string {
	banner := strings.Join([]string{
		`         /\      Grafana   /‾‾/  `,
		`    /\  /  \     |\  __   /  /   `,
		`   /  \/    \    | |/ /  /   ‾‾\ `,
		`  /          \   |   (  |  (‾)  |`,
		` / __________ \  |_|\_\  \_____/ `,
	}, "\n")

	return banner
}
