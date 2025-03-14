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
	launcherCmd "go.k6.io/k6/internal/launcher/cmd"
)

// Execute runs the k6 command.
func Execute() {
	gs := state.NewGlobalState(context.Background())

	tryBinaryProvisioning := gs.Flags.BinaryProvisioning

	var deps k6deps.Dependencies
	var opt *launcherCmd.Options
	if tryBinaryProvisioning {
		gs.Logger.Debug("trying to provision binary")

		var err error
		deps, err = analyze(gs, gs.CmdArgs[1:])
		if err != nil {
			gs.Logger.
				WithError(err).
				Error("failed to analyze dependencies, can't try binary provisioning, please report this issue")
		}

		buildRequired := isCustomBuildRequired(build.Version, deps)
		gs.Logger.
			WithField("buildRequired", buildRequired).
			WithField("deps", deps).
			Debug("binary provisioning, dependencies analyzed")

		tryBinaryProvisioning = tryBinaryProvisioning && buildRequired

		opt = launcherCmd.NewOptions(gs)
		if !opt.CanUseBuildService() && tryBinaryProvisioning {
			gs.Logger.Warn(
				"your scripts/archives require a build service token, but it's not set, " +
					"please set the K6_CLOUD_TOKEN environment variable or k6 cloud login. ",
			)
			tryBinaryProvisioning = false
		}
	} else {
		gs.Logger.Debug("binary provisioning disabled")
	}

	if tryBinaryProvisioning {
		// this will try to get the k6 binary from the build service
		// and run it, passing all the original arguments
		runWithBinaryProvisioning(gs, deps, opt)
	} else {
		// this will run the default k6 command
		k6Cmd.Execute(gs)
	}
}

func runWithBinaryProvisioning(gs *state.GlobalState, deps k6deps.Dependencies, opt *launcherCmd.Options) {
	cmd := launcherCmd.New(gs, deps, opt)

	// disable binary provisioning any second time
	gs.Env["K6_BINARY_PROVISIONING"] = "false"

	gs.Logger.Debug("running binary provisioning path")

	if err := cmd.Execute(); err != nil {
		gs.Logger.Error(formatError(err))

		var eerr *exec.ExitError
		if errors.As(err, &eerr) {
			gs.OSExit(eerr.ExitCode())
		}

		gs.OSExit(1)
	}
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
