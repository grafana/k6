package oauth

import (
	"fmt"
	"os/exec"
	"runtime"
)

// OpenInBrowser launches url in the user's default browser. Failing to open a
// browser is not fatal to a login: the caller prints the URL so the user can
// open it themselves.
func OpenInBrowser(url string) error {
	// The URL is built by k6 from validated inputs, but it is still passed as
	// an argument rather than through a shell, so it cannot be interpreted as
	// a command.
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("could not launch a browser: %w", err)
	}
	// The browser outlives k6, so the process is released rather than waited on.
	return cmd.Process.Release()
}
