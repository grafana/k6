package consts

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
)

// Version contains the current semantic version of k6.
const Version = "0.47.0"

// FullVersion returns the maximally full version and build information for
// the currently running k6 executable.
func FullVersion() string {
	goVersionArch := fmt.Sprintf("%s, %s/%s", runtime.Version(), runtime.GOOS, runtime.GOARCH)

	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return fmt.Sprintf("%s (%s)", Version, goVersionArch)
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
		return fmt.Sprintf("%s (%s)", Version, goVersionArch)
	}

	if dirty {
		commit += "-dirty"
	}

	return fmt.Sprintf("%s (commit/%s, %s)", Version, commit, goVersionArch)
}

// Banner returns the ASCII-art banner with the k6 logo and stylized website URL
func Banner() string {
	banner := strings.Join([]string{
		`          /\      |‾‾| /‾‾/   /‾‾/   `,
		`     /\  /  \     |  |/  /   /  /    `,
		`    /  \/    \    |     (   /   ‾‾\  `,
		`   /          \   |  |\  \ |  (‾)  | `,
		`  / __________ \  |__| \__\ \_____/ .io`,
	}, "\n")

	return banner
}
