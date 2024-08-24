package netext

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync/atomic"
	"time"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/metrics"
)

// Dialer wraps net.Dialer and provides k6 specific functionality -
// tracing, blacklists and DNS cache and aliases.
type Dialer struct {
	net.Dialer

	Resolver         Resolver
	Blacklist        []*lib.IPNet
	BlockedHostnames *types.HostnameTrie
	Hosts            *types.Hosts

	BytesRead    int64
	BytesWritten int64
}

// NewDialer constructs a new Dialer with the given DNS resolver.
func NewDialer(dialer net.Dialer, resolver Resolver) *Dialer {
	return &Dialer{
		Dialer:   dialer,
		Resolver: resolver,
	}
}

// BlackListedIPError is an error that is returned when a given IP is blacklisted
type BlackListedIPError struct {
	ip  net.IP
	net *lib.IPNet
}

func (b BlackListedIPError) Error() string {
	return fmt.Sprintf("IP (%s) is in a blacklisted range (%s)", b.ip, b.net)
}

// BlockedHostError is returned when a given hostname is blocked
type BlockedHostError struct {
	hostname string
	match    string
}

func (b BlockedHostError) Error() string {
	return fmt.Sprintf("hostname (%s) is in a blocked pattern (%s)", b.hostname, b.match)
}

// DialContext wraps the net.Dialer.DialContext and handles the k6 specifics
func (d *Dialer) DialContext(ctx context.Context, proto, addr string) (net.Conn, error) {
	dialAddr, err := d.getDialAddr(addr)
	if err != nil {
		return nil, err
	}
	conn, err := d.Dialer.DialContext(ctx, proto, dialAddr)
	if err != nil {
		return nil, err
	}
	conn = &Conn{conn, &d.BytesRead, &d.BytesWritten}
	return conn, err
}

// IOSamples returns samples for data send and received since it last call and zeros out.
// It uses the provided time as the sample time and tags and builtinMetrics to build the samples.
func (d *Dialer) IOSamples(
	sampleTime time.Time, ctm metrics.TagsAndMeta, builtinMetrics *metrics.BuiltinMetrics,
) metrics.SampleContainer {
	bytesWritten := atomic.SwapInt64(&d.BytesWritten, 0)
	bytesRead := atomic.SwapInt64(&d.BytesRead, 0)
	return metrics.Samples([]metrics.Sample{
		{
			TimeSeries: metrics.TimeSeries{
				Metric: builtinMetrics.DataSent,
				Tags:   ctm.Tags,
			},
			Time:     sampleTime,
			Metadata: ctm.Metadata,
			Value:    float64(bytesWritten),
		},
		{
			TimeSeries: metrics.TimeSeries{
				Metric: builtinMetrics.DataReceived,
				Tags:   ctm.Tags,
			},
			Time:     sampleTime,
			Metadata: ctm.Metadata,
			Value:    float64(bytesRead),
		},
	})
}

func (d *Dialer) getDialAddr(addr string) (string, error) {
	remote, err := d.findRemote(addr)
	if err != nil {
		return "", err
	}

	for _, ipnet := range d.Blacklist {
		if ipnet.Contains(remote.IP) {
			return "", BlackListedIPError{ip: remote.IP, net: ipnet}
		}
	}

	return remote.String(), nil
}

func (d *Dialer) findRemote(addr string) (*types.Host, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	ip := net.ParseIP(host)
	if d.BlockedHostnames != nil && ip == nil {
		if match, blocked := d.BlockedHostnames.Contains(host); blocked {
			return nil, BlockedHostError{hostname: host, match: match}
		}
	}

	if d.Hosts != nil {
		remote, e := d.getConfiguredHost(addr, host, port)
		if e != nil || remote != nil {
			return remote, e
		}
	}

	if ip != nil {
		return types.NewHost(ip, port)
	}

	ip, err = d.Resolver.LookupIP(host)
	if err != nil {
		return nil, err
	}

	if ip == nil {
		return nil, fmt.Errorf("lookup %s: no such host", host)
	}

	return types.NewHost(ip, port)
}

func (d *Dialer) getConfiguredHost(addr, host, port string) (*types.Host, error) {
	if remote := d.Hosts.Match(addr); remote != nil {
		return remote, nil
	}

	if remote := d.Hosts.Match(host); remote != nil {
		if remote.Port != 0 || port == "" {
			return remote, nil
		}

		newPort, err := strconv.Atoi(port)
		if err != nil {
			return nil, err
		}

		newRemote := *remote
		newRemote.Port = newPort

		return &newRemote, nil
	}

	return nil, nil //nolint:nilnil
}

// Conn wraps net.Conn and keeps track of sent and received data size
type Conn struct {
	net.Conn

	BytesRead, BytesWritten *int64
}

func (c *Conn) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	if n > 0 {
		atomic.AddInt64(c.BytesRead, int64(n))
	}
	return n, err
}

func (c *Conn) Write(b []byte) (int, error) {
	n, err := c.Conn.Write(b)
	if n > 0 {
		atomic.AddInt64(c.BytesWritten, int64(n))
	}
	return n, err
}
