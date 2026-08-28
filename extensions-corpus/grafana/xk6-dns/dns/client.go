package dns

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/miekg/dns"
	extensionapi "go.k6.io/k6-extension-api"
)

// Client resolves DNS queries with the extension-owned miekg/dns client.
type Client struct {
	client dns.Client
	vu     extensionapi.VU
}

// NewDNSClient creates a DNS client for vu.
func NewDNSClient(vu extensionapi.VU) *Client {
	return &Client{client: dns.Client{}, vu: vu}
}

// Resolve resolves query through nameserver.
func (r *Client) Resolve(ctx context.Context, query, recordType string, nameserver Nameserver) ([]string, error) {
	policy, err := networkPolicyFor(r.vu)
	if err != nil {
		return nil, err
	}
	normalizedQuery := strings.TrimSuffix(strings.ToLower(query), ".")
	if err := policy.CheckHost(ctx, normalizedQuery); err != nil {
		return nil, &Error{Name: "BlockedHostname", Message: fmt.Sprintf("DNS query blocked by host policy: %s", normalizedQuery), Kind: Refused}
	}

	network, err := networkFor(r.vu)
	if err != nil {
		return nil, err
	}
	concreteType, err := RecordTypeString(recordType)
	if err != nil {
		return nil, fmt.Errorf("resolve operation failed with %w, %s is an invalid DNS record type", errUnsupportedRecordType, recordType)
	}

	message := dns.Msg{}
	message.SetQuestion(query+".", uint16(concreteType))
	message.SetEdns0(4096, false)
	response, _, err := (&extensionDNSClient{Client: r.client, network: network}).ExchangeContext(ctx, &message, nameserver.Addr())
	if err != nil {
		return nil, fmt.Errorf("querying the DNS nameserver failed: %w", err)
	}
	if response.Rcode != dns.RcodeSuccess {
		return nil, newDNSError(response.Rcode, "DNS query failed")
	}

	var results []string
	for _, answer := range response.Answer {
		switch value := answer.(type) {
		case *dns.A:
			results = append(results, value.A.String())
		case *dns.AAAA:
			results = append(results, value.AAAA.String())
		case *dns.TXT:
			results = append(results, strings.Join(value.Txt, ""))
		default:
			return nil, fmt.Errorf("resolve operation failed with %w: unhandled DNS answer type %T", errUnsupportedRecordType, answer)
		}
	}
	return results, nil
}

// Lookup resolves hostname through the host's policy-aware resolver.
func (r *Client) Lookup(ctx context.Context, hostname string) ([]string, error) {
	policy, err := networkPolicyFor(r.vu)
	if err != nil {
		return nil, err
	}
	normalized := strings.TrimSuffix(strings.ToLower(hostname), ".")
	if err := policy.CheckHost(ctx, normalized); err != nil {
		return nil, &Error{Name: "BlockedHostname", Message: fmt.Sprintf("blocked hostname: %s", normalized), Kind: Refused}
	}

	network, err := networkFor(r.vu)
	if err != nil {
		return nil, err
	}
	ips, err := network.LookupHost(ctx, hostname)
	if err != nil {
		return nil, &Error{Name: "LookupFailed", Message: fmt.Sprintf("lookup of %s failed: %s", hostname, err), Kind: Refused}
	}
	return ips, nil
}

func networkFor(vu extensionapi.VU) (extensionapi.Network, error) {
	network, ok := vu.(extensionapi.Network)
	if !ok {
		return nil, fmt.Errorf("using DNS in the init context is not supported")
	}
	return network, nil
}

func networkPolicyFor(vu extensionapi.VU) (extensionapi.NetworkPolicy, error) {
	policy, ok := vu.(extensionapi.NetworkPolicy)
	if !ok {
		return nil, fmt.Errorf("DNS hostname policy capability is unavailable")
	}
	return policy, nil
}

type extensionDNSClient struct {
	dns.Client
	network extensionapi.Network
}

const defaultDNSTimeout = 5 * time.Second

func (c *extensionDNSClient) ExchangeContext(ctx context.Context, message *dns.Msg, address string) (*dns.Msg, time.Duration, error) {
	ctx, cancel := context.WithTimeoutCause(ctx, defaultDNSTimeout, fmt.Errorf("DNS operation timed out"))
	defer cancel()
	startedAt := time.Now()
	connection, err := c.network.DialContext(ctx, "udp", address)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = connection.Close() }()

	if deadline, ok := ctx.Deadline(); ok {
		if err := connection.SetDeadline(deadline); err != nil {
			return nil, 0, fmt.Errorf("set DNS connection deadline: %w", err)
		}
	} else if err := connection.SetDeadline(time.Now().Add(c.Timeout)); err != nil {
		return nil, 0, fmt.Errorf("set DNS connection deadline: %w", err)
	}

	data, err := message.Pack()
	if err != nil {
		return nil, 0, fmt.Errorf("serialize DNS packet: %w", err)
	}
	if _, err = connection.Write(data); err != nil {
		return nil, 0, fmt.Errorf("send DNS request: %w", err)
	}
	buffer := make([]byte, 4096)
	n, err := connection.Read(buffer)
	if err != nil {
		return nil, 0, fmt.Errorf("receive DNS response: %w", err)
	}
	response := &dns.Msg{}
	if err := response.Unpack(buffer[:n]); err != nil {
		return nil, 0, fmt.Errorf("deserialize DNS response: %w", err)
	}
	return response, time.Since(startedAt), nil
}
