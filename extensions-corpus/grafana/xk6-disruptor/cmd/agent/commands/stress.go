package commands

import (
	"fmt"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/agent"
	"github.com/grafana/xk6-disruptor/pkg/agent/stressors"
	"github.com/grafana/xk6-disruptor/pkg/runtime"
	"github.com/spf13/cobra"
)

// BuildStressCmd returns a cobra command with the specification of the stress command
func BuildStressCmd(env runtime.Environment, config *agent.Config) *cobra.Command {
	var duration time.Duration
	var disruption stressors.ResourceDisruption
	var opts stressors.ResourceStressOptions

	cmd := &cobra.Command{
		Use:   "stress",
		Short: "resource stressor",
		Long:  "Stress CPU resource",
		RunE: func(cmd *cobra.Command, args []string) error { //nolint:revive
			agent, err := agent.Start(env, config)
			if err != nil {
				return fmt.Errorf("initializing agent: %w", err)
			}
			defer agent.Stop()

			s, err := stressors.NewResourceStressor(disruption, opts)
			if err != nil {
				return err
			}

			return s.Apply(cmd.Context(), duration)
		},
	}
	cmd.Flags().DurationVarP(&duration, "duration", "d", 0, "duration of the disruptions")
	cmd.Flags().DurationVarP(&opts.Slice, "slice", "s", 100, "CPU stress cycle in milliseconds (default 100ms)")
	cmd.Flags().IntVarP(&disruption.Load, "load", "l", 100, "CPU load percentage (default 100%)")
	cmd.Flags().IntVarP(&disruption.CPUs, "cpus", "c", 1, "number of CPUs to stress (default 1)")

	return cmd
}
