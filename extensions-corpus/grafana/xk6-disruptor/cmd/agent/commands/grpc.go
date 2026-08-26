package commands

import (
	"fmt"
	"net"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/agent"
	"github.com/grafana/xk6-disruptor/pkg/agent/protocol"
	"github.com/grafana/xk6-disruptor/pkg/agent/protocol/grpc"
	"github.com/grafana/xk6-disruptor/pkg/iptables"
	"github.com/grafana/xk6-disruptor/pkg/runtime"

	"github.com/spf13/cobra"
)

// BuildGrpcCmd returns a cobra command with the specification of the grpc command
//
//nolint:funlen
func BuildGrpcCmd(env runtime.Environment, config *agent.Config) *cobra.Command {
	disruption := grpc.Disruption{}
	var duration time.Duration
	var port uint
	var upstreamHost string
	var targetPort uint
	transparent := true

	cmd := &cobra.Command{
		Use:   "grpc",
		Short: "grpc disruptor",
		Long: "Disrupts http request by introducing delays and errors." +
			" When running as a transparent proxy requires NET_ADMIM capabilities for setting" +
			" iptable rules.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if targetPort == 0 {
				return fmt.Errorf("target port for fault injection is required")
			}

			if transparent && (upstreamHost == "localhost" || upstreamHost == "127.0.0.1") {
				// When running in transparent mode, the Redirector will also redirect traffic directed to 127.0.0.1 to
				// the proxy. Using 127.0.0.1 as the proxy upstream would cause a redirection loop.
				return fmt.Errorf("upstream host cannot be localhost when running in transparent mode")
			}

			agent, err := agent.Start(env, config)
			if err != nil {
				return fmt.Errorf("initializing agent: %w", err)
			}

			defer agent.Stop()

			listenAddress := net.JoinHostPort("", fmt.Sprint(port))
			upstreamAddress := net.JoinHostPort(upstreamHost, fmt.Sprint(targetPort))

			listener, err := net.Listen("tcp", listenAddress)
			if err != nil {
				return fmt.Errorf("setting up listener at %q: %w", listenAddress, err)
			}

			proxy, err := grpc.NewProxy(listener, upstreamAddress, disruption)
			if err != nil {
				return err
			}

			// Redirect traffic to the proxy
			var redirector protocol.TrafficRedirector
			if transparent {
				tr := &protocol.TrafficRedirectionSpec{
					DestinationPort: targetPort, // Redirect traffic from the application (target) port...
					RedirectPort:    port,       // to the proxy port.
				}

				redirector, err = protocol.NewTrafficRedirector(tr, iptables.New(env.Executor()))
				if err != nil {
					return err
				}
			} else {
				redirector = protocol.NoopTrafficRedirector()
			}

			disruptor, err := protocol.NewDisruptor(
				env.Executor(),
				proxy,
				redirector,
			)
			if err != nil {
				return err
			}

			return agent.ApplyDisruption(cmd.Context(), disruptor, duration)
		},
	}
	cmd.Flags().DurationVarP(&duration, "duration", "d", 0, "duration of the disruptions")
	cmd.Flags().DurationVarP(&disruption.AverageDelay, "average-delay", "a", 0, "average request delay")
	cmd.Flags().DurationVarP(&disruption.DelayVariation, "delay-variation", "v", 0, "variation in request delay")
	cmd.Flags().Uint32VarP(&disruption.StatusCode, "status", "s", 0, "status code")
	cmd.Flags().Float32VarP(&disruption.ErrorRate, "rate", "r", 0, "error rate")
	cmd.Flags().StringVarP(&disruption.StatusMessage, "message", "m", "", "error message for injected faults")
	cmd.Flags().UintVarP(&port, "port", "p", 8000, "port the proxy will listen to")
	cmd.Flags().UintVarP(&targetPort, "target", "t", 0, "port the proxy will redirect request to")
	cmd.Flags().StringSliceVarP(&disruption.Excluded, "exclude", "x", []string{}, "comma-separated list of grpc services"+
		" to be excluded from disruption")
	cmd.Flags().BoolVar(&transparent, "transparent", true, "run as transparent proxy")
	cmd.Flags().StringVar(&upstreamHost, "upstream-host", "localhost",
		"upstream host to redirect traffic to")

	return cmd
}
