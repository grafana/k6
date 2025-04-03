// Package launcher is the entry point for the k6 command.
package launcher

import (
	"context"
	"errors"
	"os/exec"

	"github.com/grafana/k6deps"
	"github.com/grafana/k6provider"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/internal/build"
	k6Cmd "go.k6.io/k6/internal/cmd"
)

// anyK6Version is a wildcard version for k6
// if that appeared up in the dependencies, we'll use the base k6 version
const anyK6Version = k6deps.ConstraintsAny

// Execute runs the k6 command.
func Execute() {
	gs := state.NewGlobalState(context.Background())

	tryBinaryProvisioning := gs.Flags.BinaryProvisioning

	var deps k6deps.Dependencies
	var opt Options
	if tryBinaryProvisioning {
		gs.Logger.Debug("trying to provision binary")

		var err error
		deps, err = analyze(gs, gs.CmdArgs[1:])
		if err != nil {
			gs.Logger.
				WithError(err).
				Error("failed to analyze dependencies, can't try binary provisioning, please report this issue")
			return
		}

		buildRequired := isCustomBuildRequired(build.Version, deps)
		gs.Logger.
			WithField("buildRequired", buildRequired).
			WithField("deps", deps).
			Debug("binary provisioning, dependencies analyzed")

		tryBinaryProvisioning = tryBinaryProvisioning && buildRequired

		opt = NewOptions(gs)
		if !opt.CanUseBuildService() && tryBinaryProvisioning {
			gs.Logger.Error(
				"your scripts/archives require a build service token, but it's not set, " +
					"please set the K6_CLOUD_TOKEN environment variable or k6 cloud login. ",
			)
			return
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

func runWithBinaryProvisioning(gs *state.GlobalState, deps k6deps.Dependencies, opt Options) {
	binPath, err := provision(gs, deps, opt)
	// TODO: add logs here?
	if err != nil {
		gs.Logger.
			WithError(err).
			Error("failed to fetch a binary with required dependencies, please report this issue")
		gs.OSExit(1)
	}

	cmd := exec.CommandContext(gs.Ctx, binPath, gs.CmdArgs[1:]...) //nolint:gosec
	cmd.Stderr = gs.Stderr
	cmd.Stdout = gs.Stdout
	cmd.Stdin = gs.Stdin

	// disable binary provisioning any second time
	gs.Env["K6_BINARY_PROVISIONING"] = "false"

	gs.Logger.Debug("running binary provisioning path")

	if err := cmd.Run(); err != nil {
		gs.Logger.Error(formatError(err))

		var eerr *exec.ExitError
		if errors.As(err, &eerr) {
			gs.OSExit(eerr.ExitCode())
		}

		gs.OSExit(1)
	}
}

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

func provision(gs *state.GlobalState, deps k6deps.Dependencies, opt Options) (string, error) {
	config := k6provider.Config{
		BuildServiceURL:  opt.BuildServiceURL,
		BuildServiceAuth: opt.BuildServiceToken,
	}

	provider, err := k6provider.NewProvider(config)
	if err != nil {
		return "", err
	}

	// TODO: we need a better handle of errors here
	// like (network, auth, etc) and give a better error message
	// to the user
	binary, err := provider.GetBinary(gs.Ctx, deps)
	if err != nil {
		return "", err
	}

	// TODO: for now we just log the version, but we need to come up with a better UI/UX
	gs.Logger.Infof("k6 has been provisioned with the version %q", binary.Dependencies["k6"])

	return binary.Path, nil
}
