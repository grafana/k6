/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package netext

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"time"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/stats"

	"github.com/domainr/dnsr"
	"github.com/miekg/dns"
)

// Dialer wraps net.Dialer and provides k6 specific functionality -
// tracing, blacklists and DNS cache and aliases.
type Dialer struct {
	net.Dialer

	Resolver  *dnsr.Resolver
	IP4       map[string]bool // IPv4 last seen
	CNAME     map[string]CanonicalName
	Blacklist []*lib.IPNet
	Hosts     map[string]net.IP

	BytesRead    int64
	BytesWritten int64
}

// CanonicalName is an expiring CNAME value.
type CanonicalName struct {
	Name   string
	TTL    time.Duration
	Expiry time.Time
}

// NewDialer constructs a new Dialer and initializes its cache.
func NewDialer(dialer net.Dialer) *Dialer {
	return &Dialer{
		Dialer:   dialer,
		Resolver: dnsr.NewExpiring(0),
		IP4:      make(map[string]bool),
		CNAME:    make(map[string]CanonicalName),
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

// DialContext wraps the net.Dialer.DialContext and handles the k6 specifics
func (d *Dialer) DialContext(ctx context.Context, proto, addr string) (net.Conn, error) {
	delimiter := strings.LastIndex(addr, ":")
	host := addr[:delimiter]

	// lookup for domain defined in Hosts option before trying to resolve DNS.
	ip, ok := d.Hosts[host]
	if !ok {
		var err error
		ip, err = d.resolve(host, 10)
		if err != nil {
			return nil, err
		}
	}

	for _, ipnet := range d.Blacklist {
		if (*net.IPNet)(ipnet).Contains(ip) {
			return nil, BlackListedIPError{ip: ip, net: ipnet}
		}
	}
	ipStr := ip.String()
	if strings.ContainsRune(ipStr, ':') {
		ipStr = "[" + ipStr + "]"
	}
	conn, err := d.Dialer.DialContext(ctx, proto, ipStr+":"+addr[delimiter+1:])
	if err != nil {
		return nil, err
	}
	conn = &Conn{conn, &d.BytesRead, &d.BytesWritten}
	return conn, err
}

// resolve maps a host string to an IP address.
// Host string may be an IP address string or a domain name.
// Follows CNAME chain up to depth steps.
func (d *Dialer) resolve(host string, depth uint8) (net.IP, error) {
	ip := net.ParseIP(host)
	if ip != nil {
		return ip, nil
	}
	host, err := d.canonicalName(host, depth)
	if err != nil {
		return nil, err
	}
	observed := make(map[string]struct{})
	return d.resolveName(host, host, depth, observed)
}

// resolveName maps a domain name to an IP address.
// Follows CNAME chain up to depth steps.
// Fails on CNAME chain cycle.
func (d *Dialer) resolveName(
	requested string,
	name string,
	depth uint8,
	observed map[string]struct{},
) (net.IP, error) {
	ip, cname, err := d.lookup(name)
	if err != nil {
		// Lookup error
		return nil, err
	}
	if ip != nil {
		// Found IP address
		return ip, nil
	}
	if cname == nil {
		// Found nothing
		return nil, errors.New("unable to resolve host address `" + requested + "`")
	}
	if depth == 0 {
		// Long CNAME chain
		return nil, errors.New("CNAME chain too long for `" + requested + "`")
	}
	if _, ok := observed[cname.Value]; ok {
		// CNAME chain cycle
		return nil, errors.New("cycle in CNAME chain for `" + requested + "`")
	}
	// Found CNAME
	observed[cname.Value] = struct{}{}
	d.CNAME[name] = CanonicalName{
		Name:   cname.Value,
		TTL:    cname.TTL,
		Expiry: cname.Expiry,
	}
	return d.resolveName(requested, cname.Value, depth-1, observed)
}

// canonicalName reports the best current knowledge about a canonical name.
// Follows CNAME chain up to depth steps.
// Purges expired CNAME entries.
// Fails on a cycle.
func (d *Dialer) canonicalName(name string, depth uint8) (string, error) {
	cname := normalName(name)
	observed := make(map[string]struct{})
	observed[cname] = struct{}{}
	now := time.Now()
	for entry, ok := d.CNAME[cname]; ok; entry, ok = d.CNAME[cname] {
		if now.After(entry.Expiry) {
			// Expired entry
			delete(d.CNAME, cname)
			return cname, nil
		}
		if depth == 0 {
			// Long chain
			return "", errors.New("CNAME chain too long for `" + name + "`")
		}
		cname = entry.Name
		if _, ok := observed[cname]; ok {
			// Cycle
			return "", errors.New("cycle in CNAME chain for `" + name + "`")
		}
		observed[cname] = struct{}{}
		depth--
	}
	return cname, nil
}

// lookup performs a single lookup in the most efficient order.
// Prefers IPv4 if last resolution produced it.
// Otherwise prefers IPv6.
func (d *Dialer) lookup(host string) (net.IP, *dnsr.RR, error) {
	if d.IP4[host] {
		return d.lookup46(host)
	}
	return d.lookup64(host)
}

// lookup64 performs a single lookup preferring IPv6.
// Used on first resolution, if last resolution failed,
// or if last resolution produced IPv6.
func (d *Dialer) lookup64(host string) (net.IP, *dnsr.RR, error) {
	ip, cname, err := d.lookup6(host)
	if err != nil {
		return nil, nil, err
	}
	if ip != nil {
		return ip, nil, nil
	}
	if cname != nil {
		return nil, cname, nil
	}
	ip, cname, err = d.lookup4(host)
	if err != nil {
		return nil, nil, err
	}
	if ip != nil {
		d.IP4[host] = true
		return ip, nil, nil
	}
	if cname != nil {
		return nil, cname, nil
	}
	return nil, nil, errors.New("unable to resolve host address `" + host + "`")
}

// lookup46 performs a single lookup preferring IPv4.
// Used if last resolution produced IPv4.
// Prevents hitting network looking for IPv6 for names with only IPv4.
func (d *Dialer) lookup46(host string) (net.IP, *dnsr.RR, error) {
	ip, cname, err := d.lookup4(host)
	if err != nil {
		return nil, nil, err
	}
	if ip != nil {
		return ip, nil, nil
	}
	if cname != nil {
		return nil, cname, nil
	}
	d.IP4[host] = false
	ip, cname, err = d.lookup6(host)
	if err != nil {
		return nil, nil, err
	}
	if ip != nil {
		return ip, nil, nil
	}
	if cname != nil {
		return nil, cname, nil
	}
	return nil, nil, errors.New("unable to resolve host address `" + host + "`")
}

// lookup6 performs a single lookup for IPv6.
func (d *Dialer) lookup6(host string) (net.IP, *dnsr.RR, error) {
	rrs, err := d.Resolver.ResolveErr(host, "AAAA")
	if err != nil {
		return nil, nil, err
	}
	if len(rrs) > 0 {
		ip, cname := findIP6(rrs)
		if ip != nil {
			return ip, nil, nil
		}
		if cname != nil {
			return nil, cname, nil
		}
	}
	return nil, nil, nil
}

// lookup4 performs a single lookup for IPv4.
func (d *Dialer) lookup4(host string) (net.IP, *dnsr.RR, error) {
	rrs, err := d.Resolver.ResolveErr(host, "A")
	if err != nil {
		return nil, nil, err
	}
	if len(rrs) > 0 {
		ip, cname := findIP4(rrs)
		if ip != nil {
			return ip, nil, nil
		}
		if cname != nil {
			return nil, cname, nil
		}
	}
	return nil, nil, nil
}

// findIP6 returns the first IPv6 address found in rrs.
// Alternately returns a CNAME record if found.
func findIP6(rrs []dnsr.RR) (net.IP, *dnsr.RR) {
	var cname *dnsr.RR = nil
	for _, rr := range rrs {
		ip := extractIP6(&rr)
		if ip != nil {
			return ip, nil
		}
		if rr.Type == "CNAME" {
			cname = &rr
		}
	}
	return nil, cname
}

// findIP4 returns the first IPv4 address found in rrs.
// Alternately returns a CNAME record if found.
func findIP4(rrs []dnsr.RR) (net.IP, *dnsr.RR) {
	var cname *dnsr.RR = nil
	for _, rr := range rrs {
		ip := extractIP4(&rr)
		if ip != nil {
			return ip, nil
		}
		if rr.Type == "CNAME" {
			cname = &rr
		}
	}
	return nil, cname
}

// extractIP6 extracts an IPv6 address from rr.
// Returns nil if record is not type AAAA or address parsing fails.
func extractIP6(rr *dnsr.RR) net.IP {
	if rr.Type == "AAAA" {
		ip := net.ParseIP(rr.Value)
		if ip != nil {
			return ip
		}
	}
	return nil
}

// extractIP4 extracts an IPv4 address from rr.
// Returns nil if record is not type A or address parsing fails.
func extractIP4(rr *dnsr.RR) net.IP {
	if rr.Type == "A" {
		ip := net.ParseIP(rr.Value)
		if ip != nil {
			return ip
		}
	}
	return nil
}

// normalName normalizes a domain name.
func normalName(name string) string {
	return dns.Fqdn(strings.ToLower(name))
}

// GetTrail creates a new NetTrail instance with the Dialer
// sent and received data metrics and the supplied times and tags.
func (d *Dialer) GetTrail(startTime, endTime time.Time, fullIteration bool, tags *stats.SampleTags) *NetTrail {
	bytesWritten := atomic.SwapInt64(&d.BytesWritten, 0)
	bytesRead := atomic.SwapInt64(&d.BytesRead, 0)
	samples := []stats.Sample{
		{
			Time:   endTime,
			Metric: metrics.DataSent,
			Value:  float64(bytesWritten),
			Tags:   tags,
		},
		{
			Time:   endTime,
			Metric: metrics.DataReceived,
			Value:  float64(bytesRead),
			Tags:   tags,
		},
	}
	if fullIteration {
		samples = append(samples, stats.Sample{
			Time:   endTime,
			Metric: metrics.IterationDuration,
			Value:  stats.D(endTime.Sub(startTime)),
			Tags:   tags,
		})
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

// NetTrail contains information about the exchanged data size and length of a
// series of connections from a particular netext.Dialer
type NetTrail struct {
	BytesRead     int64
	BytesWritten  int64
	FullIteration bool
	StartTime     time.Time
	EndTime       time.Time
	Tags          *stats.SampleTags
	Samples       []stats.Sample
}

// Ensure that interfaces are implemented correctly
var _ stats.ConnectedSampleContainer = &NetTrail{}

// GetSamples implements the stats.SampleContainer interface.
func (ntr *NetTrail) GetSamples() []stats.Sample {
	return ntr.Samples
}

// GetTags implements the stats.ConnectedSampleContainer interface.
func (ntr *NetTrail) GetTags() *stats.SampleTags {
	return ntr.Tags
}

// GetTime implements the stats.ConnectedSampleContainer interface.
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
