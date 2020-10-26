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
	// TODO rename count as it is actually the accumulation of the counts up to and including this
	// on in the pool
	// maybe add another field, although technically we need count only to find out when to enter this
	// block not for anything else
	firstIP, count *big.Int
	ipv6           bool
}

// IPPool represent a slice of IPBlocks
type IPPool struct {
	list  []ipBlock
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
			return nil, fmt.Errorf("%s is not a valid ip, range ip or CIDR", s)
		}
		return ipBlockFromRange(s + "-" + s)
	}
}

func ipBlockFromRange(s string) (*ipBlock, error) {
	ss := strings.SplitN(s, "-", 2)
	ip0Str, ip1Str := strings.TrimSpace(ss[0]), strings.TrimSpace(ss[1])
	ip0, ip1 := net.ParseIP(ip0Str), net.ParseIP(ip1Str)
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
	_, pnet, err := net.ParseCIDR(s) // range start ip, cidr ipnet
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

func (b ipBlock) getIP(index *big.Int) net.IP {
	// TODO implement walking ipv6 networks first
	// that will probably require more math ... including knowing which is the next network and ...
	// thinking about it - it looks like it's going to be kind of hard or badly defined
	i := new(big.Int)
	i.Add(b.firstIP, index)
	// TODO use big.Int.FillBytes when golang 1.14 is no longer supported
	return net.IP(i.Bytes())
}

// NewIPPool return an IPBlock slice with corret weight and mode
// Possible format is range1[:mode[:weight]][,range2[:mode[:weight]]]
func NewIPPool(ranges string) (*IPPool, error) {
	ss := strings.Split(strings.TrimSpace(ranges), ",")
	pool := &IPPool{}
	pool.list = make([]ipBlock, len(ss))
	pool.count = new(big.Int)
	for i, bs := range ss {
		r, err := getIPBlock(bs)
		if err != nil {
			return nil, err
		}

		pool.count.Add(pool.count, r.count)
		r.count.Set(pool.count)
		pool.list[i] = *r
	}
	return pool, nil
}

// GetIP return an IP from a pool of IPBlock slice
func (pool *IPPool) GetIP(id uint64) net.IP {
	return pool.GetIPBig(new(big.Int).SetUint64(id))
}

// GetIPBig returns an IP form the pool with the provided id that is big.Int
func (pool *IPPool) GetIPBig(id *big.Int) net.IP {
	id = new(big.Int).Rem(id, pool.count)
	for i, b := range pool.list {
		if id.Cmp(b.count) < 0 {
			if i > 0 {
				id.Sub(id, pool.list[i-1].count)
			}
			return b.getIP(id)
		}
	}
	return nil
}

// NullIPPool is a nullable IPPool
type NullIPPool struct {
	Pool  *IPPool
	Valid bool
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
	return nil
}
