package netext

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/viki-org/dnscache"
)

func TestDialerResolveHost(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		host   string
		hosts  map[string]net.IP
		ipVer  int
		expErr string
	}{
		{"1.2.3.4", nil, 4, ""},
		{"256.1.1.1", nil, 4, "lookup 256.1.1.1: no such host"},
		{"example.com", nil, 4, ""},
		{"::1", nil, 6, ""},
		{"::1.2.3.4", nil, 6, ""},
		{"abcd:ef01:2345:6789", nil, 6, "lookup abcd:ef01:2345:6789: no such host"},
		{"2001:db8:aaaa:1::100", nil, 6, ""},
		{"ipv6.google.com", nil, 6, ""},
		{"mycustomv4.host", map[string]net.IP{"mycustomv4.host": net.ParseIP("1.2.3.4")}, 4, ""},
		{"mycustomv6.host", map[string]net.IP{"mycustomv6.host": net.ParseIP("::1")}, 6, ""},
	}

	resolver := dnscache.New(0)
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.host, func(t *testing.T) {
			ip, err := resolveHost(tc.host, tc.hosts, resolver)
			if tc.expErr != "" {
				assert.EqualError(t, err, tc.expErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.ipVer == 6, ip.To4() == nil)
		})
	}
}
