// Package launcher is the entry point for the k6 command.
package launcher

import (
	"errors"
	"fmt"
	"os/exec"
	"slices"

	"github.com/spf13/cobra"

	"github.com/Masterminds/semver/v3"
	"github.com/grafana/k6deps"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/internal/build"
)

// launcher is a k6 launcher
type Launcher struct {
	gs      *state.GlobalState
	// path to the provisioned k6 binary
	binPath string
	// function to provision a k6 binary that satisfies the dependencies
	provision func(*state.GlobalState, k6deps.Dependencies) (string, string, error)
	// function to execute k6 binary
	exec func(*state.GlobalState, string) (int, error)
	// original root command's persistent pre-run function
	rootPPRE func(*cobra.Command, []string) error
}

func New(gs *state.GlobalState) *Launcher {
	return &Launcher{
		gs:        gs,
		provision: k6buildProvision,
		exec:      execK6,
	}

}

func (l *Launcher) Install(root *cobra.Command) {
	if !l.gs.Flags.BinaryProvisioning {
		return
	}

	l.rootPPRE = root.PersistentPreRunE

	root.PersistentPreRunE = l.ppre
}

// ppre runs previous to the execution of the command.
// If the script has dependencies that cannot satisfied by the current binary, tries to provision a binary
// and executes it as a sub-process.
// Otherwise, returns control to the current binary and continue the normal execution flow.
func (l *Launcher) ppre(cmd *cobra.Command, args []string) error {
	// call root command's persistent pre run function to ensuere proper setup of logs
	err := l.rootPPRE(cmd, args)
	if err != nil {
		return err
	}

	// return early if the command do not require binary provisioning
	if !slices.Contains([]string{"run", "archive", "inspect", "cloud"}, cmd.Name()) {
		return nil
	}

	l.gs.Logger.Info("trying to provision binary")

	deps, err := k6deps.Analyze(newDepsOptions(l.gs, args))
	if err != nil {
		l.gs.Logger.
			WithError(err).
			Error("failed to analyze dependencies, can't try binary provisioning, please report this issue")
		return fmt.Errorf("failed binary provisioning")
	}

	// binary provisioning enabled but not required by this command
	// continue with regular k6 execution path
	if !isCustomBuildRequired(build.Version, deps) {
		l.gs.Logger.
			Debug("binary provisioning not required")
		return nil
	}

	l.gs.Logger.
		WithField("deps", deps).
		Info("dependencies identified, binary provisioning required")

	// get the k6 binary from the build service
	binPath, versions, err := l.provision(l.gs, deps)
	if err != nil {
		l.gs.Logger.
			WithError(err).
			Error("failed to fetch a binary with required dependencies, please report this issue")
		return fmt.Errorf("failed binary provisioning")
	}

	l.gs.Logger.
		Info("k6 has been provisioned with version(s) ", versions)

	// replace command's execution with binary provisioning
	l.binPath = binPath
	cmd.RunE = l.runE

	return nil
}

func (l *Launcher) runE(cmd *cobra.Command, args []string) error {
	l.gs.Logger.
		Debug("launching provisioned k6 binary")

	// execute provisioned binary
	// TODO: ensure we return the rc from k6
	rc, err := l.exec(l.gs, l.binPath)
	if err != nil {
		l.gs.Logger.
			WithError(err).
			Debug("failed to execute k6 binary")
			return err
	}

	l.gs.OSExit(rc)

	return nil
}

// execK6 runs the k6 binary passing the original arguments
func execK6(gs *state.GlobalState, binPath string) (int, error) {
	cmd := exec.CommandContext(gs.Ctx, binPath, gs.CmdArgs[1:]...) //nolint:gosec
	cmd.Stderr = gs.Stderr
	cmd.Stdout = gs.Stdout
	cmd.Stdin = gs.Stdin

	// disable binary provisioning to avoid a provisioning loop
	gs.Env["K6_BINARY_PROVISIONING"] = "false"

	if err := cmd.Run(); err != nil {
		var eerr *exec.ExitError
		if errors.As(err, &eerr) {
			return eerr.ExitCode(), nil
		}
		return 0, err
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
