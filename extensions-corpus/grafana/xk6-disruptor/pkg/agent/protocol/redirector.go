package protocol

import (
	"fmt"

	"github.com/grafana/xk6-disruptor/pkg/iptables"
)

// TrafficRedirectionSpec specifies the redirection of traffic to a destination
type TrafficRedirectionSpec struct {
	// DestinationPort is the original destination port where the upstream application listens.
	DestinationPort uint
	// RedirectPort is the port where the traffic should be redirected to.
	// Typically, this would be where a transparent proxy is listening.
	RedirectPort uint
}

// Redirector is an implementation of TrafficRedirector that uses iptables rules.
type Redirector struct {
	*TrafficRedirectionSpec
	iptables iptables.Iptables
}

// NewTrafficRedirector creates instances of an iptables traffic redirector
func NewTrafficRedirector(
	tr *TrafficRedirectionSpec,
	iptables iptables.Iptables,
) (*Redirector, error) {
	if tr.DestinationPort == 0 || tr.RedirectPort == 0 {
		return nil, fmt.Errorf("DestinationPort and RedirectPort must be specified")
	}

	if tr.DestinationPort == tr.RedirectPort {
		return nil, fmt.Errorf(
			"DestinationPort (%d) and RedirectPort (%d) must be different",
			tr.DestinationPort,
			tr.RedirectPort,
		)
	}

	return &Redirector{
		TrafficRedirectionSpec: tr,
		iptables:               iptables,
	}, nil
}

// rules returns the iptables rules that cause traffic to be forwarded according to the spec.
// The four returned rules fulfill two different purposes.
// - Redirect traffic to the target application through the proxy, excluding traffic from the proxy itself.
// - Reset existing, non-redirected connections to the target application, except those of the proxy itself.
// Excluding traffic from the proxy from the goals above is not entirely straightforward, mainly because the proxy,
// just like `kubectl port-forward` and sidecars, connect _from_ the loopback address 127.0.0.1.
//
// To achieve this, we take advantage of the fact that the proxy knows the pod IP and connects to it, instead of to the
// loopback address like sidecars and kubectl port-forward does. This allows us to distinguish the proxy traffic from
// port-forwarded traffic, as while both traverse the `lo` interface, the former targets the pod IP while the latter
// targets the loopback IP.
//
// +-----------+---------------+------------------------+
// | Interface | From/To       | What                   |
// +-----------+---------------+------------------------+
// | ! lo      | Anywhere      | Outside traffic        |
// +-----------+---------------+------------------------+
// | lo        | 127.0.0.0/8   | Port-forwarded traffic |
// +-----------+---------------+------------------------+
// | lo        | ! 127.0.0.0/8 | Proxy traffic          |
// +-----------+---------------+------------------------+
func (tr *Redirector) rules() []iptables.Rule {
	// redirectLocalRule is a netfilter rule that intercepts locally-originated traffic, such as that coming from sidecars
	// or `kubectl port-forward, directed to the application and redirects it to the proxy.
	// As per https://upload.wikimedia.org/wikipedia/commons/3/37/Netfilter-packet-flow.svg, locally originated traffic
	// traverses OUTPUT instead of PREROUTING.
	// Traffic created by the proxy itself to the application also traverses this chain, but is not redirected by this rule
	// as the proxy targets the pod IP and not the loopback address.
	redirectLocalRule := iptables.Rule{
		Table: "nat",
		Chain: "OUTPUT", // For local traffic
		Args: "-s 127.0.0.0/8 -d 127.0.0.1/32 " + // Coming from and directed to localhost, i.e. not the pod IP.
			fmt.Sprintf("-p tcp --dport %d ", tr.DestinationPort) + // Sent to the upstream application's port
			fmt.Sprintf("-j REDIRECT --to-port %d", tr.RedirectPort), // Forward it to the proxy address
	}

	// redirectExternalRule is a netfilter rule that intercepts external traffic directed to the application and redirects
	// it to the proxy.
	// Traffic created by the proxy itself to the application traverses is not redirected by this rule as it traverses the
	// OUTPUT chain, not PREROUTING.
	redirectExternalRule := iptables.Rule{
		Table: "nat",
		Chain: "PREROUTING", // For remote traffic
		Args: "! -i lo " + // Not coming form loopback. Technically not needed, but doesn't hurt and helps readability.
			fmt.Sprintf("-p tcp --dport %d ", tr.DestinationPort) + // Sent to the upstream application's port
			fmt.Sprintf("-j REDIRECT --to-port %d", tr.RedirectPort), // Forward it to the proxy address
	}

	// resetLocalRule is a netfilter rule that resets established connections (i.e. that have not been redirected) coming
	// to and from the loopback address.
	// This rule matches connections from sidecars and `kubectl port-forward`.
	// Connections from the proxy itself do not match this rule, as although they flow through `lo`, they are directed to
	// the pod's external IP and not the loopback address.
	resetLocalRule := iptables.Rule{
		Table: "filter",
		Chain: "INPUT", // For traffic traversing the INPUT chain
		Args: "-i lo " + // On the loopback interface
			"-s 127.0.0.0/8 -d 127.0.0.1/32 " + // Coming from and directed to localhost
			fmt.Sprintf("-p tcp --dport %d ", tr.DestinationPort) + // Directed to the upstream application's port
			"-m state --state ESTABLISHED " + // That are already ESTABLISHED, i.e. not before they are redirected
			"-j REJECT --reject-with tcp-reset", // Reject it
	}

	// resetExternalRule is a netfilter rule that resets established connections (i.e. that have not been redirected)
	// coming from anywhere except the local IP.
	// This rule matches external connections to the pod's IP address.
	// Connections from the proxy itself do not match this rule, as they flow through the `lo` interface.
	resetExternalRule := iptables.Rule{
		Table: "filter",
		Chain: "INPUT", // For traffic traversing the INPUT chain
		Args: "! -i lo " + // Not coming form loopback. This is technically not needed as loopback traffic does not
			// traverse INPUT, but helps with explicitness.
			fmt.Sprintf("-p tcp --dport %d ", tr.DestinationPort) + // Directed to the upstream application's port
			"-m state --state ESTABLISHED " + // That are already ESTABLISHED, i.e. not before they are redirected
			"-j REJECT --reject-with tcp-reset", // Reject it
	}

	return []iptables.Rule{
		redirectLocalRule,
		redirectExternalRule,
		resetLocalRule,
		resetExternalRule,
	}
}

// proxyResetRule returns a netfilter rule that rejects traffic to the proxy.
// This rule is set up after injection finishes to kill any leftover connection to the proxy.
// TODO: Run some tests to check if this is really necessary, as the proxy may already be killing conns on termination.
func (tr *Redirector) resetProxyRule() iptables.Rule {
	return iptables.Rule{
		Table: "filter",
		Chain: "INPUT",
		Args: fmt.Sprintf("-p tcp --dport %d ", tr.RedirectPort) + // Directed to the proxy port
			"-j REJECT --reject-with tcp-reset", // Reject it
	}
}

// Start applies the TrafficRedirect
func (tr *Redirector) Start() error {
	// Remove reset rule for the proxy in case it exists from a previous run.
	_ = tr.iptables.Remove(tr.resetProxyRule())

	// TODO: Use iptables.RuleSet instead, which takes care of automatically cleaning the rules.
	for _, rule := range tr.rules() {
		err := tr.iptables.Add(rule)
		if err != nil {
			return fmt.Errorf("adding rules: %w", err)
		}
	}

	return nil
}

// Stop stops the TrafficRedirect.
// Stop will continue attempting to remove all the rules it deployed even if removing one fails.
func (tr *Redirector) Stop() error {
	var errors []error

	for _, rule := range tr.rules() {
		err := tr.iptables.Remove(rule)
		if err != nil {
			errors = append(errors, err)
		}
	}

	if err := tr.iptables.Add(tr.resetProxyRule()); err != nil {
		errors = append(errors, err)
	}

	// TODO: Replace with errors.Join.
	if len(errors) > 0 {
		return errors[0]
	}

	return nil
}
