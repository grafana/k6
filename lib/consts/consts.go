package consts

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
)

// Version contains the current semantic version of k6.
const Version = "0.41.0"

// VersionDetails can be set externally as part of the build process
var VersionDetails = "" //nolint:gochecknoglobals

// FullVersion returns the maximally full version and build information for
// the currently running k6 executable.
func FullVersion() string {
	goVersionArch := fmt.Sprintf("%s, %s/%s", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	if VersionDetails != "" {
		return fmt.Sprintf("%s (%s, %s)", Version, VersionDetails, goVersionArch)
	}

	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		return fmt.Sprintf("%s (%s, %s)", Version, buildInfo.Main.Version, goVersionArch)
	}

	return fmt.Sprintf("%s (dev build, %s)", Version, goVersionArch)
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
