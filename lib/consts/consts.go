/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package consts

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
)

// Version contains the current semantic version of k6.
const Version = "0.29.0"

// VersionDetails can be set externally as part of the build process
var VersionDetails = "" // nolint:gochecknoglobals

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
