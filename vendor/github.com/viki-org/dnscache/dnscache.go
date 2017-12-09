package dnscache
// Package dnscache caches DNS lookups

import (
  "net"
  "sync"
  "time"
)

type Resolver struct {
  lock sync.RWMutex
  cache map[string][]net.IP
}

func New(refreshRate time.Duration) *Resolver {
  resolver := &Resolver {
    cache: make(map[string][]net.IP, 64),
  }
  if refreshRate > 0 {
    go resolver.autoRefresh(refreshRate)
  }
  return resolver
}

func (r *Resolver) Fetch(address string) ([]net.IP, error) {
  r.lock.RLock()
  ips, exists := r.cache[address]
  r.lock.RUnlock()
  if exists { return ips, nil }

  return r.Lookup(address)
}

func (r *Resolver) FetchOne(address string) (net.IP, error) {
  ips, err := r.Fetch(address)
  if err != nil || len(ips) == 0 { return nil, err}
  return ips[0], nil
}

func (r *Resolver) FetchOneString(address string) (string, error) {
  ip, err := r.FetchOne(address)
  if err != nil || ip == nil { return "", err }
  return ip.String(), nil
}

func (r *Resolver) Refresh() {
  i := 0
  r.lock.RLock()
  addresses := make([]string, len(r.cache))
  for key, _ := range r.cache {
    addresses[i] = key
    i++
  }
  r.lock.RUnlock()

  for _, address := range addresses {
    r.Lookup(address)
    time.Sleep(time.Second * 2)
  }
}

func (r *Resolver) Lookup(address string) ([]net.IP, error) {
  ips, err := net.LookupIP(address)
  if err != nil { return nil, err }

  r.lock.Lock()
  r.cache[address] = ips
  r.lock.Unlock()
  return ips, nil
}

func (r *Resolver) autoRefresh(rate time.Duration) {
  for {
    time.Sleep(rate)
    r.Refresh()
  }
}
