// Package launcher is the entry point for the k6 command.
package launcher

import (
	"github.com/Masterminds/semver/v3"
	"github.com/grafana/k6deps"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/internal/build"
)

// Launcher is a k6 Launcher
// It analyses the requirements of a k6 execution. If required, provisions a k6Runner to satisfy these
// requirements.
type Launcher struct {
	gs *state.GlobalState
	// provision function receives a list of dependencies with their constrains and returns
	// a k6runner than satisfies them
	provision func(*state.GlobalState, k6deps.Dependencies) (k6Runner, error)
	// k6Runner to execute k6 command
	runner k6Runner
}

// New creates a new Launcher from a GlobalState using the default fallback and provision functions
func New(gs *state.GlobalState) *Launcher {
	return &Launcher{
		gs:        gs,
		provision: k6buildProvision,
		runner:    newDefaultK6Runner(),
	}
}

// Launch executes k6 either by launching a provisioned binary or defaulting to the
// current binary if this is not necessary.
// If the fhe fallback is called, it can exit the process so don't assume it will return
func (l *Launcher) Launch() {
	// if binary provisioning is not enabled, continue with the regular k6 execution path
	if !l.gs.Flags.BinaryProvisioning {
		l.gs.Logger.Debug("Binary provisioning feature is disabled")
		l.runner.run(l.gs)
		return
	}

	l.gs.Logger.Info("Binary provisioning feature is enabled. If it's required, k6 will provision a new binary")

	deps, err := analyze(l.gs, l.gs.CmdArgs[1:])
	if err != nil {
		l.gs.Logger.
			WithError(err).
			Error("Failed to analyze the required dependencies. Please, make sure to report this issue by" +
				" opening a bug report.")
		l.gs.OSExit(1)
		return // this is required for testing
	}

	// if the command does not have dependencies or a custom build
	if !customBuildRequired(build.Version, deps) {
		l.gs.Logger.
			Debug("The current k6 binary already satisfies all the required dependencies," +
				" it isn't required to provision a new binary.")
		l.runner.run(l.gs)
		return
	}

	l.gs.Logger.
		WithField("deps", deps).
		Info("The current k6 binary doesn't satisfy all the required dependencies, it is required to" +
			" provision a new binary.")

	runner, err := l.provision(l.gs, deps)
	if err != nil {
		l.gs.Logger.
			WithError(err).
			Error("Failed to provision a new k6 binary with required dependencies." +
				" Please, make sure to report this issue by opening a bug report.")
		l.gs.OSExit(1)
		return
	}

	runner.run(l.gs)
}

// customBuildRequired checks if the build is required
// it's required if there is one or more dependencies other than k6 itself
// or if the required k6 version is not satisfied by the current binary's version
// TODO: get the version of any built-in extension and check if they satisfy the dependencies
func customBuildRequired(baseK6Version string, deps k6deps.Dependencies) bool {
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
