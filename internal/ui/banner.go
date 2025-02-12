package ui

import "strings"

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
