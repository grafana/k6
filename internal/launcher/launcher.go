// Package launcher is the entry point for the k6 command.
package launcher

import (
	"github.com/Masterminds/semver/v3"
	"github.com/grafana/k6deps"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/internal/build"
)

// commandExecutor executes the requested k6 command line command.
// It abstract the execution path from the concrete binary.
type commandExecutor interface {
	run(*state.GlobalState)
}

// Launcher is a k6 launcher. It analyses the requirements of a k6 execution,
// then if required, it provisions a binary executor to satisfy the requirements.
type Launcher struct {
	// gs is the global state of k6.
	gs *state.GlobalState

	// provision generates a custom binary from the received list of dependencies
	// with their constrains, and it returns an executor that satisfies them.
	provision func(*state.GlobalState, k6deps.Dependencies) (commandExecutor, error)

	// commandExecutor executes the requested k6 command line command
	commandExecutor commandExecutor
}

// New creates a new Launcher from a GlobalState using the default fallback and provision functions
func New(gs *state.GlobalState) *Launcher {
	defaultRunner := &currentBinary{}
	return &Launcher{
		gs:              gs,
		provision:       k6buildProvision,
		commandExecutor: defaultRunner,
	}
}

// Launch executes k6 either by launching a provisioned binary or defaulting to the
// current binary if this is not necessary.
// If the fhe fallback is called, it can exit the process so don't assume it will return
func (l *Launcher) Launch() {
	// If binary provisioning is not enabled, continue with the regular k6 execution path
	if !l.gs.Flags.BinaryProvisioning {
		l.gs.Logger.Debug("Binary provisioning feature is disabled")
		l.commandExecutor.run(l.gs)
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

	if !customBuildRequired(build.Version, deps) {
		l.gs.Logger.
			Debug("The current k6 binary already satisfies all the required dependencies," +
				" it isn't required to provision a new binary.")
		l.commandExecutor.run(l.gs)
		return
	}

	l.gs.Logger.
		WithField("deps", deps).
		Info("The current k6 binary doesn't satisfy all the required dependencies, it is required to" +
			" provision a new binary.")

	customBinary, err := l.provision(l.gs, deps)
	if err != nil {
		l.gs.Logger.
			WithError(err).
			Error("Failed to provision a new k6 binary with required dependencies." +
				" Please, make sure to report this issue by opening a bug report.")
		l.gs.OSExit(1)
		return
	}

	customBinary.run(l.gs)
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
