package netext

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/lib/testutils/mockresolver"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/types"
)

func TestDialerAddr(t *testing.T) {
	t.Parallel()
	dialer := NewDialer(net.Dialer{}, newResolver())
	hosts, err := types.NewHosts(
		map[string]types.Host{
			"example.com":                {IP: net.ParseIP("3.4.5.6")},
			"example.com:443":            {IP: net.ParseIP("3.4.5.6"), Port: 8443},
			"example.com:8080":           {IP: net.ParseIP("3.4.5.6"), Port: 9090},
			"example-deny-host.com":      {IP: net.ParseIP("8.9.10.11")},
			"example-ipv6.com":           {IP: net.ParseIP("2001:db8::68")},
			"example-ipv6.com:443":       {IP: net.ParseIP("2001:db8::68"), Port: 8443},
			"example-ipv6-deny-host.com": {IP: net.ParseIP("::1")},
		})
	require.NoError(t, err)
	dialer.Hosts = hosts
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
		t.Run(tc.address, func(t *testing.T) {
			t.Parallel()
			addr, err := dialer.getDialAddr(tc.address)

			if tc.expErr != "" {
				require.EqualError(t, err, tc.expErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expAddress, addr.String())
			}
		})
	}
}

func TestDialerAddrBlockHostnamesStar(t *testing.T) {
	t.Parallel()
	dialer := NewDialer(net.Dialer{}, newResolver())
	hosts, err := types.NewHosts(map[string]types.Host{
		"example.com": {IP: net.ParseIP("3.4.5.6")},
	})
	require.NoError(t, err)
	dialer.Hosts = hosts

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
		t.Run(tc.address, func(t *testing.T) {
			t.Parallel()
			addr, err := dialer.getDialAddr(tc.address)

			if tc.expErr != "" {
				require.EqualError(t, err, tc.expErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expAddress, addr.String())
			}
		})
	}
}

func TestDialerAddrAllowHostnames(t *testing.T) {
	t.Parallel()
	dialer := NewDialer(net.Dialer{}, newResolver())
	hosts, err := types.NewHosts(map[string]types.Host{
		"allowed.com":       {IP: net.ParseIP("1.2.3.4")},
		"example.com":       {IP: net.ParseIP("5.6.7.8")},
		"test.wildcard.com": {IP: net.ParseIP("9.10.11.12")},
	})
	require.NoError(t, err)
	dialer.Hosts = hosts

	allowed, err := types.NewHostnameTrie([]string{"allowed.com", "*.wildcard.com"})
	require.NoError(t, err)
	dialer.AllowedHostnames = allowed

	testCases := []struct {
		address, expAddress, expErr string
	}{
		// Allowed hostnames
		{"allowed.com:80", "1.2.3.4:80", ""},
		{"test.wildcard.com:443", "9.10.11.12:443", ""},
		// Not allowed hostnames
		{"example.com:80", "", "hostname (example.com) is not in the allowed hostnames list"},
		{"notallowed.com:80", "", "hostname (notallowed.com) is not in the allowed hostnames list"},
		// IPs should bypass the allowlist
		{"1.2.3.4:80", "1.2.3.4:80", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.address, func(t *testing.T) {
			t.Parallel()
			addr, err := dialer.getDialAddr(tc.address)

			if tc.expErr != "" {
				require.EqualError(t, err, tc.expErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expAddress, addr.String())
			}
		})
	}
}

// Benchmarks /etc/hosts like hostname mapping
func BenchmarkDialerHosts(b *testing.B) {
	hosts, err := types.NewHosts(map[string]types.Host{
		"k6.io":                {IP: []byte("192.168.1.1"), Port: 80},
		"specific.k6.io":       {IP: []byte("192.168.1.2"), Port: 80},
		"grafana.com":          {IP: []byte("aa::ff"), Port: 80},
		"specific.grafana.com": {IP: []byte("aa:bb:::ff"), Port: 80},
	})
	require.NoError(b, err)

	dialer := Dialer{
		Dialer: net.Dialer{},
		Hosts:  hosts,
	}

	tcs := []string{"k6.io", "specific.k6.io", "grafana.com", "specific.grafana.com", "not.exists.com"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tc := range tcs {
			//nolint:errcheck
			dialer.getDialAddr(tc)
		}
	}
}

func TestDialerResolveAddr(t *testing.T) {
	t.Parallel()
	dialer := NewDialer(net.Dialer{}, newResolver())
	hosts, err := types.NewHosts(
		map[string]types.Host{
			"example.com":                {IP: net.ParseIP("3.4.5.6")},
			"example.com:443":            {IP: net.ParseIP("3.4.5.6"), Port: 8443},
			"example.com:8080":           {IP: net.ParseIP("3.4.5.6"), Port: 9090},
			"example-deny-host.com":      {IP: net.ParseIP("8.9.10.11")},
			"example-ipv6.com":           {IP: net.ParseIP("2001:db8::68")},
			"example-ipv6.com:443":       {IP: net.ParseIP("2001:db8::68"), Port: 8443},
			"example-ipv6-deny-host.com": {IP: net.ParseIP("::1")},
		})
	require.NoError(t, err)
	dialer.Hosts = hosts
	ipNet, err := lib.ParseCIDR("8.9.10.0/24")
	require.NoError(t, err)

	ipV6Net, err := lib.ParseCIDR("::1/24")
	require.NoError(t, err)

	dialer.Blacklist = []*lib.IPNet{ipNet, ipV6Net}

	testCases := []struct {
		name          string
		address       string
		expectedIP    string
		expectedPort  int
		expectedError string
	}{
		// IPv4 with port
		{"IPv4_with_resolver", "example-resolver.com:80", "1.2.3.4", 80, ""},
		{"IPv4_with_hosts_mapping", "example.com:80", "3.4.5.6", 80, ""},
		{"IPv4_with_custom_port_in_hosts", "example.com:443", "3.4.5.6", 8443, ""},
		{"IPv4_with_different_custom_port", "example.com:8080", "3.4.5.6", 9090, ""},
		{"IPv4_direct_IP", "1.2.3.4:80", "1.2.3.4", 80, ""},
		{"IPv4_blacklisted_via_resolver", "example-deny-resolver.com:80", "", 0, "IP (8.9.10.11) is in a blacklisted range (8.9.10.0/24)"},
		{"IPv4_blacklisted_via_hosts", "example-deny-host.com:80", "", 0, "IP (8.9.10.11) is in a blacklisted range (8.9.10.0/24)"},
		{"IPv4_non-existent_host", "no-such-host.com:80", "", 0, "lookup no-such-host.com: no such host"},

		// IPv4 without port
		{"IPv4_without_port", "example-resolver.com", "1.2.3.4", 0, ""},
		{"IPv4_hosts_mapping_without_port", "example.com", "3.4.5.6", 0, ""},
		{"IPv4_direct_IP_without_port", "1.2.3.4", "1.2.3.4", 0, ""},

		// IPv6 with port
		{"IPv6_with_hosts_mapping", "example-ipv6.com:443", "2001:db8::68", 8443, ""},
		{"IPv6_direct_IP", "[2001:db8:aaaa:1::100]:443", "2001:db8:aaaa:1::100", 443, ""},
		{"IPv6_blacklisted_via_resolver", "example-ipv6-deny-resolver.com:80", "", 0, "IP (::1) is in a blacklisted range (::/24)"},
		{"IPv6_blacklisted_via_hosts", "example-ipv6-deny-host.com:80", "", 0, "IP (::1) is in a blacklisted range (::/24)"},

		// IPv6 without port
		{"IPv6_without_port", "example-ipv6.com", "2001:db8::68", 0, ""},
		{"IPv6_direct_IP_without_port", "2001:db8:aaaa:1::100", "2001:db8:aaaa:1::100", 0, ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ip, port, err := dialer.ResolveAddr(tc.address)

			if tc.expectedError != "" {
				require.EqualError(t, err, tc.expectedError)
				require.Nil(t, ip)
				require.Equal(t, 0, port)
			} else {
				require.NoError(t, err)
				require.Equal(t, net.ParseIP(tc.expectedIP), ip)
				require.Equal(t, tc.expectedPort, port)
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
		},
	)
}
