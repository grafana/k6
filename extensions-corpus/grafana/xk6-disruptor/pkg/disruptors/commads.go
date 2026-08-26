package disruptors

import (
	"fmt"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/types/intstr"
	"github.com/grafana/xk6-disruptor/pkg/utils"

	corev1 "k8s.io/api/core/v1"
)

func buildGrpcFaultCmd(
	targetAddress string,
	fault GrpcFault,
	duration time.Duration,
	options GrpcDisruptionOptions,
) []string {
	cmd := []string{
		"xk6-disruptor-agent",
		"grpc",
		"-d", utils.DurationSeconds(duration),
		"-t", fmt.Sprint(fault.Port),
	}

	// TODO: make port mandatory
	if fault.Port != intstr.NullValue {
		cmd = append(cmd, "-t", fault.Port.Str())
	}

	if fault.AverageDelay > 0 {
		cmd = append(
			cmd,
			"-a",
			utils.DurationMillSeconds(fault.AverageDelay),
			"-v",
			utils.DurationMillSeconds(fault.DelayVariation),
		)
	}

	if fault.ErrorRate > 0 {
		cmd = append(
			cmd,
			"-s",
			fmt.Sprint(fault.StatusCode),
			"-r",
			fmt.Sprint(fault.ErrorRate),
		)
		if fault.StatusMessage != "" {
			cmd = append(cmd, "-m", fault.StatusMessage)
		}
	}

	if len(fault.Exclude) > 0 {
		cmd = append(cmd, "-x", fault.Exclude)
	}

	if options.ProxyPort != 0 {
		cmd = append(cmd, "-p", fmt.Sprint(options.ProxyPort))
	}

	cmd = append(cmd, "--upstream-host", targetAddress)

	return cmd
}

func buildHTTPFaultCmd(
	targetAddress string,
	fault HTTPFault,
	duration time.Duration,
	options HTTPDisruptionOptions,
) []string {
	cmd := []string{
		"xk6-disruptor-agent",
		"http",
		"-d", utils.DurationSeconds(duration),
	}

	// TODO: make port mandatory
	if fault.Port != intstr.NullValue {
		cmd = append(cmd, "-t", fault.Port.Str())
	}

	if fault.AverageDelay > 0 {
		cmd = append(
			cmd,
			"-a",
			utils.DurationMillSeconds(fault.AverageDelay),
			"-v",
			utils.DurationMillSeconds(fault.DelayVariation),
		)
	}

	if fault.ErrorRate > 0 {
		cmd = append(
			cmd,
			"-e",
			fmt.Sprint(fault.ErrorCode),
			"-r",
			fmt.Sprint(fault.ErrorRate),
		)
		if fault.ErrorBody != "" {
			cmd = append(cmd, "-b", fault.ErrorBody)
		}
	}

	if len(fault.Exclude) > 0 {
		cmd = append(cmd, "-x", fault.Exclude)
	}

	if options.ProxyPort != 0 {
		cmd = append(cmd, "-p", fmt.Sprint(options.ProxyPort))
	}

	cmd = append(cmd, "--upstream-host", targetAddress)

	return cmd
}

func buildNetworkFaultCmd(fault NetworkFault, duration time.Duration) []string {
	cmd := []string{
		"xk6-disruptor-agent",
		"network-drop",
		"-d", utils.DurationSeconds(duration),
	}

	if fault.Port != 0 {
		cmd = append(cmd, "-p", fmt.Sprint(fault.Port))
	}

	if fault.Protocol != "" {
		cmd = append(cmd, "-P", fault.Protocol)
	}

	return cmd
}

func buildCleanupCmd() []string {
	return []string{"xk6-disruptor-agent", "cleanup"}
}

// PodHTTPFaultCommand implements the PodVisitCommands interface for injecting
// HttpFaults in a Pod
type PodHTTPFaultCommand struct {
	fault    HTTPFault
	duration time.Duration
	options  HTTPDisruptionOptions
}

// Commands return the command for injecting a HttpFault in a Pod
func (c PodHTTPFaultCommand) Commands(pod corev1.Pod) (VisitCommands, error) {
	if utils.HasHostNetwork(pod) {
		return VisitCommands{}, fmt.Errorf("fault cannot be safely injected because pod %q uses hostNetwork", pod.Name)
	}

	// find the container port for fault injection
	port, err := utils.FindPort(c.fault.Port, pod)
	if err != nil {
		return VisitCommands{}, err
	}
	podFault := c.fault
	podFault.Port = port

	targetAddress, err := utils.PodIP(pod)
	if err != nil {
		return VisitCommands{}, err
	}

	return VisitCommands{
		Exec:    buildHTTPFaultCmd(targetAddress, podFault, c.duration, c.options),
		Cleanup: buildCleanupCmd(),
	}, nil
}

// PodGrpcFaultCommand implements the PodVisitCommands interface for injecting GrpcFaults in a Pod
type PodGrpcFaultCommand struct {
	fault    GrpcFault
	duration time.Duration
	options  GrpcDisruptionOptions
}

// Commands return the command for injecting a GrpcFault in a Pod
func (c PodGrpcFaultCommand) Commands(pod corev1.Pod) (VisitCommands, error) {
	if utils.HasHostNetwork(pod) {
		return VisitCommands{}, fmt.Errorf("fault cannot be safely injected because pod %q uses hostNetwork", pod.Name)
	}

	// find the container port for fault injection
	port, err := utils.FindPort(c.fault.Port, pod)
	if err != nil {
		return VisitCommands{}, err
	}
	podFault := c.fault
	podFault.Port = port

	targetAddress, err := utils.PodIP(pod)
	if err != nil {
		return VisitCommands{}, err
	}

	return VisitCommands{
		Exec:    buildGrpcFaultCmd(targetAddress, c.fault, c.duration, c.options),
		Cleanup: buildCleanupCmd(),
	}, nil
}

// PodNetworkFaultCommand implements the PodVisitCommands interface for injecting NetworkFaults in a Pod
type PodNetworkFaultCommand struct {
	fault    NetworkFault
	duration time.Duration
}

// Commands return the command for injecting a NetworkFault in a Pod
func (c PodNetworkFaultCommand) Commands(_ corev1.Pod) (VisitCommands, error) {
	return VisitCommands{
		Exec:    buildNetworkFaultCmd(c.fault, c.duration),
		Cleanup: buildCleanupCmd(),
	}, nil
}
