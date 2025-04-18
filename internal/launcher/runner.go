package launcher

import (
	"errors"
	"fmt"
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

	// Copy environment variables to the k6 process and skip binary provisioning feature flag to disable it.
	// If not disabled, then the executed k6 binary would enter an infinite loop, where it continuously
	// process the input script, detect dependencies, and retrigger provisioning.
	env := []string{}
	for k, v := range gs.Env {
		if k == "K6_BINARY_PROVISIONING" {
			continue
		}
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env

	gs.Logger.Debug("Launching the provisioned k6 binary")

	rc := 0
	if err := cmd.Run(); err != nil {
		rc = 1
		gs.Logger.
			WithError(err).
			Error("Failed to run the provisioned k6 binary")

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
