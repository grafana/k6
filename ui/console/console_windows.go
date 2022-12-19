//go:build windows
// +build windows

package console

import (
	"os"
)

func getWinchSignal() os.Signal {
	return nil
}
