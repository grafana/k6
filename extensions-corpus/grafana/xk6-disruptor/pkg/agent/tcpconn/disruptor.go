// Package tcpconn contains a TCP connection disruptor.
package tcpconn

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/florianl/go-nfqueue/v2"
	"github.com/grafana/xk6-disruptor/pkg/iptables"
)

// Disruptor applies TCP Connection disruptions by dropping connections according to a Dropper. A filter decides which
// connections are considered for dropping.
type Disruptor struct {
	Iptables iptables.Iptables
	Dropper  Dropper
	Filter   Filter
}

// Filter holds the matchers used to know which traffic should be intercepted.
type Filter struct {
	// Port is the target port to match which connections will be intercepted.
	Port uint
}

// ErrDurationTooShort is returned when the supplied duration is smaller than 1s.
var ErrDurationTooShort = errors.New("duration must be at least 1 second")

// Apply starts the disruption by subjecting connections that match the configured Filter to the Dropper.
func (d Disruptor) Apply(ctx context.Context, duration time.Duration) error {
	if duration < time.Second {
		return ErrDurationTooShort
	}

	ruleset := iptables.NewRuleSet(d.Iptables)
	//nolint:errcheck // Errors while removing rules are not actionable.
	defer ruleset.Remove()

	config := randomNFQConfig()
	for _, r := range d.rules(config) {
		err := ruleset.Add(r)
		if err != nil {
			return err
		}
	}

	queue, err := nfqueue.Open(&nfqueue.Config{
		NfQueue:  config.queueID,
		Copymode: nfqueue.NfQnlCopyPacket, // Copymode must be set to NfQnlCopyPacket to be able to read the packet.

		// TODO: Refine this magic value. Larger values will cause nfqueue to error such as:
		// netlink receive: recvmsg: no buffer space available
		// Likely this means that we're trying to use too much memory for this queue.
		MaxQueueLen:  32,
		MaxPacketLen: 0xffff, // TODO: This can probably be smaller for IPv4 on top of ethernet (1500 mtu).
	})
	if err != nil {
		return fmt.Errorf("creating nfqueue: %w", err)
	}

	//nolint:errcheck
	defer queue.Close()

	ctx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	errCh := make(chan error)

	err = queue.RegisterWithErrorFunc(ctx,
		func(packet nfqueue.Attribute) int {
			if d.Dropper.Drop(*packet.Payload) {
				_ = queue.SetVerdictWithMark(*packet.PacketID, nfqueue.NfRepeat, int(config.rejectMark))
				return 0
			}

			_ = queue.SetVerdict(*packet.PacketID, nfqueue.NfAccept)

			return 0
		},
		func(err error) int {
			errCh <- err
			return 1
		},
	)
	if err != nil {
		return fmt.Errorf("registering nqueue handlers: %w", err)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return fmt.Errorf("reading packet from NFQueue: %w", err)
	}
}

// rules returns the iptables rules that need to be set in place for the disruption to work.
// These rules are safe by default, meaning that if for some reason the rules are left over, no packet will be dropped.
func (d Disruptor) rules(c nfqConfig) []iptables.Rule {
	return []iptables.Rule{
		{
			// This rule rejects with tcp-reset traffic arriving to the disruption port if it has the RejectMark set by
			// the queue when Reject() is called on a packet. Packets that are Reject()ed are requeued and thus will
			// traverse this rule again even if they didn't the first time they arrived, when they weren't marked.
			Table: "filter", Chain: "INPUT", Args: fmt.Sprintf(
				"-p tcp --dport %d -m mark --mark %d -j REJECT --reject-with tcp-reset",
				d.Filter.Port, c.rejectMark,
			),
		},
		{
			// This rule sends other (non-marked) traffic to the queue, so it can make a decision over whether to
			// drop it or not.
			// --queue-bypass instruct netfilter to ACCEPT packets if nothing is listening on this queue.
			Table: "filter", Chain: "INPUT", Args: fmt.Sprintf(
				"-p tcp --dport %d -j NFQUEUE --queue-num %d --queue-bypass",
				d.Filter.Port, c.queueID,
			),
		},
	}
}

// nfqConfig contains netfilter queue IDs that are used to build the iptables rules and set the userspace packet
// listeners.
type nfqConfig struct {
	// queueID is an arbitrary integer used to identify a queue where a handler listens and a disruptor redirects target
	// packets.
	queueID uint16
	// rejectMark is an arbitrary integer which the handler uses to mark packets that need to be dropped.
	rejectMark uint32
}

// randomNFQConfig returns a NFQConfig with two random integers to be used as queue IDs and reject mark.
// To ensure the numbers are not zero, which have a special meaning for netfilter, they are ORed with 0b1, as adding 1
// can actually result in the number overflowing and becoming zero.
func randomNFQConfig() nfqConfig {
	return nfqConfig{
		queueID:    uint16(rand.Int31()) | 0b1, //nolint:gosec  //integer overflow is not a concern here
		rejectMark: uint32(rand.Int31()) | 0b1, //nolint:gosec
	}
}
