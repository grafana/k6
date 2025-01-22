//go:build windows
// +build windows

package cmd

import (
	"os"
)

func getWinchSignal() os.Signal {
	return nil
}
