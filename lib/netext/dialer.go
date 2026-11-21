package netext

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/net/idna"

	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/lib/types"
	"go.k6.io/k6/v2/metrics"
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
	conn, err := d.Dialer.DialContext(ctx, proto, dialAddr.String())
	if err != nil {
		return nil, err
	}
	conn = &Conn{conn, &d.BytesRead, &d.BytesWritten}
	return conn, err
}

// CheckTargetAddr applies the configured hostname and IP restrictions to a
// target without resolving a hostname. It is used before a request is handed
// to a forward proxy, where DialContext only sees the proxy address.
func (d *Dialer) CheckTargetAddr(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}

	// Scoped IPv6 literals are still IP destinations for the proxy; compare
	// their zone-free address against the blacklist.
	if i := strings.LastIndexByte(host, '%'); i >= 0 {
		host = host[:i]
	}
	// Match the transport's ASCII form for IDNs and Unicode-mapped IPv4.
	if ascii, err := idna.Lookup.ToASCII(host); err == nil && ascii != "" {
		host = ascii
	} else {
		host = strings.ToLower(host)
	}
	host = strings.TrimSuffix(host, ".")
	ip := net.ParseIP(host)
	if ip != nil {
		for _, ipnet := range d.Blacklist {
			if ipnet.Contains(ip) {
				return BlackListedIPError{ip: ip, net: ipnet}
			}
		}
		return nil
	}

	if d.BlockedHostnames != nil {
		if match, blocked := d.BlockedHostnames.Contains(host); blocked {
			return BlockedHostError{hostname: host, match: match}
		}
	}

	if d.Hosts != nil {
		_, port, _ := net.SplitHostPort(addr)
		remote := d.Hosts.Match(net.JoinHostPort(host, port))
		if remote == nil {
			remote = d.Hosts.Match(host)
		}
		if remote != nil && remote.IP != nil {
			for _, ipnet := range d.Blacklist {
				if ipnet.Contains(remote.IP) {
					return BlackListedIPError{ip: remote.IP, net: ipnet}
				}
			}
		}
	}

	return nil
}

// ProxyFromEnvironment returns the proxy for the request after checking the
// effective destination the proxy will receive. DialContext cannot do that
// check when a proxy is used because it receives the proxy address.
func (d *Dialer) ProxyFromEnvironment(req *http.Request) (*url.URL, error) {
	return d.proxyWithHostCheck(req, true, http.ProxyFromEnvironment)
}

func (d *Dialer) proxyFromEnvironmentWithHostCheck(req *http.Request, checkHost bool) (*url.URL, error) {
	return d.proxyWithHostCheck(req, checkHost, http.ProxyFromEnvironment)
}

func (d *Dialer) proxyWithHostCheck(
	req *http.Request,
	checkHost bool,
	proxyFunc func(*http.Request) (*url.URL, error),
) (*url.URL, error) {
	proxyURL, err := proxyFunc(req)
	if err != nil || proxyURL == nil {
		return proxyURL, err
	}

	if err := d.CheckTargetAddr(proxyTargetAddr(req.URL)); err != nil {
		return nil, err
	}

	// For an HTTP/1 forward proxy Go writes the Host override into the absolute
	// request target, so the proxy can receive an authority different from the URL.
	// SOCKS proxies tunnel to the URL destination and only forward Host inside the
	// tunnel, so they do not need this check.
	if checkHost && req.URL.Scheme == "http" && req.Host != "" &&
		proxyURL.Scheme != "socks5" &&
		proxyURL.Scheme != "socks5h" {
		host := req.Host
		if h, port, err := net.SplitHostPort(host); err != nil {
			if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
				host += ":80"
			} else {
				host = net.JoinHostPort(host, "80")
			}
		} else if port == "" {
			host = net.JoinHostPort(h, "80")
		}
		if err := d.CheckTargetAddr(host); err != nil {
			return nil, err
		}
	}

	return proxyURL, nil
}

func proxyTargetAddr(u *url.URL) string {
	port := u.Port()
	if port == "" {
		switch u.Scheme {
		case "https", "wss":
			port = "443"
		default:
			port = "80"
		}
	}
	return net.JoinHostPort(u.Hostname(), port)
}

// ProxyFromEnvironment returns a proxy callback for a DialContexter. Custom
// dialers that only implement DialContext keep the default environment lookup.
func ProxyFromEnvironment(d lib.DialContexter) func(*http.Request) (*url.URL, error) {
	if dialer, ok := d.(*Dialer); ok {
		return dialer.ProxyFromEnvironment
	}
	return http.ProxyFromEnvironment
}

// ProxyFromEnvironmentURLOnly is like ProxyFromEnvironment but checks only the
// URL destination. WebSocket dialers use it because they send CONNECT to the URL
// authority and forward a Host override inside the tunnel.
func ProxyFromEnvironmentURLOnly(d lib.DialContexter) func(*http.Request) (*url.URL, error) {
	if dialer, ok := d.(*Dialer); ok {
		return func(req *http.Request) (*url.URL, error) {
			return dialer.proxyFromEnvironmentWithHostCheck(req, false)
		}
	}
	return http.ProxyFromEnvironment
}

// ResolveAddr looks up the IP address for the given host and optionally port.
// The address is expected in the form "host:port" or just "host".
// It returns the resolved IP, and an error if any.
func (d *Dialer) ResolveAddr(addr string) (net.IP, int, error) {
	// Check if the address has a port, if not add a dummy port for parsing
	if _, _, err := net.SplitHostPort(addr); err != nil {
		// Address doesn't have a port, add a dummy one
		addr = net.JoinHostPort(addr, "0")
	}

	remote, err := d.getDialAddr(addr)
	if err != nil {
		return nil, 0, err
	}

	return remote.IP, remote.Port, nil
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

func (d *Dialer) getDialAddr(addr string) (*types.Host, error) {
	remote, err := d.findRemote(addr)
	if err != nil {
		return nil, err
	}

	for _, ipnet := range d.Blacklist {
		if ipnet.Contains(remote.IP) {
			return nil, BlackListedIPError{ip: remote.IP, net: ipnet}
		}
	}

	return remote, nil
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
