/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
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

	"github.com/loadimpact/k6/lib"
	"github.com/stretchr/testify/assert"
)

type testResolver struct {
	hosts map[string]net.IP
}

func (r testResolver) FetchOne(host string) (net.IP, error) { return r.hosts[host], nil }

func TestDialerAddr(t *testing.T) {
	dialer := newDialerWithResolver(net.Dialer{}, newResolver())
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
	assert.NoError(t, err)

	ipV6Net, err := lib.ParseCIDR("::1/24")
	assert.NoError(t, err)

	dialer.Blacklist = []*lib.IPNet{ipNet, ipV6Net}

	addr, err := dialer.dialAddr("example-resolver.com:80")
	assert.NoError(t, err)
	assert.Equal(t, "1.2.3.4:80", addr)

	addr, err = dialer.dialAddr("example.com:80")
	assert.NoError(t, err)
	assert.Equal(t, "3.4.5.6:80", addr)

	addr, err = dialer.dialAddr("example.com:443")
	assert.NoError(t, err)
	assert.Equal(t, "3.4.5.6:8443", addr)

	addr, err = dialer.dialAddr("example.com:8080")
	assert.NoError(t, err)
	assert.Equal(t, "3.4.5.6:9090", addr)

	addr, err = dialer.dialAddr("example-ipv6.com:80")
	assert.NoError(t, err)
	assert.Equal(t, "[2001:db8::68]:80", addr)

	addr, err = dialer.dialAddr("example-ipv6.com:443")
	assert.NoError(t, err)
	assert.Equal(t, "[2001:db8::68]:8443", addr)

	_, err = dialer.dialAddr("example-deny-resolver.com:80")
	assert.EqualError(t, err, "IP (8.9.10.11) is in a blacklisted range (8.9.10.0/24)")

	_, err = dialer.dialAddr("example-deny-host.com:80")
	assert.EqualError(t, err, "IP (8.9.10.11) is in a blacklisted range (8.9.10.0/24)")

	_, err = dialer.dialAddr("example-ipv6-deny-resolver.com:80")
	assert.EqualError(t, err, "IP (::1) is in a blacklisted range (::/24)")

	_, err = dialer.dialAddr("example-ipv6-deny-host.com:80")
	assert.EqualError(t, err, "IP (::1) is in a blacklisted range (::/24)")
}

func newResolver() testResolver {
	return testResolver{
		hosts: map[string]net.IP{
			"example-resolver.com":           net.ParseIP("1.2.3.4"),
			"example-deny-resolver.com":      net.ParseIP("8.9.10.11"),
			"example-ipv6-deny-resolver.com": net.ParseIP("::1"),
		},
	}
}
