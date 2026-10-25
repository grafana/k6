package oauth

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
)

// OpenInBrowser launches url in the user's default browser. Failing to open a
// browser is not fatal to a login: the caller prints the URL so the user can
// open it themselves.
//
// ctx only bounds the opener process, which hands the URL to the browser and
// exits immediately; the browser itself outlives it.
func OpenInBrowser(ctx context.Context, url string) error {
	// The URL is built by k6 from an allowlisted stack URL, and is passed as an
	// argument rather than through a shell, so it cannot be interpreted as a
	// command. The opener itself is a fixed, non-user-controlled name.
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(ctx, "open", url) //nolint:gosec // G204: see above
	case "windows":
		cmd = exec.CommandContext(ctx, "rundll32", "url.dll,FileProtocolHandler", url) //nolint:gosec // G204: see above
	default:
		cmd = exec.CommandContext(ctx, "xdg-open", url) //nolint:gosec // G204: see above
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("could not launch a browser: %w", err)
	}
	// The browser outlives k6, so the process is released rather than waited on.
	return cmd.Process.Release()
}
