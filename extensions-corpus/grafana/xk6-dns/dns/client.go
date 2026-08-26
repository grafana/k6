package dns

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
	"go.k6.io/k6/v2/js/common"
	"go.k6.io/k6/v2/js/modules"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/lib/netext"
)

// Client is a DNS resolver that uses the `miekg/dns` package under the hood.
//
// It implements the Resolver interface.
type Client struct {
	// client is the DNS client used to resolve queries.
	client dns.Client

	// k6Client is lazily initialized with k6's dialer in VU context
	k6Client *k6DNSClient

	// once ensures k6Client is initialized only once
	once sync.Once

	vu modules.VU
}

// NewDNSClient creates a new Client.
func NewDNSClient(vu modules.VU) (*Client, error) {
	client := &Client{
		client: dns.Client{},
		vu:     vu,
	}

	return client, nil
}

// Resolve resolves a domain name to a slice of IP addresses using the given nameserver.
// It returns a slice of IP addresses as strings.
func (r *Client) Resolve(
	ctx context.Context,
	query, recordType string,
	nameserver Nameserver,
) ([]string, error) {
	err := ensureVUContext(r.vu, "resolve")
	if err != nil {
		return nil, common.NewInitContextError("using resolve in the init context is not supported")
	}

	// Ensure k6 client is initialized (lazy initialization)
	err = r.ensureK6Client()
	if err != nil {
		return nil, fmt.Errorf("failed to ensure dns client is initialized: %w", err)
	}

	if r.vu.State().Dialer == nil {
		return nil, fmt.Errorf("k6 internal network dialer is unavailable (internal error)")
	}

	k6Dialer, ok := r.vu.State().Dialer.(*netext.Dialer)
	if !ok {
		return nil, fmt.Errorf("k6 internal network dialer has an unexpected type (internal error)")
	}

	// If k6's dialer is available, honor its blocked hostnames configuration.
	//
	// As the dns resolution process involves querying a specific nameserver, via its IP address,
	// to query about a specific hostname, we need to check if the hostname is blocked by the k6 dialer.
	//
	// As such, we explicitly perform a check against the DNS query, and can't rely on the underlying dialer's
	// existing logic that performs the check on the actual IP address targeted by the connection (which in our
	// case is the nameserver's IP address, not the hostname).
	if k6Dialer.BlockedHostnames != nil {
		normalizedQuery := strings.TrimSuffix(strings.ToLower(query), ".")
		if _, blocked := k6Dialer.BlockedHostnames.Contains(normalizedQuery); blocked {
			return nil, &Error{
				Name:    "BlockedHostname",
				Message: fmt.Sprintf("DNS query blocked by k6 option blockHostnames: %s", normalizedQuery),
				Kind:    Refused,
			}
		}
	}

	concreteType, err := RecordTypeString(recordType)
	if err != nil {
		return nil, fmt.Errorf(
			"resolve operation failed with %w, %s is an invalid DNS record type",
			errUnsupportedRecordType,
			recordType,
		)
	}

	// Prepare the DNS query message
	//
	// Because the dns package [dns.SetQuestion] function expects specific
	// uint16 values for the record type, and we don't want to leak that
	// to our public API, we need to convert our RecordType to the
	// corresponding uint16 value.
	message := dns.Msg{}
	message.SetQuestion(query+".", uint16(concreteType))
	message.SetEdns0(4096, false)

	// Query the nameserver using k6's dialer
	response, _, err := r.k6Client.ExchangeContext(ctx, &message, nameserver.Addr())
	if err != nil {
		return nil, fmt.Errorf("querying the DNS nameserver failed: %w", err)
	}

	if response.Rcode != dns.RcodeSuccess {
		return nil, newDNSError(response.Rcode, "DNS query failed")
	}

	var results []string
	for _, a := range response.Answer {
		switch t := a.(type) {
		case *dns.A:
			results = append(results, t.A.String())
		case *dns.AAAA:
			results = append(results, t.AAAA.String())
		case *dns.TXT:
			results = append(results, strings.Join(t.Txt, ""))
		default:
			return nil, fmt.Errorf(
				"resolve operation failed with %w: unhandled DNS answer type %T",
				errUnsupportedRecordType,
				a,
			)
		}
	}

	return results, nil
}

// Lookup resolves a domain name to a slice of IP addresses.
//
// It prefers the k6 VU dialer's resolver if available, so that k6 networking
// options (e.g. blockHostnames) are honored. It falls back to the system's
// default resolver otherwise.
func (r *Client) Lookup(ctx context.Context, hostname string) ([]string, error) {
	err := ensureVUContext(r.vu, "lookup")
	if err != nil {
		return nil, err
	}

	if r.vu.State().Dialer == nil {
		return nil, fmt.Errorf("unable to perform DNS lookup; reason: k6 internal network dialer is not available")
	}

	// If k6's dialer is available, honor its blocked hostnames' configuration.
	if d, ok := r.vu.State().Dialer.(*netext.Dialer); ok && d.BlockedHostnames != nil {
		normalized := strings.TrimSuffix(strings.ToLower(hostname), ".")
		if _, blocked := d.BlockedHostnames.Contains(normalized); blocked {
			return nil, &Error{
				Name:    "BlockedHostname",
				Message: fmt.Sprintf("blocked hostname: %s", normalized),
				Kind:    Refused,
			}
		}
	}

	resolver := &net.Resolver{
		Dial: r.vu.State().Dialer.DialContext,
	}

	ips, err := resolver.LookupHost(ctx, hostname)
	if err != nil {
		return nil, &Error{
			Name:    "LookupFailed",
			Message: fmt.Sprintf("lookup of %s failed: %s", hostname, err.Error()),
			Kind:    Refused,
		}
	}

	return ips, nil
}

// k6DNSClient wraps dns.Client to use k6's dialer
// This ensures k6's networking options (blockHostnames, blacklistIPs) are respected
type k6DNSClient struct {
	dns.Client
	k6Dialer lib.DialContexter
}

// defaultDNSTimeout is the default timeout for DNS operations.
const defaultDNSTimeout = 5 * time.Second

// ExchangeContext overrides the default ExchangeContext to use k6's dialer
func (c *k6DNSClient) ExchangeContext(
	ctx context.Context,
	m *dns.Msg,
	address string,
) (*dns.Msg, time.Duration, error) {
	ctx, cancel := context.WithTimeoutCause(ctx, defaultDNSTimeout, fmt.Errorf("DNS operation timed out"))
	defer cancel()

	// If k6 dialer is not available, fall back to standard DNS client behavior
	if c.k6Dialer == nil {
		return c.Client.ExchangeContext(ctx, m, address)
	}

	start := time.Now()

	// Create a connection using k6's dialer
	conn, err := c.k6Dialer.DialContext(ctx, "udp", address)
	if err != nil {
		return nil, 0, err
	}
	defer func() {
		closeErr := conn.Close()
		if closeErr != nil {
			log.Fatalf("failed to close k6 DNS connection: %v", closeErr)
		}
	}()

	// Set a reasonable deadline for the operation
	var deadlineErr error
	if deadline, ok := ctx.Deadline(); ok {
		deadlineErr = conn.SetDeadline(deadline)
	} else {
		deadlineErr = conn.SetDeadline(time.Now().Add(c.Timeout))
	}
	if deadlineErr != nil {
		return nil, 0, fmt.Errorf("unable to set dns connection deadline; reason: %w", deadlineErr)
	}

	// Pack the DNS message and write it
	data, err := m.Pack()
	if err != nil {
		return nil, 0, fmt.Errorf("unable to serialize DNS packet; reason: %w", err)
	}

	_, err = conn.Write(data)
	if err != nil {
		return nil, 0, fmt.Errorf("unable to send request DNS packet; reason: %w", err)
	}

	// Read the response
	buffer := make([]byte, 4096)
	n, err := conn.Read(buffer)
	if err != nil {
		return nil, 0, fmt.Errorf("unable to receive response DNS packet; reason: %w", err)
	}

	// Unpack the response
	response := &dns.Msg{}
	err = response.Unpack(buffer[:n])
	if err != nil {
		return nil, 0, fmt.Errorf("unable to deserialize response DNS packet; reason: %w", err)
	}

	totalTime := time.Since(start)
	return response, totalTime, nil
}

// ensureK6Client lazily initializes the k6 DNS client with k6's dialer.
// This must be called in VU context where the dialer is available.
func (r *Client) ensureK6Client() (err error) {
	r.once.Do(func() {
		contextErr := ensureVUContext(r.vu, "dns client")
		if contextErr != nil {
			err = common.NewInitContextError("using dns module in the init context is not supported")
			return
		}
		if r.vu.State().Dialer == nil {
			err = fmt.Errorf("inconsisten k6 internal state, no network dialer available")
			return
		}

		// Create the k6 DNS client with k6's dialer
		r.k6Client = &k6DNSClient{
			Client:   dns.Client{},
			k6Dialer: r.vu.State().Dialer,
		}
	})

	return err
}
