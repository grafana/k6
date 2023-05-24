package mockresolver

import (
	"fmt"
	"net"
	"sync"
)

// MockResolver implements netext.Resolver, and allows changing the host
// mapping at runtime.
type MockResolver struct {
	m           sync.RWMutex
	hosts       map[string][]net.IP
	ResolveHook func(host string, result []net.IP)
}

// New returns a new MockResolver.
func New(hosts map[string][]net.IP) *MockResolver {
	if hosts == nil {
		hosts = make(map[string][]net.IP)
	}
	return &MockResolver{hosts: hosts}
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
func (r *MockResolver) LookupIPAll(host string) (ips []net.IP, err error) {
	defer func() {
		if r.ResolveHook != nil {
			r.ResolveHook(host, ips)
		}
	}()
	r.m.RLock()
	defer r.m.RUnlock()
	if ips, ok := r.hosts[host]; ok {
		return ips, nil
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
