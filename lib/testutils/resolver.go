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

package testutils

import (
	"net"
	"sync"
)

// MockResolver implements netext.DNSResolver, and allows changing the host
// mapping at runtime.
type MockResolver struct {
	m     *sync.Mutex
	hosts map[string]net.IP
}

func NewMockResolver(hosts map[string]net.IP) *MockResolver {
	if hosts == nil {
		hosts = make(map[string]net.IP)
	}
	return &MockResolver{&sync.Mutex{}, hosts}
}

func (r *MockResolver) FetchOne(host string) (net.IP, error) {
	r.m.Lock()
	defer r.m.Unlock()
	if ip, ok := r.hosts[host]; ok {
		return ip, nil
	}
	return nil, nil
}

func (r *MockResolver) Set(host, ip string) {
	r.m.Lock()
	defer r.m.Unlock()
	r.hosts[host] = net.ParseIP(ip)
}
