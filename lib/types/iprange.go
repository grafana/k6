package types

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"net"
	"strconv"
	"strings"
)

// selectMode tells how IP is selected in a IPBlock splited from --ip option
type selectMode uint

const (
	roundRobin selectMode = iota
	random
)

// IPNum split net.IP 16 bytes into two uint64 numbers
// IPv6 128bit split to 64bit network + 64bit host
// IPv4 32bit is included in host 64bit
type IPNum struct{ host, net uint64 }

// IPBlock represents a continuous segment of IP addresses
// ip      - the first IP in this address segment. CIDR can skip lower IPs
// ipStart - uint64 conversion of fistIP[8:16] and fistIP[0:8]
// ipCount - number of 64bit hosts (32bit if IPv4) in this address block
//           number of 64bit nets (always 1 if IPv4) in this address block
// weight  - how many IPs will be chosen from this segment. default is all
//           over-number also allowed to get more weight
type IPBlock struct {
	ip      net.IP
	ipStart IPNum
	ipCount IPNum
	weight  uint64
}

// IPPool represent a slice of IPBlocks
// mode - method to choose IP, sequential(0) or random(1). default is 0
type IPPool struct {
	Blocks []IPBlock
	mode   selectMode
}

// GetIPBlock return a struct of a sequential IP range
// support single IP, range format by '-' or CIDR by '/'
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

func hashIDToUint64(id uint64) uint64 {
	a, b := fnv.New64(), make([]byte, 8)
	binary.LittleEndian.PutUint64(b, id)
	if _, err := a.Write(b); err != nil {
		return 0
	}
	return a.Sum64()
}

func ipUint64(ip net.IP) (uint64, uint64) {
	ip128 := ip.To16()
	net64 := binary.BigEndian.Uint64(ip128[:8])
	host64 := binary.BigEndian.Uint64(ip128[8:])
	return net64, host64
}

func ipBlockFromRange(s string) *IPBlock {
	ss := strings.SplitN(s, "-", 2)
	ip0Str, ip1Str := strings.TrimSpace(ss[0]), strings.TrimSpace(ss[1])
	ip0, ip1 := net.ParseIP(ip0Str), net.ParseIP(ip1Str) // len(ParseIP())==16
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
	hostNum, netNum := h1-h0+1, n1-n0+1
	return &IPBlock{
		ip:      ip0,
		ipStart: IPNum{host: h0, net: n0},
		ipCount: IPNum{host: hostNum, net: netNum},
		weight:  hostNum,
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
	// must: 0 <= zbits <= 64
	offsetCIDR := func(zbits int, offset uint64) uint64 {
		switch zbits {
		case 0:
			return uint64(1)
		case 64:
			n := ^uint64(0) - offset
			if n+1 > n {
				n++
			}
			return n
		default:
			return (uint64(1) << zbits) - offset
		}
	}
	hostNum, netNum := offsetCIDR(hz, hk-h0), offsetCIDR(nz, nk-n0)
	// remove head IP like x.x.x.0 from CIDR
	if hk == h0 && hz > 0 {
		hk++
		hostNum--
	}
	// remove tail IP like x.x.x.255 from CIDR
	if hz > 1 {
		hostNum--
	}
	return &IPBlock{
		ip:      ipk,
		ipStart: IPNum{host: hk, net: nk},
		ipCount: IPNum{host: hostNum, net: netNum},
		weight:  hostNum,
	}
}

// GetRandomIP return a random IP by ID from an IP block
func (b IPBlock) GetRandomIP(id uint64) net.IP {
	r := hashIDToUint64(id % b.weight)
	if ip4 := b.ip.To4(); ip4 != nil {
		i := b.ipStart.host + r%b.ipCount.host
		return net.IPv4(byte(i>>24), byte(i>>16), byte(i>>8), byte(i))
	}
	if ip6 := b.ip.To16(); ip6 != nil {
		netN := b.ipStart.net + r%b.ipCount.net
		hostN := b.ipStart.host + r%b.ipCount.host
		if hostN < b.ipStart.host {
			netN++
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
		i := b.ipStart.host + hostIndex%b.weight
		return net.IPv4(byte(i>>24), byte(i>>16), byte(i>>8), byte(i))
	}
	if ip6 := b.ip.To16(); ip6 != nil {
		netN := b.ipStart.net + netIndex%b.ipCount.net
		hostN := b.ipStart.host + hostIndex%b.weight
		if hostN < b.ipStart.host {
			netN++
		}
		if ip := make(net.IP, net.IPv6len); ip != nil {
			binary.BigEndian.PutUint64(ip[:8], netN)
			binary.BigEndian.PutUint64(ip[8:], hostN)
			return ip
		}
	}
	return nil
}

// GetPool return an IPBlock slice with corret weight
// ranges - possible format is range1[:mode[:weight]][,range2[:mode[:weight]]]
// mode   - method to choose IP, sequential(0) or random(1). default is 0
func GetPool(ranges string, mode int) IPPool {
	ss := strings.Split(strings.TrimSpace(ranges), ",")
	pool := IPPool{Blocks: make([]IPBlock, 0), mode: selectMode(mode)}
	for _, bs := range ss {
		// range | weight
		rmw := strings.Split(strings.TrimSpace(bs), "|")
		sz := len(rmw)
		if sz < 1 {
			continue
		}
		rangeStr := strings.TrimSpace(rmw[0])
		b := GetIPBlock(rangeStr)
		if b == nil {
			continue
		}
		b.weight = b.ipCount.host
		if sz > 1 {
			weightStr := strings.TrimSpace(rmw[1])
			weight, err := strconv.ParseUint(weightStr, 10, 64)
			if err == nil {
				b.weight = weight
			}
		}
		pool.Blocks = append(pool.Blocks, *b)
	}
	return pool
}

// GetIP return an IP from a pool of IPBlock slice
func (pool IPPool) GetIP(id uint64) net.IP {
	nblocks := len(pool.Blocks)
	if nblocks < 1 {
		return nil
	}
	weight, total := uint64(0), uint64(0)
	for i := range pool.Blocks {
		total += pool.Blocks[i].weight
	}
	idx := id % total
	for _, b := range pool.Blocks {
		weight += b.weight
		if idx < weight {
			switch pool.mode {
			case random:
				return b.GetRandomIP(id)
			case roundRobin:
				return b.GetRoundRobinIP(id, id)
			default:
				return nil
			}
		}
	}
	return nil
}
