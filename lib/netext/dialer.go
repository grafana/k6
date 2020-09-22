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
	"syscall"
	"time"

	"github.com/benburkert/dns"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/stats"
)

// Dialer wraps net.Dialer and provides k6 specific functionality -
// tracing, blacklists and DNS cache and aliases.
type Dialer struct {
	net.Dialer

	Resolver  Resolver
	Blacklist []*lib.IPNet
	Hosts     map[string]*lib.HostAddress

	BytesRead    int64
	BytesWritten int64

	dnsCache DNSCache
	ttl      time.Duration
	selecter selecter
}

// NewDialer constructs a new Dialer with the given DNS resolver.
func NewDialer(
	dialer net.Dialer, blacklist []*lib.IPNet, hosts map[string]*lib.HostAddress,
	dnsConfig lib.DNSConfig,
	// TODO take DNSCache so it's shared between VUs
) *Dialer {
	r := rand.New(rand.NewSource(time.Now().UnixNano())) // nolint: gosec

	ttl, err := parseTTL(dnsConfig.TTL.String)
	if err != nil {
		panic(err) // TODO fix
	}
	strategy := dnsConfig.Strategy.DNSStrategy
	if !strategy.IsADNSStrategy() {
		strategy = lib.DefaultDNSConfig().Strategy.DNSStrategy
	}
	d := &Dialer{
		Blacklist: blacklist,
		Hosts:     hosts,
		selecter: selecter{
			strategy:   strategy,
			rrm:        &sync.Mutex{},
			rand:       r,
			roundRobin: make(map[string]uint8),
		},
		ttl: ttl,
		dnsCache: DNSCache{
			v4: &dnsCache{
				RWMutex: sync.RWMutex{},
				cache:   make(map[string]*cacheRecord),
			},
			v6: &dnsCache{
				RWMutex: sync.RWMutex{},
				cache:   make(map[string]*cacheRecord),
			},
		},
	}
	dialer.Resolver = &net.Resolver{
		PreferGo: true,
		Dial: (&dns.Client{
			Resolver: d,
		}).Dial,
	}
	if len(d.Blacklist) != 0 {
		dialer.Control = func(network, address string, c syscall.RawConn) error {
			ipStr, _, err := net.SplitHostPort(address)
			if err != nil { // this should never happen
				return err
			}
			ip := net.ParseIP(ipStr)

			for _, ipnet := range d.Blacklist {
				if ipnet.Contains(ip) {
					return BlackListedIPError{ip: ip, net: ipnet}
				}
			}
			return nil
		}
	}
	d.Dialer = dialer // TODO fix this possibly by using a pointer or just rewriting this whole configuration
	return d
}

// ServeDNS TODO write
//nolint:funlen,gocognit
func (d *Dialer) ServeDNS(ctx context.Context, mw dns.MessageWriter, q *dns.Query) {
	// TODO log errors when we have a logger
	// TODO rewrite this possibly by updating the library or building our own so that error handling
	// is better
	// TODO we technically could get a question for both ... but this doesn't happen currently with
	// golang's stdlib implementation so ... hopefully this wont' be a problem
	if len(q.Questions) != 1 && !(q.Questions[0].Type == dns.TypeA || q.Questions[0].Type == dns.TypeAAAA) {
		m, _ := mw.Recur(ctx) // this error automatically get's set
		for _, answer := range m.Answers {
			mw.Answer(answer.Name, answer.TTL, answer.Record)
		}
		_ = mw.Reply(ctx) // there is nothing to with that error
		return
	}

	question := q.Questions[0]
	switch question.Type { //nolint: exhaustive
	case dns.TypeA:
		res := d.dnsCache.v4.Get(question.Name)
		ttl := d.ttl
		fmt.Println("A")
		res.Lock()
		if len(res.ips) == 0 || res.validTo.Before(time.Now()) {
			fmt.Println("A miss", time.Until(res.validTo))
			m, err := mw.Recur(ctx) // this error automatically get's set
			if err != nil {
				res.Unlock() // TODO maybe move to defer
				return
			}
			res.ips = make([]net.IP, 0, len(m.Answers))
			for _, answer := range m.Answers {
				if a, ok := answer.Record.(*dns.A); ok {
					if ttl < 0 {
						ttl = answer.TTL
					}
					res.ips = append(res.ips, a.A)
				}
			}
			if ttl > 0 {
				res.validTo = time.Now().Add(ttl)
			}
		}
		ip := d.selecter.selectOne(question.Name, res.ips)
		res.Unlock() // TODO maybe move to a defer
		fmt.Println(ip)
		if ip != nil {
			mw.Answer(question.Name, ttl, &dns.A{A: ip})
		} else {
			mw.Status(dns.NXDomain)
		}
		_ = mw.Reply(ctx)
	case dns.TypeAAAA: // TODO DRY
		fmt.Println("AAAA")
		res := d.dnsCache.v6.Get(question.Name)
		ttl := d.ttl
		res.Lock()
		if len(res.ips) == 0 || res.validTo.Before(time.Now()) {
			fmt.Println("AAAA miss", time.Until(res.validTo))
			m, err := mw.Recur(ctx) // this error automatically get's set
			if err != nil {
				res.Unlock() // TODO maybe move to defer
				return
			}
			res.ips = make([]net.IP, 0, len(m.Answers))
			for _, answer := range m.Answers {
				if a, ok := answer.Record.(*dns.AAAA); ok {
					if ttl < 0 {
						ttl = answer.TTL
					}
					res.ips = append(res.ips, a.AAAA)
				}
			}
			if ttl > 0 {
				res.validTo = time.Now().Add(ttl)
			}
		}
		ip := d.selecter.selectOne(question.Name, res.ips)
		res.Unlock() // TODO maybe move to a defer
		fmt.Println(ip)
		if ip != nil {
			mw.Answer(question.Name, ttl, &dns.AAAA{AAAA: ip})
		} else {
			mw.Status(dns.NXDomain)
		}
		_ = mw.Reply(ctx)
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
	remote, err := d.getConfiguredHost(addr)
	if err != nil {
		return nil, err
	}
	if remote != nil {
		addr = remote.String()
	}
	conn, err := d.Dialer.DialContext(ctx, proto, addr)
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
	remote, err := d.getConfiguredHost(addr)
	if err != nil {
		return "", err
	}
	return remote.String(), nil
}

func (d *Dialer) getConfiguredHost(addr string) (*lib.HostAddress, error) {
	if remote, ok := d.Hosts[addr]; ok {
		return remote, nil
	}

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
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

func parseTTL(ttlS string) (time.Duration, error) {
	ttl := time.Duration(0)
	switch ttlS {
	case "real":
		ttl = -1
	case "inf":
		// cache "indefinitely"
		ttl = time.Hour * 24 * 365
	case "0":
		// disable cache
	case "":
		ttlS = lib.DefaultDNSConfig().TTL.String
		fallthrough
	default:
		origTTLs := ttlS
		// Treat unitless values as milliseconds
		if t, err := strconv.ParseFloat(ttlS, 32); err == nil {
			ttlS = fmt.Sprintf("%.2fms", t)
		}
		var err error
		ttl, err = types.ParseExtendedDuration(ttlS)
		if ttl < 0 || err != nil {
			return ttl, fmt.Errorf("invalid DNS TTL: %s", origTTLs)
		}
	}
	return ttl, nil
}
