package launcher

import (
	"errors"
	"os/exec"

	"go.k6.io/k6/cmd/state"
	k6Cmd "go.k6.io/k6/internal/cmd"
)

// k6Runner defines the interface for running a k6 command
type k6Runner interface {
	run(*state.GlobalState)
}

// execRunner executes a k6 command in a subprocess
type execRunner struct {
	binPath string
}

// newK6Runner returns a k6Runner given the path to the k6 binary and the arguments to pass
func newK6Runner(binPath string) k6Runner {
	return &execRunner{
		binPath: binPath,
	}
}

// run executes the k6 binary in a process passing the original arguments
func (r *execRunner) run(gs *state.GlobalState) {
	cmd := exec.CommandContext(gs.Ctx, r.binPath, gs.CmdArgs[1:]...) //nolint:gosec
	cmd.Stderr = gs.Stderr
	cmd.Stdout = gs.Stdout
	cmd.Stdin = gs.Stdin

	// disable binary provisioning to avoid a provisioning loop.
	// if we keep it enabled the k6 binary executed here will receive the same input script
	// will analyze it and detect the dependencies, triggering the binary provisioning again
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

// defaultK6Runner defines a k6Runner that executes k6 using the current binary
type defaultK6Runner struct{}

// newDefaultK6Runner returns a defaultK6Runner
func newDefaultK6Runner() k6Runner {
	return &defaultK6Runner{}
}

func (r *defaultK6Runner) run(gs *state.GlobalState) {
	k6Cmd.ExecuteWithGlobalState(gs)
}
