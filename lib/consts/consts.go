// Package consts houses some constants needed across k6
package consts

import (
	"strings"
)

// Version contains the current semantic version of k6.
const Version = "0.55.0"

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
