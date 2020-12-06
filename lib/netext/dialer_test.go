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
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/testutils/mockresolver"
	"github.com/loadimpact/k6/lib/types"
)

func TestDialerAddr(t *testing.T) {
	dialer := NewDialer(net.Dialer{}, newResolver())
	dialer.Hosts = map[string]*lib.HostAddress{
		"example.com":                {IP: net.ParseIP("3.4.5.6")},
		"example.com:443":            {IP: net.ParseIP("3.4.5.6"), Port: 8443},
		"example.com:8080":           {IP: net.ParseIP("3.4.5.6"), Port: 9090},
		"example-deny-host.com":      {IP: net.ParseIP("8.9.10.11")},
		"example-ipv6.com":           {IP: net.ParseIP("2001:db8::68")},
		"example-ipv6.com:443":       {IP: net.ParseIP("2001:db8::68"), Port: 8443},
		"example-ipv6-deny-host.com": {IP: net.ParseIP("::1")},
	}

	ipNet, err := lib.ParseCIDR("8.9.10.0/24")
	require.NoError(t, err)

	ipV6Net, err := lib.ParseCIDR("::1/24")
	require.NoError(t, err)

	dialer.Blacklist = []*lib.IPNet{ipNet, ipV6Net}

	testCases := []struct {
		address, expAddress, expErr string
	}{
		// IPv4
		{"example-resolver.com:80", "1.2.3.4:80", ""},
		{"example.com:80", "3.4.5.6:80", ""},
		{"example.com:443", "3.4.5.6:8443", ""},
		{"example.com:8080", "3.4.5.6:9090", ""},
		{"1.2.3.4:80", "1.2.3.4:80", ""},
		{"1.2.3.4", "", "address 1.2.3.4: missing port in address"},
		{"example-deny-resolver.com:80", "", "IP (8.9.10.11) is in a blacklisted range (8.9.10.0/24)"},
		{"example-deny-host.com:80", "", "IP (8.9.10.11) is in a blacklisted range (8.9.10.0/24)"},
		{"no-such-host.com:80", "", "lookup no-such-host.com: no such host"},

		// IPv6
		{"example-ipv6.com:443", "[2001:db8::68]:8443", ""},
		{"[2001:db8:aaaa:1::100]:443", "[2001:db8:aaaa:1::100]:443", ""},
		{"[::1.2.3.4]", "", "address [::1.2.3.4]: missing port in address"},
		{"example-ipv6-deny-resolver.com:80", "", "IP (::1) is in a blacklisted range (::/24)"},
		{"example-ipv6-deny-host.com:80", "", "IP (::1) is in a blacklisted range (::/24)"},
		{"example-ipv6-deny-host.com:80", "", "IP (::1) is in a blacklisted range (::/24)"},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.address, func(t *testing.T) {
			addr, err := dialer.getDialAddr(tc.address)

			if tc.expErr != "" {
				require.EqualError(t, err, tc.expErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expAddress, addr)
			}
		})
	}
}

func TestDialerAddrBlockHostnamesStar(t *testing.T) {
	dialer := NewDialer(net.Dialer{}, newResolver())
	dialer.Hosts = map[string]*lib.HostAddress{
		"example.com": {IP: net.ParseIP("3.4.5.6")},
	}

	blocked, err := types.NewHostnameTrie([]string{"*"})
	require.NoError(t, err)
	dialer.BlockedHostnames = blocked
	testCases := []struct {
		address, expAddress, expErr string
	}{
		// IPv4
		{"example.com:80", "", "hostname (example.com) is in a blocked pattern (*)"},
		{"example.com:443", "", "hostname (example.com) is in a blocked pattern (*)"},
		{"not.com:30", "", "hostname (not.com) is in a blocked pattern (*)"},
		{"1.2.3.4:80", "1.2.3.4:80", ""},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.address, func(t *testing.T) {
			addr, err := dialer.getDialAddr(tc.address)

			if tc.expErr != "" {
				require.EqualError(t, err, tc.expErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expAddress, addr)
			}
		})
	}
}

func newResolver() *mockresolver.MockResolver {
	return mockresolver.New(
		map[string][]net.IP{
			"example-resolver.com":           {net.ParseIP("1.2.3.4")},
			"example-deny-resolver.com":      {net.ParseIP("8.9.10.11")},
			"example-ipv6-deny-resolver.com": {net.ParseIP("::1")},
		}, nil,
	)
}
