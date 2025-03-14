// Package cmd contains the cobra command for the k6 launcher.
package cmd

import (
	"github.com/grafana/k6deps"
	"github.com/spf13/cobra"
	k6State "go.k6.io/k6/cmd/state"
)

// New creates new cobra command for exec command.
func New(gs *k6State.GlobalState, deps k6deps.Dependencies, opt *Options) *cobra.Command {
	state := newState(gs, deps, opt)

	root := &cobra.Command{
		Use:                "k6 [flags] [command]",
		Short:              "Run k6 with extensions",
		SilenceUsage:       true,
		SilenceErrors:      true,
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
		DisableAutoGenTag:  true,
		CompletionOptions:  cobra.CompletionOptions{DisableDefaultCmd: true},
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return state.preRunE()
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return state.runE()
		},
	}

	return root
}
