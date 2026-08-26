// Package network contains a DROP connection disruptor.
package network

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/iptables"
)

// Disruptor applies network disruptions by dropping packets using iptables DROP rules.
// A filter decides which packets (PORT, PROTOCOL) are considered for dropping.
type Disruptor struct {
	Iptables iptables.Iptables
	Filter   Filter
}

// Filter decides which packets (PORT, PROTOCOL) are considered for dropping.
type Filter struct {
	Protocol string
	Port     uint
}

// ErrDurationTooShort is returned when the supplied duration is smaller than 1s.
var ErrDurationTooShort = errors.New("duration must be at least 1 second")

// Apply is applying the network drop disruption
func (d Disruptor) Apply(ctx context.Context, duration time.Duration) error {
	if duration < time.Second {
		return ErrDurationTooShort
	}
	ruleset := iptables.NewRuleSet(d.Iptables)
	//nolint:errcheck // Errors while removing rules are not actionable.
	defer ruleset.Remove()

	for _, r := range d.rules() {
		err := ruleset.Add(r)
		if err != nil {
			return err
		}
	}

	ctx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	// Wait for request duration or context cancellation to restore state
	select {
	case <-time.After(duration):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (d Disruptor) rules() []iptables.Rule {
	var args string

	args = "-j DROP"

	if d.Filter.Port != 0 {
		args = fmt.Sprintf("--dport %d %s", d.Filter.Port, args)
	}

	if d.Filter.Protocol != "" {
		args = fmt.Sprintf("-p %s %s", d.Filter.Protocol, args)
	}

	return []iptables.Rule{
		{
			// This rule drops all INPUT packets that match the filter criteria
			Table: "filter", Chain: "INPUT", Args: args,
		},
	}
}
