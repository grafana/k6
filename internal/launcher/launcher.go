// Package launcher is the entry point for the k6 command.
package launcher

import (
	"context"
	"errors"
	"os/exec"

	"github.com/grafana/k6deps"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/internal/build"
	k6Cmd "go.k6.io/k6/internal/cmd"
)

// Execute runs the k6 command.
func Execute() {
	gs := state.NewGlobalState(context.Background())

	gs.OSExit(newLauncher(gs).launch())
}

// launcher is a k6 launcher
type launcher struct {
	gs *state.GlobalState
	// function to fall back if binary provisioning is not required
	fallback func(gs *state.GlobalState)
	// function to provision a k6 binary that satisfies the dependencies
	provision func(*state.GlobalState, k6deps.Dependencies) (string, string, error)
	// function to execute k6 binary
	run func(*state.GlobalState, string) (error, int)
}

func newLauncher(gs *state.GlobalState) *launcher {
	return &launcher{
		gs:        gs,
		fallback:  k6Cmd.Execute,
		provision: k6buildProvision,
		run:       runK6Cmd,
	}
}

// launch executes k6 either by launching a provisioned binary or defaulting to the
// current binary it this is not necessary.
// Returns an int to be used as exit code.
// If the fhe fallback is called, it can exit the process so don't assume it will return
func (l *launcher) launch() int {
	// if binary provisioning not enabled, continue with regular k6 execution path
	if !l.gs.Flags.BinaryProvisioning {
		l.gs.Logger.Debug("binary provisioning disabled")
		l.fallback(l.gs)
		return 0
	}

	// TODO: maybe use Info to alert user it is using the feature?
	l.gs.Logger.Debug("trying to provision binary")

	deps, err := analyze(l.gs, l.gs.CmdArgs[1:])
	if err != nil {
		l.gs.Logger.
			WithError(err).
			Error("failed to analyze dependencies, can't try binary provisioning, please report this issue")
		return 1
	}

	// binary provisioning enabled but not required by this command
	// continue with regular k6 execution path
	if !isCustomBuildRequired(build.Version, deps) {
		l.gs.Logger.
			Debug("binary provisioning not required")
		l.fallback(l.gs)
		return 0
	}

	l.gs.Logger.
		WithField("deps", deps).
		Info("dependencies identified, binary provisioning required")

	// this will try to get the k6 binary from the build service
	// and run it, passing all the original arguments
	binPath, versions, err := l.provision(l.gs, deps)
	if err != nil {
		l.gs.Logger.
			WithError(err).
			Error("failed to fetch a binary with required dependencies, please report this issue")
		return 1
	}

	l.gs.Logger.
		Info("k6 has been provisioned with version", versions)

	l.gs.Logger.Debug("launching provisioned k6 binary")

	if err, rc := l.run(l.gs, binPath); err != nil {
		l.gs.Logger.Error(err)
		return rc
	}

	return 0
}

// runs the k6 binary
func runK6Cmd(gs *state.GlobalState, binPath string) (error, int) {
	cmd := exec.CommandContext(gs.Ctx, binPath, gs.CmdArgs[1:]...) //nolint:gosec
	cmd.Stderr = gs.Stderr
	cmd.Stdout = gs.Stdout
	cmd.Stdin = gs.Stdin

	// disable binary provisioning any second time
	gs.Env["K6_BINARY_PROVISIONING"] = "false"

	if err := cmd.Run(); err != nil {
		var eerr *exec.ExitError
		if errors.As(err, &eerr) {
			return err, eerr.ExitCode()
		}
	}

	return nil, 0
}

// anyK6Version is a wildcard version for k6
// if that appeared up in the dependencies, we'll use the base k6 version
const anyK6Version = k6deps.ConstraintsAny

// isCustomBuildRequired checks if the build is required
// it's required if there is no k6 dependency in deps
// or if the resolved version is not the base version
// or if there are more than one (not k6) dependency
func isCustomBuildRequired(baseK6Version string, deps k6deps.Dependencies) bool {
	if len(deps) == 0 {
		return false
	}

	// Early return if there are multiple dependencies
	if len(deps) > 1 {
		return true
	}

	k6Dependency, hasK6 := deps["k6"]

	// Early return if there's exactly one non-k6 dependency
	if len(deps) == 1 && !hasK6 {
		return true
	}

	// Get k6 version constraint if it exists
	v := anyK6Version
	if hasK6 && k6Dependency != nil && k6Dependency.Constraints != nil {
		v = k6Dependency.Constraints.String()
	}

	// No build required when default version is used
	if v == anyK6Version {
		return false
	}

	// No build required when using the base version
	return v != baseK6Version
}
