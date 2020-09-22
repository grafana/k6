package dns

import (
	"context"
	cryptorand "crypto/rand"
	"errors"
	"io"
	"math/big"
	"net"
	"sync/atomic"
)

// NameServers is a slice of DNS nameserver addresses.
type NameServers []net.Addr

// Random picks a random Addr from s every time.
func (s NameServers) Random(rand io.Reader) ProxyFunc {
	addrsByNet := s.netAddrsMap()

	maxByNet := make(map[string]*big.Int, len(addrsByNet))
	for network, addrs := range addrsByNet {
		maxByNet[network] = big.NewInt(int64(len(addrs)))
	}

	return func(_ context.Context, addr net.Addr) (net.Addr, error) {
		network := addr.Network()
		max, ok := maxByNet[network]
		if !ok {
			return nil, errors.New("no nameservers for network: " + network)
		}

		addrs, ok := addrsByNet[network]
		if !ok {
			panic("impossible")
		}

		idx, err := cryptorand.Int(rand, max)
		if err != nil {
			return nil, err
		}

		return addrs[idx.Uint64()], nil
	}
}

// RoundRobin picks the next Addr of s by index of the last pick.
func (s NameServers) RoundRobin() ProxyFunc {
	addrsByNet := s.netAddrsMap()

	idxByNet := make(map[string]*uint32, len(s))
	for network := range addrsByNet {
		idxByNet[network] = new(uint32)
	}

	return func(_ context.Context, addr net.Addr) (net.Addr, error) {
		network := addr.Network()
		idx, ok := idxByNet[network]
		if !ok {
			return nil, errors.New("no nameservers for network: " + network)
		}

		addrs, ok := addrsByNet[network]
		if !ok {
			panic("impossible")
		}

		return addrs[int(atomic.AddUint32(idx, 1)-1)%len(addrs)], nil
	}
}

func (s NameServers) netAddrsMap() map[string][]net.Addr {
	addrsByNet := make(map[string][]net.Addr, len(s))
	for _, addr := range s {
		network := addr.Network()
		addrsByNet[network] = append(addrsByNet[network], addr)
	}
	return addrsByNet
}
