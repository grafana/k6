package types

import (
	"math/big"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func get128BigInt(hi, lo int64) *big.Int {
	max64 := new(big.Int).Exp(big.NewInt(2), big.NewInt(64), nil)
	h := big.NewInt(hi)
	h.Mul(h, max64)
	return h.Add(h, big.NewInt(lo))
}

func TestIpBlock(t *testing.T) {
	t.Parallel()
	testdata := map[string]struct {
		count           *big.Int
		firstIP, lastIP net.IP
	}{
		"192.168.0.101": {new(big.Int).SetInt64(1), net.ParseIP("192.168.0.101"), net.ParseIP("192.168.0.101")},

		"192.168.0.101-192.168.0.200":    {new(big.Int).SetInt64(100), net.ParseIP("192.168.0.101"), net.ParseIP("192.168.0.200")},
		"192.168.0.100-192.168.0.200":    {new(big.Int).SetInt64(101), net.ParseIP("192.168.0.100"), net.ParseIP("192.168.0.200")},
		"fd00:1:1:0::0-fd00:1:1:ff::3ff": {get128BigInt(255, 1024), net.ParseIP("fd00:1:1:0::0"), net.ParseIP("fd00:1:1:ff::3ff")},
		"fd00:1:1:2::1-fd00:1:1:ff::3ff": {get128BigInt(253, 1023), net.ParseIP("fd00:1:1:2::1"), net.ParseIP("fd00:1:1:ff::3ff")},

		"192.168.0.0/16":  {get128BigInt(0, 65534), net.ParseIP("192.168.0.1"), net.ParseIP("192.168.255.254")},
		"192.168.0.1/16":  {get128BigInt(0, 65534), net.ParseIP("192.168.0.1"), net.ParseIP("192.168.255.254")},
		"192.168.0.10/16": {get128BigInt(0, 65534), net.ParseIP("192.168.0.1"), net.ParseIP("192.168.255.254")},
		"192.168.0.10/31": {get128BigInt(0, 2), net.ParseIP("192.168.0.10"), net.ParseIP("192.168.0.11")},
		"192.168.0.10/32": {get128BigInt(0, 1), net.ParseIP("192.168.0.10"), net.ParseIP("192.168.0.10")},
		"fd00::0/120":     {get128BigInt(0, 256), net.ParseIP("fd00::0"), net.ParseIP("fd00::ff")},
		"fd00::1/120":     {get128BigInt(0, 256), net.ParseIP("fd00::0"), net.ParseIP("fd00::ff")},
		"fd00::3/120":     {get128BigInt(0, 256), net.ParseIP("fd00::0"), net.ParseIP("fd00::ff")},
		"fd00::0/112":     {get128BigInt(0, 65536), net.ParseIP("fd00::0"), net.ParseIP("fd00::ffff")},
		"fd00::1/112":     {get128BigInt(0, 65536), net.ParseIP("fd00::0"), net.ParseIP("fd00::ffff")},
		"fd00::2/112":     {get128BigInt(0, 65536), net.ParseIP("fd00::0"), net.ParseIP("fd00::ffff")},
	}
	for name, data := range testdata {
		name, data := name, data
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b, err := getIPBlock(name)
			require.NoError(t, err)
			assert.Equal(t, data.count, b.count)
			pb := ipPoolBlock{firstIP: b.firstIP}
			idx := big.NewInt(0)
			assert.Equal(t, data.firstIP.To16(), pb.getIP(idx).To16())
			idx.Sub(idx.Add(idx, b.count), big.NewInt(1))
			assert.Equal(t, data.lastIP.To16(), pb.getIP(idx).To16())
		})
	}
}

func TestIPPool(t *testing.T) {
	t.Parallel()
	testdata := map[string]struct {
		count   *big.Int
		queries map[uint64]net.IP
	}{
		"192.168.0.101": {
			count:   new(big.Int).SetInt64(1),
			queries: map[uint64]net.IP{0: net.ParseIP("192.168.0.101"), 12: net.ParseIP("192.168.0.101")},
		},
		"192.168.0.101,192.168.0.102": {
			count: new(big.Int).SetInt64(2),
			queries: map[uint64]net.IP{
				0:  net.ParseIP("192.168.0.101"),
				1:  net.ParseIP("192.168.0.102"),
				12: net.ParseIP("192.168.0.101"),
				13: net.ParseIP("192.168.0.102"),
			},
		},
		"192.168.0.101-192.168.0.105,fd00::2/112": {
			count: new(big.Int).SetInt64(65541),
			queries: map[uint64]net.IP{
				0:     net.ParseIP("192.168.0.101"),
				1:     net.ParseIP("192.168.0.102"),
				5:     net.ParseIP("fd00::0"),
				6:     net.ParseIP("fd00::1"),
				65541: net.ParseIP("192.168.0.101"),
			},
		},

		"192.168.0.101,192.168.0.102,192.168.0.103,192.168.0.104,192.168.0.105-192.168.0.105,fd00::2/112": {
			count: new(big.Int).SetInt64(65541),
			queries: map[uint64]net.IP{
				0:     net.ParseIP("192.168.0.101"),
				1:     net.ParseIP("192.168.0.102"),
				2:     net.ParseIP("192.168.0.103"),
				3:     net.ParseIP("192.168.0.104"),
				4:     net.ParseIP("192.168.0.105"),
				5:     net.ParseIP("fd00::0"),
				6:     net.ParseIP("fd00::1"),
				65541: net.ParseIP("192.168.0.101"),
			},
		},
	}
	for name, data := range testdata {
		name, data := name, data
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			p, err := NewIPPool(name)
			require.NoError(t, err)
			assert.Equal(t, data.count, p.count)
			for q, a := range data.queries {
				assert.Equal(t, a.To16(), p.GetIP(q).To16(), "index %d", q)
			}
		})
	}
}

func TestIpBlockError(t *testing.T) {
	t.Parallel()
	testdata := map[string]string{
		"whatever":                       "not a valid IP",
		"192.168.0.1012":                 "not a valid IP",
		"192.168.0.10/244":               "invalid CIDR",
		"fd00::0/244":                    "invalid CIDR",
		"192.168.0.101-192.168.0.102/32": "wrong IP range format",
		"192.168.0.101-fd00::1":          "mixed IP range format",
		"fd00::1-192.168.0.101":          "mixed IP range format",
		"192.168.0.100-192.168.0.2":      "negative IP range",
		"fd00:1:1:0::0-fd00:1:0:ff::3ff": "negative IP range",
	}
	for name, data := range testdata {
		name, data := name, data
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := getIPBlock(name)
			require.Error(t, err)
			require.Contains(t, err.Error(), data)
		})
	}
}

func TestNullIPPoolMarshalText(t *testing.T) {
	t.Parallel()

	rangeInput := "192.168.20.12-192.168.20.15,192.168.10.0/27"
	p, err := NewIPPool(rangeInput)
	require.NoError(t, err)

	nullpool := NullIPPool{
		Pool:  p,
		Valid: true,
		raw:   []byte(rangeInput),
	}
	text, err := nullpool.MarshalText()
	require.NoError(t, err)
	assert.Equal(t, "192.168.20.12-192.168.20.15,192.168.10.0/27", string(text))
}
