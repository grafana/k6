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

package netext

import (
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/loadimpact/k6/lib"
)

type mockResolver struct {
	hosts map[string]net.IP
}

func (r *mockResolver) FetchOne(h string) (net.IP, error) {
	var (
		ip net.IP
		ok bool
	)
	if ip, ok = r.hosts[h]; !ok {
		return nil, fmt.Errorf("mock lookup %s: no such host", h)
	}
	return ip, nil
}

func TestDialerCheckAndResolveAddress(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		address, expAddress, expErr string
	}{
		// IPv4
		{"1.2.3.4:80", "1.2.3.4:80", ""},
		{"example.com:443", "93.184.216.34:443", ""},
		{"mycustomv4.host:443", "1.2.3.4:443", ""},
		{"1.2.3.4", "", "address 1.2.3.4: missing port in address"},
		{"256.1.1.1:80", "", "mock lookup 256.1.1.1: no such host"},
		{"blockedv4.host:443", "", "IP (10.0.0.10) is in a blacklisted range (10.0.0.0/8)"},

		// IPv6
		{"::1", "", "address ::1: too many colons in address"},
		{"[::1.2.3.4]", "", "address [::1.2.3.4]: missing port in address"},
		{"[::1.2.3.4]:443", "[::102:304]:443", ""},
		{"[abcd:ef01:2345:6789]:443", "", "mock lookup abcd:ef01:2345:6789: no such host"},
		{"[2001:db8:aaaa:1::100]:443", "[2001:db8:aaaa:1::100]:443", ""},
		{"ipv6.google.com:443", "[2a00:1450:4007:812::200e]:443", ""},
		{"blockedv6.host:443", "", "IP (2600::1) is in a blacklisted range (2600::/64)"},
	}

	block4, err := lib.ParseCIDR("10.0.0.0/8")
	require.NoError(t, err)
	block6, err := lib.ParseCIDR("2600::/64")
	require.NoError(t, err)

	dialer := &Dialer{
		Blacklist: []*lib.IPNet{block4, block6},
		Hosts: map[string]net.IP{
			"mycustomv4.host": net.ParseIP("1.2.3.4"),
			"mycustomv6.host": net.ParseIP("::1"),
		},
	}
	resolver := &mockResolver{hosts: map[string]net.IP{
		"example.com":     net.ParseIP("93.184.216.34"),
		"ipv6.google.com": net.ParseIP("2a00:1450:4007:812::200e"),
		"blockedv4.host":  net.ParseIP("10.0.0.10"),
		"blockedv6.host":  net.ParseIP("2600::1"),
	}}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.address, func(t *testing.T) {
			address, err := dialer.checkAndResolveAddress(tc.address, resolver)
			if tc.expErr != "" {
				assert.EqualError(t, err, tc.expErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expAddress, address)
		})
	}
}
