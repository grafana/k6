package netext

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/lib/testutils/mockresolver"
	"go.k6.io/k6/lib/types"
)

func TestResolver(t *testing.T) {
	t.Parallel()

	host := "myhost"
	mr := mockresolver.New(map[string][]net.IP{
		host: {
			net.ParseIP("127.0.0.10"),
			net.ParseIP("127.0.0.11"),
			net.ParseIP("127.0.0.12"),
			net.ParseIP("2001:db8::10"),
			net.ParseIP("2001:db8::11"),
			net.ParseIP("2001:db8::12"),
		},
	})

	t.Run("LookupIP", func(t *testing.T) {
		t.Parallel()
		testCases := []struct {
			ttl   time.Duration
			sel   types.DNSSelect
			pol   types.DNSPolicy
			expIP []net.IP
		}{
			{
				0, types.DNSfirst, types.DNSpreferIPv4,
				[]net.IP{net.ParseIP("127.0.0.10")},
			},
			{
				time.Second, types.DNSfirst, types.DNSpreferIPv4,
				[]net.IP{net.ParseIP("127.0.0.10")},
			},
			{0, types.DNSroundRobin, types.DNSonlyIPv6, []net.IP{
				net.ParseIP("2001:db8::10"),
				net.ParseIP("2001:db8::11"),
				net.ParseIP("2001:db8::12"),
				net.ParseIP("2001:db8::10"),
			}},
			{
				0, types.DNSfirst, types.DNSpreferIPv6,
				[]net.IP{net.ParseIP("2001:db8::10")},
			},
			{0, types.DNSroundRobin, types.DNSpreferIPv4, []net.IP{
				net.ParseIP("127.0.0.10"),
				net.ParseIP("127.0.0.11"),
				net.ParseIP("127.0.0.12"),
				net.ParseIP("127.0.0.10"),
			}},
		}

		for _, tc := range testCases {
			tc := tc
			t.Run(fmt.Sprintf("%s_%s_%s", tc.ttl, tc.sel, tc.pol), func(t *testing.T) {
				t.Parallel()
				r := NewResolver(mr.LookupIPAll, tc.ttl, tc.sel, tc.pol)
				ip, err := r.LookupIP(host)
				require.NoError(t, err)
				assert.Equal(t, tc.expIP[0], ip)

				if tc.ttl > 0 {
					require.IsType(t, &cacheResolver{}, r)
					cr, ok := r.(*cacheResolver)
					assert.True(t, ok)
					assert.Len(t, cr.cache, 1)
					assert.Equal(t, tc.ttl, cr.ttl)
					firstLookup := cr.cache[host].lastLookup
					time.Sleep(cr.ttl + 100*time.Millisecond)
					_, err = r.LookupIP(host)
					require.NoError(t, err)
					assert.True(t, cr.cache[host].lastLookup.After(firstLookup))
				}

				if tc.sel == types.DNSroundRobin {
					ips := []net.IP{ip}
					for i := 0; i < 3; i++ {
						ip, err = r.LookupIP(host)
						require.NoError(t, err)
						ips = append(ips, ip)
					}
					assert.Equal(t, tc.expIP, ips)
				}
			})
		}
	})
}
