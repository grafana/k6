package consts

import (
	"strings"
)

// Version contains the current semantic version of k6.
//nolint:gochecknoglobals
var Version = "0.25.2-dev"

// Banner contains the ASCII-art banner with the k6 logo and stylized website URL
//TODO: make these into methods, only the version needs to be a variable
//nolint:gochecknoglobals
var Banner = strings.Join([]string{
	`          /\      |‾‾|  /‾‾/  /‾/   `,
	`     /\  /  \     |  |_/  /  / /    `,
	`    /  \/    \    |      |  /  ‾‾\  `,
	`   /          \   |  |‾\  \ | (_) | `,
	`  / __________ \  |__|  \__\ \___/ .io`,
}, "\n")
