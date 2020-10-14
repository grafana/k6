package js

import (
	"fmt"
	"strings"
	"strconv"
	"net"
	"time"
	"math/rand"
	"encoding/binary"
)

type SelectMode uint
const (
        LoopIncSelectIP SelectMode = iota // increase one by one, no IP is leaking
        RandomSelectIP // random
	ModLenSelectIP // like LoopIncSelect, may leak IP(s) if id is not continous
)
type IPBlock struct {
        ip        net.IP
        hostStart uint64
        netStart  uint64
        hostN     uint64
        netN      uint64
        weight    uint64
        mode  SelectMode
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

func ipBlockFromRange(s string) *IPBlock {
	ss := strings.SplitN(s, "-", 2)
	ip0, ip1 := net.ParseIP(ss[0]), net.ParseIP(ss[1]) // len(ParseIP())==16
	if ip0 == nil || ip1 == nil {
		fmt.Println("Wrong IP range format: ", s)
		return nil
	}
	n0 := binary.BigEndian.Uint64(ip0[:8])
	h0 := binary.BigEndian.Uint64(ip0[8:])
	n1 := binary.BigEndian.Uint64(ip1[:8])
	h1 := binary.BigEndian.Uint64(ip1[8:])
	if (n0 > n1) || (h0 > h1) {
		fmt.Println("Negative IP range: ", s)
		return nil
	}
	return &IPBlock{ip0, h0, n0, h1-h0+1, n1-n0+1, 1, LoopIncSelectIP}
}

func ipBlockFromCIDR(s string) *IPBlock {
	ipk, pnet, err := net.ParseCIDR(s)
	if err != nil {
		fmt.Println("ParseCIDR() failed: ", s)
		return nil
	}
	// ipk (ip start in cidr) >= ip0 (cidr-aligned ip base)
	ip0 := pnet.IP.To16()
	nk := binary.BigEndian.Uint64(ipk[:8])
	hk := binary.BigEndian.Uint64(ipk[8:])
	n0 := binary.BigEndian.Uint64(ip0[:8])
	h0 := binary.BigEndian.Uint64(ip0[8:])
	if hk < h0 || nk < n0 {
		fmt.Println("Wrong PraseCIDR result: ", s)
		return nil
	}
	ones, bits := pnet.Mask.Size()
	nz, hz := 0, bits - ones
	if hz > 64 {
		nz, hz = hz - 64, 64
	}
	// must: 0 <= z <= 64
	offsetCIDR := func (zbits int, offset uint64) uint64 {
		switch zbits {
		case 0:
			return uint64(1)
		case 64:
			n := ^uint64(0) - offset
			if n + 1 > n {
				n = n + 1
			}
			return n
		default:
			return (uint64(1) << zbits) - offset
		}
	}
        hostNum := offsetCIDR(hz, hk-h0)
        netNum := offsetCIDR(nz, nk-n0)
        return &IPBlock{ip0, h0, n0, hostNum, netNum, 1, LoopIncSelectIP}
}

// GetRandomIP return a random IP by seed from an IP block
func (b IPBlock) GetRandomIP(seed int64) net.IP {
	r := rand.New(rand.NewSource(seed))
	h := r.Uint64()
	n := r.Uint64()
	return  b.GetModNIndexedIP(h, n)
}

// GetModNIndexedIP return a IP by indexes from an IP block
func (b IPBlock) GetModNIndexedIP(hostIndex, netIndex uint64) net.IP {
	if ip4 := b.ip.To4(); ip4 != nil {
                i := b.hostStart + hostIndex % b.hostN
                return net.IPv4(byte(i>>24), byte(i>>16), byte(i>>8), byte(i))
        }
        if ip6 := b.ip.To16(); ip6 != nil {
                netN := b.netStart + netIndex % b.netN
                hostN := b.hostStart + hostIndex % b.hostN
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
                if sz > 2 {
                        if weight, err := strconv.ParseUint(rmw[2], 10, 64); err == nil {
                                r.weight = weight
                        }
                }
                pool = append(pool, *r)
                for i := 0; i < len(pool) - 1; i++ {
                        pool[i].weight += r.weight
                }
        }
        return pool
}

func (pool IPPool) GetIP(id int64) net.IP {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := pool[0]
	k := r.Uint64() % b.weight
	for i := len(pool) - 1; i > 0; i-- {
		if k < pool[i].weight {
			b = pool[i]
			break
		}
	}
	switch b.mode {
	case RandomSelectIP:
		return b.GetRandomIP(id)
	case LoopIncSelectIP:
		return b.GetModNIndexedIP(uint64(id), uint64(id))
	}
	return nil
}
