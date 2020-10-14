package js

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"net"
	"testing"
	"time"
)

func ipInRange(ip net.IP, ipStart string, ipEnd string) bool {
	start, end := net.ParseIP(ipStart), net.ParseIP(ipEnd)
	if start == nil || end == nil {
		return false
	}
	if c := ip.To4(); c != nil {
		a, b := start.To4(), end.To4()
		return bytes.Compare(c, a) >= 0 && bytes.Compare(c, b) <= 0
	}
	if d := ip.To16(); d != nil {
		a, b := start.To16(), end.To16()
		return bytes.Compare(d[:8], a[:8]) >= 0 &&
			bytes.Compare(d[:8], b[:8]) <= 0 &&
			bytes.Compare(d[8:], a[8:]) >= 0 &&
			bytes.Compare(d[8:], b[8:]) <= 0
	}
	return false
}

func TestIpBlockFromRange(t *testing.T) {
	testdata := map[string]struct {
		hostCount uint64
		netCount  uint64
		ipBlock   string
	}{
		"ipv4 range 1": {100, 1, "192.168.0.101-192.168.0.200"},
		"ipv4 range 2": {101, 1, "192.168.0.100-192.168.0.200"},
		"ipv6 range 1": {1024, 256, "fd00:1:1:0::0-fd00:1:1:ff::3ff"},
		"ipv6 range 2": {1023, 254, "fd00:1:1:2::1-fd00:1:1:ff::3ff"},
	}
	for name, data := range testdata {
		data := data
		t.Run(name, func(t *testing.T) {
			b := ipBlockFromRange(data.ipBlock)
			assert.NotNil(t, b)
			assert.Equal(t, data.hostCount, b.hostN)
			assert.Equal(t, data.netCount, b.netN)
		})
	}
}

func TestIpBlockFromCIDR(t *testing.T) {
	testdata := map[string]struct {
		hostCount uint64
		netCount  uint64
		ipBlock   string
	}{
		"ipv4 cidr 1": {65536, 1, "192.168.0.0/16"},
		"ipv4 cidr 2": {65526, 1, "192.168.0.10/16"},
		"ipv6 cidr 1": {65536, 1, "fd00::0/112"},
		"ipv6 cidr 2": {65535, 1, "fd00::1/112"},
	}
	for name, data := range testdata {
		data := data
		t.Run(name, func(t *testing.T) {
			b := ipBlockFromCIDR(data.ipBlock)
			assert.NotNil(t, b)
			assert.Equal(t, data.hostCount, b.hostN)
			assert.Equal(t, data.netCount, b.netN)
		})
	}
}

func TestGetIPBlock(t *testing.T) {
	testdata := map[string]struct {
		hostCount uint64
		netCount  uint64
		ipBlock   string
	}{
		"ipv4 range 1": {100, 1, "192.168.0.101-192.168.0.200"},
		"ipv4 range 2": {101, 1, "192.168.0.100-192.168.0.200"},
		"ipv4 cidr 1":  {65536, 1, "192.168.0.0/16"},
		"ipv4 cidr 2":  {65526, 1, "192.168.0.10/16"},
		"ipv6 range 1": {1024, 256, "fd00:1:1:0::0-fd00:1:1:ff::3ff"},
		"ipv6 range 2": {1023, 254, "fd00:1:1:2::1-fd00:1:1:ff::3ff"},
		"ipv6 cidr 1":  {65536, 1, "fd00::0/112"},
		"ipv6 cidr 2":  {65535, 1, "fd00::1/112"},
	}
	for name, data := range testdata {
		data := data
		t.Run(name, func(t *testing.T) {
			b := GetIPBlock(data.ipBlock)
			assert.NotNil(t, b)
			assert.Equal(t, data.hostCount, b.hostN)
			assert.Equal(t, data.netCount, b.netN)
		})
	}
}

func TestGetRandomIP(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	testdata := map[string]struct {
		hostCount uint64
		netCount  uint64
		ipBlock   string
		ipStart   string
		ipEnd     string
	}{
		"ipv4 cidr 1":  {65536, 1, "192.168.0.0/16", "192.168.0.0", "192.168.255.255"},
		"ipv4 cidr 2":  {65526, 1, "192.168.0.10/16", "192.168.10.0", "192.168.255.255"},
		"ipv4 range 1": {100, 1, "192.168.0.101-192.168.0.200", "192.168.0.101", "192.168.0.200"},
		"ipv4 range 2": {101, 1, "192.168.0.100-192.168.0.200", "192.168.0.100", "192.168.0.200"},
		"ipv6 cidr 1":  {65536, 1, "fd00::0/112", "fd00::0", "fd00::ffff"},
		"ipv6 cidr 2":  {65535, 1, "fd00::1/112", "fd00::1", "fd00::ffff"},
		"ipv6 range 1": {1024, 256, "fd00:1:1:0::0-fd00:1:1:ff::3ff", "fd00:1:1:0::0", "fd00:1:1:ff::3ff"},
		"ipv6 range 2": {1023, 254, "fd00:1:1:2::1-fd00:1:1:ff::3ff", "fd00:1:1:2::1", "fd00:1:1:ff::3ff"},
	}
	for name, data := range testdata {
		data := data
		t.Run(name, func(t *testing.T) {
			b := GetIPBlock(data.ipBlock)
			assert.NotNil(t, b)
			assert.Equal(t, data.hostCount, b.hostN)
			assert.Equal(t, data.netCount, b.netN)
			ip := b.GetRandomIP(int64(r.Uint64()) % 1048576)
			assert.NotNil(t, ip)
			assert.True(t, ipInRange(ip, data.ipStart, data.ipEnd))
		})
	}
}

func TestGetModNIndexedIP(t *testing.T) {
	testdata := map[string]struct {
		hostCount uint64
		netCount  uint64
		ipBlock   string
		ipStart   string
		ipEnd     string
	}{
		"ipv4 cidr 1":  {65536, 1, "192.168.0.0/16", "192.168.0.0", "192.168.255.255"},
		"ipv4 cidr 2":  {65526, 1, "192.168.0.10/16", "192.168.10.0", "192.168.255.255"},
		"ipv4 range 1": {100, 1, "192.168.0.101-192.168.0.200", "192.168.0.101", "192.168.0.200"},
		"ipv4 range 2": {101, 1, "192.168.0.100-192.168.0.200", "192.168.0.100", "192.168.0.200"},
		"ipv6 cidr 1":  {65536, 1, "fd00::0/112", "fd00::0", "fd00::ffff"},
		"ipv6 cidr 2":  {65535, 1, "fd00::1/112", "fd00::1", "fd00::ffff"},
		"ipv6 range 1": {1024, 256, "fd00:1:1:0::0-fd00:1:1:ff::3ff", "fd00:1:1:0::0", "fd00:1:1:ff::3ff"},
		"ipv6 range 2": {1023, 254, "fd00:1:1:2::1-fd00:1:1:ff::3ff", "fd00:1:1:2::1", "fd00:1:1:ff::3ff"},
	}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for name, data := range testdata {
		data := data
		t.Run(name, func(t *testing.T) {
			b := GetIPBlock(data.ipBlock)
			assert.NotNil(t, b)
			assert.Equal(t, data.hostCount, b.hostN)
			assert.Equal(t, data.netCount, b.netN)
			idx := r.Uint64() % 1048576
			ip := b.GetModNIndexedIP(idx, idx)
			assert.NotNil(t, ip)
			assert.True(t, ipInRange(ip, data.ipStart, data.ipEnd))
		})
	}
}

func TestGetPool(t *testing.T) {
	testdata := map[string]struct {
		ipBlock string
		weight  uint64
		mode    SelectMode
	}{
		"mode 0 weight 1":  {"192.168.0.1,1.1.1.1|0,2.2.2.100-2.2.2.200|0|1", 1, LoopIncSelectIP},
		"mode 1 weight 50": {"1.1.0.0/16|1|50,2.2.2.100-2.2.2.120|1|50", 50, RandomSelectIP},
		"ipv4 list":        {"192.168.0.1,192.168.0.2,192.168.0.3", 1, LoopIncSelectIP},
		"ipv6 list":        {"fd00:1:1:2::1,fd00:1:1:ff::2,fd00:1:1:ff::3", 1, LoopIncSelectIP},
		"ipv4 ipv6 mixed":  {"fd00:1:1:2::1/120|1,192.168.0.0/16|1", 1, RandomSelectIP},
	}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for name, data := range testdata {
		data := data
		t.Run(name, func(t *testing.T) {
			p := GetPool(data.ipBlock)
			assert.NotNil(t, p)
			for _, b := range p {
				assert.Equal(t, data.mode, b.mode)
			}
			for i, j := len(p)-1, uint64(0); i >= 0; i-- {
				j += data.weight
				assert.Equal(t, j, p[i].weight)
			}
			ip := p.GetIP(int64(r.Uint64() % 1048576))
			assert.NotNil(t, ip)
		})
	}
}
