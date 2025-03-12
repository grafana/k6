package cmd

import (
	"context"
	"os/exec"

	"github.com/grafana/k6deps"
)

// Command returns the exec.Cmd struct to execute k6 with the given arguments.
// If the given subcommand has a script argument, it analyzes the dependencies
// in the script and provisions a k6 executable based on them.
// In Options, you can also specify environment variable and manifest file as dependency sources.
// The second return value is a cleanup function that is used to delete this temporary directory.
// TODO: as the cache is now handled by the k6provider library, consider removing the cleanup function
func Command(
	ctx context.Context,
	args []string,
	deps k6deps.Dependencies,
	opts *Options,
) (*exec.Cmd, func() error, error) {
	exe, err := provision(ctx, deps, opts)
	if err != nil {
		return nil, nil, err
	}

	cmd := exec.CommandContext(ctx, exe, args...) //nolint:gosec

	// TODO: once k6provider implements the cleanup of binary return the proper cleanup function (pablochacin)
	return cmd, func() error { return nil }, nil
}
