package commands

import (
	"syscall"

	"github.com/grafana/xk6-disruptor/pkg/runtime"
	"github.com/spf13/cobra"
)

// BuiltCleanupCmd returns a cobra command with the specification of the kill command
func BuiltCleanupCmd(env runtime.Environment) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "stops any ongoing fault injection and cleans resources",
		RunE: func(cmd *cobra.Command, args []string) error { //nolint:revive
			runningProcess := env.Lock().Owner()
			// no instance is currently running
			if runningProcess == -1 {
				return nil
			}

			return syscall.Kill(runningProcess, syscall.SIGTERM)

			// TODO: cleanup resources (e.g iptables)
		},
	}

	return cmd
}
