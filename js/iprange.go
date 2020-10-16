package js

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"strings"
)

type SelectMode uint

const (
	LoopIncSelectIP SelectMode = iota // increase one by one, no IP is leaking
	RandomSelectIP                    // random
)

type IPBlock struct {
	ip        net.IP
	hostStart uint64
	netStart  uint64
	hostN     uint64
	netN      uint64
	weight    uint64
	split     uint64
	mode      SelectMode
}
type IPPool []IPBlock

// Return a consistant IP block
// support range '-' or CIDR '/' format
// support up to 2^32 IPv4 addresses and 2^64 * 2^64 IPv6 addesses
// for IPv6, high 64 bits is independent to low 64 bits if input > 64 bits
// this is useful if you want to test some IPv6 nets without traveling 2^64 hosts
func GetIPBlock(s string) *IPBlock {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '-':
			return ipBlockFromRange(s)
		case '/':
			return ipBlockFromCIDR(s)
		}
	}
	return ipBlockFromRange(s + "-" + s)
}

func ipUint64(ip net.IP) (uint64, uint64) {
	ip128 := ip.To16()
	net64 := binary.BigEndian.Uint64(ip128[:8])
	host64 := binary.BigEndian.Uint64(ip128[8:])
	return net64, host64
}

func ipBlockFromRange(s string) *IPBlock {
	ss := strings.SplitN(s, "-", 2)
	ip0, ip1 := net.ParseIP(ss[0]), net.ParseIP(ss[1]) // len(ParseIP())==16
	if ip0 == nil || ip1 == nil {
		fmt.Println("Wrong IP range format: ", s)
		return nil
	}
	n0, h0 := ipUint64(ip0)
	n1, h1 := ipUint64(ip1)
	if (n0 > n1) || (h0 > h1) {
		fmt.Println("Negative IP range: ", s)
		return nil
	}
	return &IPBlock{
		ip:        ip0,
		hostStart: h0,
		netStart:  n0,
		hostN:     h1 - h0 + 1,
		netN:      n1 - n0 + 1,
		weight:    1,
		split:     1,
		mode:      LoopIncSelectIP,
	}
}

func ipBlockFromCIDR(s string) *IPBlock {
	ipk, pnet, err := net.ParseCIDR(s) // range start ip, cidr ipnet
	if err != nil {
		fmt.Println("ParseCIDR() failed: ", s)
		return nil
	}
	ip0 := pnet.IP.To16() // cidr base ip
	nk, hk := ipUint64(ipk)
	n0, h0 := ipUint64(ip0)
	if hk < h0 || nk < n0 {
		fmt.Println("Wrong PraseCIDR result: ", s)
		return nil
	}
	ones, bits := pnet.Mask.Size()
	nz, hz := 0, bits-ones
	if hz > 64 {
		nz, hz = hz-64, 64
	}
	// must: 0 <= z <= 64
	offsetCIDR := func(zbits int, offset uint64) uint64 {
		switch zbits {
		case 0:
			return uint64(1)
		case 64:
			n := ^uint64(0) - offset
			if n+1 > n {
				n = n + 1
			}
			return n
		default:
			return (uint64(1) << zbits) - offset
		}
	}
	return &IPBlock{
		ip:        ipk,
		hostStart: hk,
		netStart:  nk,
		hostN:     offsetCIDR(hz, hk-h0),
		netN:      offsetCIDR(nz, nk-n0),
		weight:    1,
		split:     1,
		mode:      LoopIncSelectIP,
	}
}

// GetRandomIP return a random IP by seed from an IP block
func (b IPBlock) GetRandomIP(seed uint64) net.IP {
	r := rand.New(rand.NewSource(int64(seed)))
	if ip4 := b.ip.To4(); ip4 != nil {
		i := b.hostStart + r.Uint64()%b.hostN
		return net.IPv4(byte(i>>24), byte(i>>16), byte(i>>8), byte(i))
	}
	if ip6 := b.ip.To16(); ip6 != nil {
		netN := b.netStart + r.Uint64()%b.netN
		hostN := b.hostStart + r.Uint64()%b.hostN
		if hostN < b.hostStart {
			netN += 1
		}
		if ip := make(net.IP, net.IPv6len); ip != nil {
			binary.BigEndian.PutUint64(ip[:8], netN)
			binary.BigEndian.PutUint64(ip[8:], hostN)
			return ip
		}
	}
	return nil
}

// GetRoundRobinIP return a IP by indexes from an IP block
func (b IPBlock) GetRoundRobinIP(hostIndex, netIndex uint64) net.IP {
	if ip4 := b.ip.To4(); ip4 != nil {
		i := b.hostStart + hostIndex%b.weight
		return net.IPv4(byte(i>>24), byte(i>>16), byte(i>>8), byte(i))
	}
	if ip6 := b.ip.To16(); ip6 != nil {
		netN := b.netStart + netIndex%b.netN
		hostN := b.hostStart + hostIndex%b.weight
		if hostN < b.hostStart {
			netN += 1
		}
		if ip := make(net.IP, net.IPv6len); ip != nil {
			binary.BigEndian.PutUint64(ip[:8], netN)
			binary.BigEndian.PutUint64(ip[8:], hostN)
			return ip
		}
	}
	return nil
}

// Parse range1[:mode[:weight]][,range2[:mode[:weight]]] and return an IPBlock slice
func GetPool(ranges string) IPPool {
	ss := strings.Split(strings.TrimSpace(ranges), ",")
	pool := make([]IPBlock, 0)
	for _, bs := range ss {
		rmw := strings.Split(bs, "|") // range:mode:weight
		sz := len(rmw)
		if sz < 1 {
			continue
		}
		r := GetIPBlock(rmw[0])
		if r == nil {
			continue
		}
		if sz > 1 {
			if mode, err := strconv.Atoi(rmw[1]); err == nil {
				r.mode = SelectMode(mode)
			}
		}
		r.weight = r.hostN
		if sz > 2 {
			if weight, err := strconv.ParseUint(rmw[2], 10, 64); err == nil {
				r.weight = weight
			}
		}
		r.split = r.weight
		if len(pool) > 0 {
			r.split += pool[len(pool)-1].split
		}
		pool = append(pool, *r)
	}
	return pool
}

func (pool IPPool) GetIP(id uint64) net.IP {
	if len(pool) < 1 {
		return nil
	}
	idx := id % pool[len(pool)-1].split
	for _, b := range pool {
		if idx < b.split {
			switch b.mode {
			case RandomSelectIP:
				return b.GetRandomIP(id)
			case LoopIncSelectIP:
				return b.GetRoundRobinIP(id, id)
			default:
				return nil
			}
		}
	}
	return nil
}
