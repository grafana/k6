// Package cmd is here to provide a way for xk6 to build a binary with added extensions
package cmd

import (
	"context"

	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/internal/cmd"
)

// Execute the k6 command
// It only is exported here for backwards compatibility and the ability to use xk6 to build extended k6
func Execute() {
	gs := state.NewGlobalState(context.Background())
	cmd.ExecuteWithGlobalState(gs)
}
