package launcher

import (
	"errors"
	"os/exec"

	"go.k6.io/k6/cmd/state"
	k6Cmd "go.k6.io/k6/internal/cmd"
)

// customBinary runs the requested commands
// on a different binary on a subprocess passing the original arguments
type customBinary struct {
	// path represents the local file path
	// on the file system of the binary
	path string
}

func (b *customBinary) run(gs *state.GlobalState) {
	cmd := exec.CommandContext(gs.Ctx, b.path, gs.CmdArgs[1:]...) //nolint:gosec
	cmd.Stderr = gs.Stderr
	cmd.Stdout = gs.Stdout
	cmd.Stdin = gs.Stdin

	// Disable binary provisioning to avoid a recursive loop.
	// If we keep it enabled then the k6 binary executed here will receive the same input script,
	// and it will repeat the process again. It will analyze, detect the dependencies then triggering
	// again the binary provisioning.
	gs.Env["K6_BINARY_PROVISIONING"] = "false"

	gs.Logger.Debug("Launching the new provisioned k6 binary")

	rc := 0
	if err := cmd.Run(); err != nil {
		rc = 1
		gs.Logger.
			WithError(err).
			Error("Failed to run the new provisioned k6 binary")

		var eerr *exec.ExitError
		if errors.As(err, &eerr) {
			rc = eerr.ExitCode()
		}
	}
	gs.OSExit(rc)
}

// currentBinary runs the requested commands on the current binary
type currentBinary struct{}

func (b *currentBinary) run(gs *state.GlobalState) {
	k6Cmd.ExecuteWithGlobalState(gs)
}
