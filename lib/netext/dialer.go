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
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"github.com/viki-org/dnscache"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/stats"
)

// DNSResolver is an interface that fetches DNS information about a given host.
type DNSResolver interface {
	Fetch(host string) (net.IP, error)
}

type resolver struct {
	cache      *dnscache.Resolver
	strategy   lib.DNSStrategy
	m          sync.Mutex
	roundRobin map[string]uint8
}

// NewDNSResolver returns a new DNS resolver. If ttl is a valid duration,
// responses will be cached for the specified period, or indefinitely if it's 0.
// The IP returned from Fetch() will be selected based on the given strategy.
func NewDNSResolver(ttl types.NullDuration, strategy lib.DNSStrategy) DNSResolver {
	var cache *dnscache.Resolver
	if ttl.Valid {
		cache = dnscache.New(time.Duration(ttl.Duration))
	}
	return &resolver{
		cache:      cache,
		strategy:   strategy,
		roundRobin: make(map[string]uint8),
	}
}

// Fetch does a DNS lookup or uses the cache if enabled, and returns a single IP
// selected with the preset strategy.
func (r *resolver) Fetch(host string) (net.IP, error) {
	var (
		ips []net.IP
		err error
	)
	if r.cache != nil {
		ips, err = r.cache.Fetch(host)
	} else {
		ips, err = net.LookupIP(host)
	}
	if err != nil {
		return nil, err
	}

	if len(ips) == 0 {
		return nil, errors.Errorf("DNS lookup for %s returned 0 IPs", host)
	}

	return r.selectOne(host, ips), nil
}

func (r *resolver) selectOne(host string, ips []net.IP) net.IP {
	switch r.strategy {
	case lib.DNSFirst:
		return ips[0]
	case lib.DNSRoundRobin:
		r.m.Lock()
		defer func() {
			r.roundRobin[host]++
			r.m.Unlock()
		}()
		// TODO: The index approach is not stable and we'll need to track
		// individual IPs... Do we really want to implement an additional cache
		// for this, since dnscache does not expose its internal one? If so, we
		// should scrap dnscache entirely and re-implement it here.
		return ips[int(r.roundRobin[host])%len(ips)]
	case lib.DNSRandom:
		return ips[rand.Intn(len(ips))]
	}
	return nil
}

// Dialer wraps net.Dialer and provides k6 specific functionality -
// tracing, blacklists and DNS cache and aliases.
type Dialer struct {
	net.Dialer

	Resolver  DNSResolver
	Blacklist []*lib.IPNet
	Hosts     map[string]*lib.HostAddress

	BytesRead    int64
	BytesWritten int64
}

// NewDialer constructs a new Dialer with the given DNS resolver.
func NewDialer(dialer net.Dialer, resolver DNSResolver) *Dialer {
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
// https://github.com/loadimpact/k6/pull/1203#discussion_r337938370
func (d *Dialer) GetTrail(
	startTime, endTime time.Time, fullIteration bool, emitIterations bool, tags *stats.SampleTags,
) *NetTrail {
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
		if emitIterations {
			samples = append(samples, stats.Sample{
				Time:   endTime,
				Metric: metrics.Iterations,
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

	remote, err := d.getConfiguredHost(addr, host, port)
	if err != nil || remote != nil {
		return remote, err
	}

	ip := net.ParseIP(host)
	if ip != nil {
		return lib.NewHostAddress(ip, port)
	}

	ip, err = d.Resolver.Fetch(host)
	if err != nil {
		return nil, err
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

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}
