package disruptors

import (
	"context"
	"time"
)

// NetworkFaultInjector defines the interface for injecting network faults
type NetworkFaultInjector interface {
	InjectNetworkFaults(ctx context.Context, fault NetworkFault, duration time.Duration) error
}

// NetworkFault specifies a network fault to be injected
type NetworkFault struct {
	// Port to target for network disruption (0 means all ports)
	Port uint `js:"port"`
	// Protocol to target for network disruption (tcp, udp, icmp, or empty for all)
	Protocol string `js:"protocol"`
}
