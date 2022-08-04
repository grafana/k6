package types

import (
	"errors"
	"fmt"
	"math/big"
	"net"
	"strings"
)

// ipBlock represents a continuous segment of IP addresses
type ipBlock struct {
	firstIP, count *big.Int
	ipv6           bool
}

// ipPoolBlock is similar to ipBlock but instead of knowing its count/size it knows the first index
// from which it starts in an IPPool
type ipPoolBlock struct {
	firstIP, startIndex *big.Int
}

// IPPool represent a slice of IPBlocks
type IPPool struct {
	list  []ipPoolBlock
	count *big.Int
}

func getIPBlock(s string) (*ipBlock, error) {
	switch {
	case strings.Contains(s, "-"):
		return ipBlockFromRange(s)
	case strings.Contains(s, "/"):
		return ipBlockFromCIDR(s)
	default:
		if net.ParseIP(s) == nil {
			return nil, fmt.Errorf("%s is not a valid IP, IP range or CIDR", s)
		}
		return ipBlockFromRange(s + "-" + s)
	}
}

func ipBlockFromRange(s string) (*ipBlock, error) {
	ss := strings.SplitN(s, "-", 2)
	ip0, ip1 := net.ParseIP(ss[0]), net.ParseIP(ss[1])
	if ip0 == nil || ip1 == nil {
		return nil, errors.New("wrong IP range format: " + s)
	}
	if (ip0.To4() == nil) != (ip1.To4() == nil) { // XOR
		return nil, errors.New("mixed IP range format: " + s)
	}
	block := ipBlockFromTwoIPs(ip0, ip1)

	if block.count.Sign() <= 0 {
		return nil, errors.New("negative IP range: " + s)
	}
	return block, nil
}

func ipBlockFromTwoIPs(ip0, ip1 net.IP) *ipBlock {
	// This code doesn't do any checks on the validity of the arguments, that should be
	// done before and/or after it is called
	var block ipBlock
	block.firstIP = new(big.Int)
	block.count = new(big.Int)
	block.ipv6 = ip0.To4() == nil
	if block.ipv6 {
		block.firstIP.SetBytes(ip0.To16())
		block.count.SetBytes(ip1.To16())
	} else {
		block.firstIP.SetBytes(ip0.To4())
		block.count.SetBytes(ip1.To4())
	}
	block.count.Sub(block.count, block.firstIP)
	block.count.Add(block.count, big.NewInt(1))

	return &block
}

func ipBlockFromCIDR(s string) (*ipBlock, error) {
	_, pnet, err := net.ParseCIDR(s)
	if err != nil {
		return nil, fmt.Errorf("parseCIDR() failed parsing %s: %w", s, err)
	}
	ip0 := pnet.IP
	// TODO: this is just to copy it, it will probably be better to copy the bytes ...
	ip1 := net.ParseIP(ip0.String())
	if ip1.To4() == nil {
		ip1 = ip1.To16()
	} else {
		ip1 = ip1.To4()
	}
	for i := range ip1 {
		ip1[i] |= (255 ^ pnet.Mask[i])
	}
	block := ipBlockFromTwoIPs(ip0, ip1)
	// in the case of ipv4 if the network is bigger than 31 the first and last IP are reserved so we
	// need to reduce the addresses by 2 and increment the first ip
	if !block.ipv6 && big.NewInt(2).Cmp(block.count) < 0 {
		block.count.Sub(block.count, big.NewInt(2))
		block.firstIP.Add(block.firstIP, big.NewInt(1))
	}
	return block, nil
}

func (b ipPoolBlock) getIP(index *big.Int) net.IP {
	// TODO implement walking ipv6 networks first
	// that will probably require more math ... including knowing which is the next network and ...
	// thinking about it - it looks like it's going to be kind of hard or badly defined
	i := new(big.Int)
	i.Add(b.firstIP, index)
	// TODO use big.Int.FillBytes when golang 1.14 is no longer supported
	return net.IP(i.Bytes())
}

// NewIPPool returns an IPPool slice from the provided string representation that should be comma
// separated list of IPs, IP ranges(ip1-ip2) and CIDRs
func NewIPPool(ranges string) (*IPPool, error) {
	ss := strings.Split(ranges, ",")
	pool := &IPPool{}
	pool.list = make([]ipPoolBlock, len(ss))
	pool.count = new(big.Int)
	for i, bs := range ss {
		r, err := getIPBlock(bs)
		if err != nil {
			return nil, err
		}

		pool.list[i] = ipPoolBlock{
			firstIP:    r.firstIP,
			startIndex: new(big.Int).Set(pool.count), // this is how many there are until now
		}
		pool.count.Add(pool.count, r.count)
	}

	// The list gets reversed here as later it is searched based on when the index we are looking is
	// bigger than startIndex but it will be true always for the first block which is with
	// startIndex 0. This can also be fixed by iterating in reverse but this seems better
	for i := 0; i < len(pool.list)/2; i++ {
		pool.list[i], pool.list[len(pool.list)-1-i] = pool.list[len(pool.list)-1-i], pool.list[i]
	}
	return pool, nil
}

// GetIP return an IP from a pool of IPBlock slice
func (pool *IPPool) GetIP(index uint64) net.IP {
	return pool.GetIPBig(new(big.Int).SetUint64(index))
}

// GetIPBig returns an IP from the pool with the provided index that is big.Int
func (pool *IPPool) GetIPBig(index *big.Int) net.IP {
	index = new(big.Int).Rem(index, pool.count)
	for _, b := range pool.list {
		if index.Cmp(b.startIndex) >= 0 {
			return b.getIP(index.Sub(index, b.startIndex))
		}
	}
	return nil
}

// NullIPPool is a nullable IPPool
type NullIPPool struct {
	Pool  *IPPool
	Valid bool
	raw   []byte
}

// UnmarshalText converts text data to a valid NullIPPool
func (n *NullIPPool) UnmarshalText(data []byte) error {
	if len(data) == 0 {
		*n = NullIPPool{}
		return nil
	}
	var err error
	n.Pool, err = NewIPPool(string(data))
	if err != nil {
		return err
	}
	n.Valid = true
	n.raw = data
	return nil
}

// MarshalText returns the IPs pool in text form
func (n *NullIPPool) MarshalText() ([]byte, error) {
	return n.raw, nil
}
