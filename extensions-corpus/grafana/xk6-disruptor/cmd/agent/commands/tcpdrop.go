package commands

import (
	"fmt"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/agent"
	"github.com/grafana/xk6-disruptor/pkg/agent/tcpconn"
	"github.com/grafana/xk6-disruptor/pkg/iptables"
	"github.com/grafana/xk6-disruptor/pkg/runtime"
	"github.com/spf13/cobra"
)

// BuildTCPDropCmd returns a cobra command with the specification of the tcp-drop command.
func BuildTCPDropCmd(env runtime.Environment, config *agent.Config) *cobra.Command {
	var duration time.Duration
	filter := tcpconn.Filter{}
	dropRate := 0.0

	cmd := &cobra.Command{
		Use:   "tcp-drop",
		Short: "tcp connection drop",
		Long: "Disrupts TCP connections by terminating a certain percentage of them." +
			" Requires either to be run as root, or the NET_ADMIN capability.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if filter.Port == 0 {
				return fmt.Errorf("target port for fault injection is required")
			}

			agent, err := agent.Start(env, config)
			if err != nil {
				return fmt.Errorf("initializing agent: %w", err)
			}

			defer agent.Stop()

			dropper := tcpconn.TCPConnectionDropper{
				DropRate: dropRate,
			}

			disruptor := tcpconn.Disruptor{
				Iptables: iptables.New(env.Executor()),
				Filter:   filter,
				Dropper:  dropper,
			}

			return agent.ApplyDisruption(cmd.Context(), disruptor, duration)
		},
	}

	cmd.Flags().DurationVarP(&duration, "duration", "d", 0, "duration of the disruptions")
	cmd.Flags().UintVarP(&filter.Port, "port", "p", 0, "target port of the connections to be disrupted")
	cmd.Flags().Float64VarP(&dropRate, "rate", "r", 0, "fraction of connections to drop")

	return cmd
}
