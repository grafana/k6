/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2020 Load Impact
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
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/loadimpact/k6/lib"
)

// TODO: Figure out a non-global way to expose this for mocking in tests that
// doesn't involve changing the Resolver interface...
var LookupIP = net.LookupIP

// Resolver is an interface that returns DNS information about a given host.
type Resolver interface {
	LookupIP(host string) (net.IP, error)
}

type resolver struct {
	strategy   lib.DNSStrategy
	rrm        *sync.Mutex
	rand       *rand.Rand
	roundRobin map[string]uint8
}

type cacheRecord struct {
	ips        []net.IP
	lastLookup time.Time
}

type cacheResolver struct {
	resolver
	ttl   time.Duration
	cm    *sync.Mutex
	cache map[string]cacheRecord
}

// NewResolver returns a new DNS resolver. If ttl is not 0, responses
// will be cached per host for the specified period. The IP returned from
// LookupIP() will be selected based on the given strategy.
func NewResolver(ttl time.Duration, strategy lib.DNSStrategy) Resolver {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	res := resolver{
		strategy:   strategy,
		rrm:        &sync.Mutex{},
		rand:       r,
		roundRobin: make(map[string]uint8),
	}
	if ttl == 0 {
		return &res
	}
	return &cacheResolver{
		resolver: res,
		ttl:      ttl,
		cm:       &sync.Mutex{},
		cache:    make(map[string]cacheRecord),
	}
}

// LookupIP returns a single IP resolved for host, selected by the
// configured strategy.
func (r *resolver) LookupIP(host string) (net.IP, error) {
	ips, err := LookupIP(host)
	if err != nil {
		return nil, err
	}
	return r.selectOne(host, ips), nil
}

// LookupIP returns a single IP resolved for host, selected by the configured
// strategy. Results are cached per host and will be refreshed if the last
// lookup time exceeds the configured TTL (not the TTL returned in the DNS
// record).
func (r *cacheResolver) LookupIP(host string) (net.IP, error) {
	r.cm.Lock()
	defer r.cm.Unlock()

	var ips []net.IP
	// TODO: Invalidate? When?
	if d, ok := r.cache[host]; ok && time.Now().UTC().Before(d.lastLookup.Add(r.ttl)) {
		ips = r.cache[host].ips
	} else {
		var err error
		ips, err = LookupIP(host)
		if err != nil {
			return nil, err
		}
		r.cache[host] = cacheRecord{ips: ips, lastLookup: time.Now().UTC()}
	}

	return r.selectOne(host, ips), nil
}

func (r *resolver) selectOne(host string, ips []net.IP) net.IP {
	switch r.strategy {
	case lib.DNSFirst:
		return ips[0]
	case lib.DNSRoundRobin:
		r.rrm.Lock()
		defer func() {
			r.roundRobin[host]++
			r.rrm.Unlock()
		}()
		// NOTE: This index approach is not stable and might result in returning
		// repeated or skipped IPs if the records change during a test run.
		return ips[int(r.roundRobin[host])%len(ips)]
	case lib.DNSRandom:
		return ips[r.rand.Intn(len(ips))]
	}
	return nil
}
