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

package mockresolver

import (
	"fmt"
	"net"
	"sync"
)

// MockResolver implements netext.Resolver, and allows changing the host
// mapping at runtime.
type MockResolver struct {
	m        sync.RWMutex
	hosts    map[string][]net.IP
	fallback func(host string) ([]net.IP, error)
}

// New returns a new MockResolver.
func New(hosts map[string][]net.IP, fallback func(host string) ([]net.IP, error)) *MockResolver {
	if hosts == nil {
		hosts = make(map[string][]net.IP)
	}
	return &MockResolver{hosts: hosts, fallback: fallback}
}

// LookupIP returns the first IP mapped for host.
func (r *MockResolver) LookupIP(host string) (net.IP, error) {
	if ips, err := r.LookupIPAll(host); err != nil {
		return nil, err
	} else if len(ips) > 0 {
		return ips[0], nil
	}
	return nil, nil
}

// LookupIPAll returns all IPs mapped for host. It mimics the net.LookupIP
// signature so that it can be used to mock netext.LookupIP in tests.
func (r *MockResolver) LookupIPAll(host string) ([]net.IP, error) {
	r.m.RLock()
	defer r.m.RUnlock()
	if ips, ok := r.hosts[host]; ok {
		return ips, nil
	}
	if r.fallback != nil {
		return r.fallback(host)
	}
	return nil, fmt.Errorf("lookup %s: no such host", host)
}

// Set the host to resolve to ip.
func (r *MockResolver) Set(host, ip string) {
	r.m.Lock()
	defer r.m.Unlock()
	r.hosts[host] = []net.IP{net.ParseIP(ip)}
}

// Unset removes the host.
func (r *MockResolver) Unset(host string) {
	r.m.Lock()
	defer r.m.Unlock()
	delete(r.hosts, host)
}
