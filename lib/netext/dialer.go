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
	Hosts            map[string]*lib.HostAddress

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

// GetTrail creates a new NetTrail instance with the Dialer
// sent and received data metrics and the supplied times and tags.
// TODO: Refactor this according to
// https://github.com/k6io/k6/pull/1203#discussion_r337938370
func (d *Dialer) GetTrail(
	startTime, endTime time.Time, fullIteration bool, emitIterations bool, tags *metrics.SampleTags,
	builtinMetrics *metrics.BuiltinMetrics,
) *NetTrail {
	bytesWritten := atomic.SwapInt64(&d.BytesWritten, 0)
	bytesRead := atomic.SwapInt64(&d.BytesRead, 0)
	samples := []metrics.Sample{
		{
			Time:   endTime,
			Metric: builtinMetrics.DataSent,
			Value:  float64(bytesWritten),
			Tags:   tags,
		},
		{
			Time:   endTime,
			Metric: builtinMetrics.DataReceived,
			Value:  float64(bytesRead),
			Tags:   tags,
		},
	}
	if fullIteration {
		samples = append(samples, metrics.Sample{
			Time:   endTime,
			Metric: builtinMetrics.IterationDuration,
			Value:  metrics.D(endTime.Sub(startTime)),
			Tags:   tags,
		})
		if emitIterations {
			samples = append(samples, metrics.Sample{
				Time:   endTime,
				Metric: builtinMetrics.Iterations,
				Value:  1,
				Tags:   tags,
			})
		}
	}

	return &NetTrail{
		BytesRead:     bytesRead,
		BytesWritten:  bytesWritten,
		FullIteration: fullIteration,
		StartTime:     startTime,
		EndTime:       endTime,
		Tags:          tags,
		Samples:       samples,
	}
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

func (d *Dialer) findRemote(addr string) (*lib.HostAddress, error) {
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

	remote, err := d.getConfiguredHost(addr, host, port)
	if err != nil || remote != nil {
		return remote, err
	}

	if ip != nil {
		return lib.NewHostAddress(ip, port)
	}

	ip, err = d.Resolver.LookupIP(host)
	if err != nil {
		return nil, err
	}

	if ip == nil {
		return nil, fmt.Errorf("lookup %s: no such host", host)
	}

	return lib.NewHostAddress(ip, port)
}

func (d *Dialer) getConfiguredHost(addr, host, port string) (*lib.HostAddress, error) {
	if remote, ok := d.Hosts[addr]; ok {
		return remote, nil
	}

	if remote, ok := d.Hosts[host]; ok {
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

	return nil, nil
}

// NetTrail contains information about the exchanged data size and length of a
// series of connections from a particular netext.Dialer
type NetTrail struct {
	BytesRead     int64
	BytesWritten  int64
	FullIteration bool
	StartTime     time.Time
	EndTime       time.Time
	Tags          *metrics.SampleTags
	Samples       []metrics.Sample
}

// Ensure that interfaces are implemented correctly
var _ metrics.ConnectedSampleContainer = &NetTrail{}

// GetSamples implements the metrics.SampleContainer interface.
func (ntr *NetTrail) GetSamples() []metrics.Sample {
	return ntr.Samples
}

// GetTags implements the metrics.ConnectedSampleContainer interface.
func (ntr *NetTrail) GetTags() *metrics.SampleTags {
	return ntr.Tags
}

// GetTime implements the metrics.ConnectedSampleContainer interface.
func (ntr *NetTrail) GetTime() time.Time {
	return ntr.EndTime
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
