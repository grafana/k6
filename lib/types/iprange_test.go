package types

import (
	"bytes"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
	for name, d := range testdata {
		data := d
		t.Run(name, func(t *testing.T) {
			b := ipBlockFromRange(data.ipBlock)
			assert.NotNil(t, b)
			assert.Equal(t, data.hostCount, b.ipCount.host)
			assert.Equal(t, data.netCount, b.ipCount.net)
		})
	}
}

func TestIpBlockFromCIDR(t *testing.T) {
	testdata := map[string]struct {
		hostCount uint64
		netCount  uint64
		ipBlock   string
	}{
		"ipv4 cidr 1": {65534, 1, "192.168.0.0/16"},
		"ipv4 cidr 2": {65534, 1, "192.168.0.1/16"},
		"ipv4 cidr 3": {65525, 1, "192.168.0.10/16"},
		"ipv6 cidr 1": {254, 1, "fd00::0/120"},
		"ipv6 cidr 2": {254, 1, "fd00::1/120"},
		"ipv6 cidr 3": {252, 1, "fd00::3/120"},
		"ipv6 cidr 4": {65534, 1, "fd00::0/112"},
		"ipv6 cidr 5": {65534, 1, "fd00::1/112"},
		"ipv6 cidr 6": {65533, 1, "fd00::2/112"},
	}
	for name, d := range testdata {
		data := d
		t.Run(name, func(t *testing.T) {
			b := ipBlockFromCIDR(data.ipBlock)
			assert.NotNil(t, b)
			assert.Equal(t, data.hostCount, b.ipCount.host)
			assert.Equal(t, data.netCount, b.ipCount.net)
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
		"ipv4 cidr 1":  {254, 1, "192.168.0.0/24"},
		"ipv4 cidr 2":  {254, 1, "192.168.0.1/24"},
		"ipv4 cidr 3":  {245, 1, "192.168.0.10/24"},
		"ipv6 range 1": {1024, 256, "fd00:1:1:0::0-fd00:1:1:ff::3ff"},
		"ipv6 range 2": {1023, 254, "fd00:1:1:2::1-fd00:1:1:ff::3ff"},
		"ipv6 cidr 1":  {65534, 1, "fd00::0/112"},
		"ipv6 cidr 2":  {65534, 1, "fd00::1/112"},
		"ipv6 cidr 3":  {65533, 1, "fd00::2/112"},
	}
	for name, d := range testdata {
		data := d
		t.Run(name, func(t *testing.T) {
			b := GetIPBlock(data.ipBlock)
			assert.NotNil(t, b)
			assert.Equal(t, data.hostCount, b.ipCount.host)
			assert.Equal(t, data.netCount, b.ipCount.net)
		})
	}
}

func TestGetRandomIP(t *testing.T) {
	testdata := map[string]struct {
		hostCount uint64
		netCount  uint64
		ipBlock   string
		ipStart   string
		ipEnd     string
	}{
		"ipv4 cidr 1":  {65534, 1, "192.168.0.0/16", "192.168.0.1", "192.168.255.254"},
		"ipv4 cidr 2":  {65525, 1, "192.168.0.10/16", "192.168.0.10", "192.168.255.254"},
		"ipv4 range 1": {100, 1, "192.168.0.101-192.168.0.200", "192.168.0.101", "192.168.0.200"},
		"ipv4 range 2": {101, 1, "192.168.0.100-192.168.0.200", "192.168.0.100", "192.168.0.200"},
		"ipv6 cidr 1":  {65534, 1, "fd00::0/112", "fd00::1", "fd00::fffe"},
		"ipv6 cidr 2":  {65534, 1, "fd00::1/112", "fd00::1", "fd00::fffe"},
		"ipv6 cidr 3":  {65532, 1, "fd00::3/112", "fd00::3", "fd00::fffe"},
		"ipv6 range 1": {1024, 256, "fd00:1:1:0::0-fd00:1:1:ff::3ff", "fd00:1:1:0::0", "fd00:1:1:ff::3ff"},
		"ipv6 range 2": {1023, 254, "fd00:1:1:2::1-fd00:1:1:ff::3ff", "fd00:1:1:2::1", "fd00:1:1:ff::3ff"},
	}
	for name, d := range testdata {
		data := d
		t.Run(name, func(t *testing.T) {
			b := GetIPBlock(data.ipBlock)
			assert.NotNil(t, b)
			assert.Equal(t, data.hostCount, b.ipCount.host)
			assert.Equal(t, data.netCount, b.ipCount.net)
			for i := 0; i < 300; i++ {
				hv := hashIDToUint64(uint64(time.Now().UnixNano()))
				ip := b.GetRandomIP(hv % 1048576)
				assert.NotNil(t, ip)
				assert.True(t, ipInRange(ip, data.ipStart, data.ipEnd))
			}
		})
	}
}

func TestGetRoundRobinIP(t *testing.T) {
	var i uint64
	testdata := map[string]struct {
		hostCount uint64
		netCount  uint64
		ipBlock   string
		ipStart   string
		ipEnd     string
	}{
		"ipv4 cidr 1":  {65534, 1, "192.168.0.0/16", "192.168.0.1", "192.168.255.254"},
		"ipv4 cidr 2":  {65534, 1, "192.168.0.1/16", "192.168.0.1", "192.168.255.254"},
		"ipv4 cidr 3":  {65525, 1, "192.168.0.10/16", "192.168.0.10", "192.168.255.254"},
		"ipv4 range 1": {100, 1, "192.168.0.101-192.168.0.200", "192.168.0.101", "192.168.0.200"},
		"ipv4 range 2": {101, 1, "192.168.0.100-192.168.0.200", "192.168.0.100", "192.168.0.200"},
		"ipv6 cidr 1":  {65534, 1, "fd00::0/112", "fd00::1", "fd00::fffe"},
		"ipv6 cidr 2":  {65534, 1, "fd00::1/112", "fd00::1", "fd00::fffe"},
		"ipv6 cidr 3":  {65532, 1, "fd00::3/112", "fd00::3", "fd00::fffe"},
		"ipv6 range 1": {1024, 256, "fd00:1:1:0::0-fd00:1:1:ff::3ff", "fd00:1:1:0::0", "fd00:1:1:ff::3ff"},
		"ipv6 range 2": {1023, 254, "fd00:1:1:2::1-fd00:1:1:ff::3ff", "fd00:1:1:2::1", "fd00:1:1:ff::3ff"},
	}
	for name, d := range testdata {
		data := d
		t.Run(name, func(t *testing.T) {
			b := GetIPBlock(data.ipBlock)
			assert.NotNil(t, b)
			assert.Equal(t, data.hostCount, b.ipCount.host)
			assert.Equal(t, data.netCount, b.ipCount.net)
			for i = 1; i < 300; i++ {
				ip := b.GetRoundRobinIP(i, i)
				assert.NotNil(t, ip)
				n, h := ipUint64(ip)
				assert.Equal(t, i%data.hostCount, h-b.ipStart.host)
				assert.Equal(t, i%data.netCount, n-b.ipStart.net)
				assert.True(t, ipInRange(ip, data.ipStart, data.ipEnd))
			}
		})
	}
}

func TestGetPoolNoRandomNoWeight(t *testing.T) {
	testdata := map[string]struct {
		ipBlock string
		weight  uint64
	}{
		"IPv4 Single IPs":       {"1.1.1.1, 1.1.2.1, 1.1.3.1, 1.1.4.1", 1},
		"IPv4 CIDRs":            {"1.1.1.0/24, 2.2.2.0/24, 3.3.0.1/24", 254},
		"IPv4 Ranges":           {"1.1.1.11-1.1.1.20, 2.2.2.5-2.2.2.14", 10},
		"IPv4 Ranges and CIDRs": {"1.1.1.1-1.1.1.254, 2.2.2.0/24", 254},
		"IPv4 IPs and Ranges":   {"1.1.1.1, 1.1.1.2, 3.3.3.3/32", 1},
		"ipv6 Single IPs":       {"fd00:1:1:2::1, fd00:1:1:ff::2, fd00:1:1:ff::3", 1},
		"ipv6 CIDRs":            {"fd00::/120, fd00::1/120, fd00::0.0.0.1/120", 254},
		"ipv6 Ranges":           {"fd00::1 - fd00::a, fd00::5 - fd00::e", 10},
		"IPv6 Ranges and CIDRs": {"fd00::1-fd00::fe, fd00::1/120", 254},
		"IPv6 IPs and Ranges":   {"fd00::1, fd00::2, fd00::3-fd00::3", 1},
		"Mix IPv4 and IPv6":     {"fd00::/120, 192.168.0.0/24", 254},
	}
	for name, d := range testdata {
		data := d
		t.Run(name, func(t *testing.T) {
			p := GetPool(data.ipBlock, 0)
			assert.NotNil(t, p)
			for _, b := range p.Blocks {
				assert.Equal(t, data.weight, b.weight)
			}
			hv := hashIDToUint64(uint64(time.Now().UnixNano()))
			ip := p.GetIP(hv % 1048576)
			assert.NotNil(t, ip)
		})
	}
}

func TestGetPoolWithWeightAndRandom(t *testing.T) {
	testdata := map[string]struct {
		ipBlock string
		weight  uint64
		mode    selectMode
	}{
		"mode defaut sequential 1":              {"1.1.1.1, 2.2.2.2/32", 1, roundRobin},
		"mode defaut sequential 2":              {"1.1.1.0/24, 2.2.2.1/24, 3.3.3.0/24", 254, roundRobin},
		"mode change to random":                 {"1.1.1.0/24|254, 2.2.2.1-2.2.2.254|254, 3.3.3.0/24", 254, random},
		"weight default IPBlock IP number":      {"2.2.2.0/24, 3.3.3.1-3.3.3.254/24", 254, roundRobin},
		"weight more than IPBlock IP number":    {"2.2.2.2|8, 3.3.3.0/30|8", 8, roundRobin},
		"weight less than IPBlock IP number":    {"1.1.1.0/24|20", 20, roundRobin},
		"random weight lesstaking from IPBlock": {"1.1.1.0/24|100", 100, random},
		"random weight overtaking from IPBlock": {"2.2.2.0/28|80", 80, random},
		"ipv6 weight random host only":          {"fd00::/120, fd01::101-fd00::1fe, fd02::201/120|254", 254, random},
		"random weight ipv4 ipv6 mixed":         {"fd00:1:1:2::1/120|100, 192.168.0.0/16|100", 100, random},
	}
	for name, d := range testdata {
		data := d
		t.Run(name, func(t *testing.T) {
			p := GetPool(data.ipBlock, int(data.mode))
			assert.NotNil(t, p)
			assert.Equal(t, data.mode, p.mode)
			for _, b := range p.Blocks {
				assert.Equal(t, data.weight, b.weight)
			}
			hv := hashIDToUint64(uint64(time.Now().UnixNano()))
			ip := p.GetIP(hv % 1048576)
			assert.NotNil(t, ip)
		})
	}
}
