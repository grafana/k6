// Package launcher is the entry point for the k6 command.
package launcher

import (
	"errors"
	"os/exec"

	"github.com/Masterminds/semver/v3"
	"github.com/grafana/k6deps"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/internal/build"
	k6Cmd "go.k6.io/k6/internal/cmd"
)

// Launcher is a k6 Launcher
type Launcher struct {
	gs *state.GlobalState
	// function to fall back if binary provisioning is not required
	fallback func(gs *state.GlobalState)
	// function to provision a k6 binary that satisfies the dependencies
	provision func(*state.GlobalState, k6deps.Dependencies) (string, string, error)
	// function to execute k6 binary
	run func(*state.GlobalState, string) (int, error)
}

// New creates a new Launcher from a GlobalState using the default fallback and provision functions
func New(gs *state.GlobalState) *Launcher {
	return &Launcher{
		gs:        gs,
		fallback:  k6Cmd.ExecuteWithGlobalState,
		provision: k6buildProvision,
		run:       runK6Cmd,
	}
}

// Launch executes k6 either by launching a provisioned binary or defaulting to the
// current binary if this is not necessary.
// If the fhe fallback is called, it can exit the process so don't assume it will return
func (l *Launcher) Launch() {
	// if binary provisioning is not enabled, continue with the regular k6 execution path
	if !l.gs.Flags.BinaryProvisioning {
		l.gs.Logger.Debug("Binary provisioning feature is disabled")
		l.fallback(l.gs)
		return
	}

	l.gs.Logger.Info("Binary provisioning feature is enabled. If it's required, k6 will provision a new binary")

	deps, err := k6deps.Analyze(newDepsOptions(l.gs, l.gs.CmdArgs[1:]))
	if err != nil {
		l.gs.Logger.
			WithError(err).
			Error("Failed to analyze the required dependencies. Please, make sure to report this issue by" +
				" opening a bug report.")
		l.gs.OSExit(1)
	}

	// binary provisioning enabled but not required by this command
	// continue with regular k6 execution path
	if !isCustomBuildRequired(build.Version, deps) {
		l.gs.Logger.
			Debug("The current k6 binary already satisfies all the required dependencies," +
				" it isn't required to provision a new binary.")
		l.fallback(l.gs)
		return
	}

	l.gs.Logger.
		WithField("deps", deps).
		Info("The current k6 binary doesn't satisfy all the required dependencies, it is required to" +
			" provision a new binary.")

	l.launchCustomBuild(deps)
}

func (l *Launcher) launchCustomBuild(deps k6deps.Dependencies) {
	// get the k6 binary from the build service
	binPath, versions, err := l.provision(l.gs, deps)
	if err != nil {
		l.gs.Logger.
			WithError(err).
			Error("Failed to provision a new k6 binary with required dependencies. Please, make sure to" +
				" report this issue by opening a bug report.")
		l.gs.OSExit(1)
		// in tests calling l.gs.OSExit does not ends execution so we have to return
		return
	}

	l.gs.Logger.
		Info("A new k6 binary has been provisioned with version(s): ", versions)

	l.gs.Logger.Debug("Launching the new provisioned k6 binary")

	// execute provisioned binary
	if rc, err := l.run(l.gs, binPath); err != nil {
		l.gs.Logger.WithError(err).Error("Failed to run the new provisioned k6 binary")
		l.gs.OSExit(rc)
	}
}

// runK6Cmd runs the k6 binary passing the original arguments
func runK6Cmd(gs *state.GlobalState, binPath string) (int, error) {
	cmd := exec.CommandContext(gs.Ctx, binPath, gs.CmdArgs[1:]...) //nolint:gosec
	cmd.Stderr = gs.Stderr
	cmd.Stdout = gs.Stdout
	cmd.Stdin = gs.Stdin

	// disable binary provisioning to avoid a provisioning loop
	gs.Env["K6_BINARY_PROVISIONING"] = "false"

	if err := cmd.Run(); err != nil {
		var eerr *exec.ExitError
		if errors.As(err, &eerr) {
			return eerr.ExitCode(), err
		}
	}

	return 0, nil
}

// isCustomBuildRequired checks if the build is required
// it's required if there is one or more dependencies other than k6 itself
// or if the required k6 version is not satisfied by the current binary's version
// TODO: get the version of any built-in extension and check if they satisfy the dependencies
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
	if !hasK6 {
		return true
	}

	// Ignore k6 dependency if nil
	if k6Dependency == nil || k6Dependency.Constraints == nil {
		return false
	}

	k6Ver, err := semver.NewVersion(baseK6Version)
	if err != nil {
		// ignore if baseK6Version is not a valid sem ver (e.g. a development version)
		return true
	}

	// if the current version satisfies the constrains, binary provisioning is not required
	return !k6Dependency.Constraints.Check(k6Ver)
}
