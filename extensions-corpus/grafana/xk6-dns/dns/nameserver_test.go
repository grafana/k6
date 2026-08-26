package dns

import (
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_parseNameserverAddr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		addr     string
		wantIP   net.IP
		wantPort uint16
		wantErr  assert.ErrorAssertionFunc
	}{
		{
			name:     "IPv4 address with port",
			addr:     "192.168.1.1:8080",
			wantIP:   net.ParseIP("192.168.1.1"),
			wantPort: 8080,
			wantErr:  assert.NoError,
		},
		{
			name:     "IPv4 address without port",
			addr:     "192.168.1.1",
			wantIP:   net.ParseIP("192.168.1.1"),
			wantPort: 53,
			wantErr:  assert.NoError,
		},
		{
			name:     "IPv6 with port",
			addr:     "[2001:db8::1]:8080",
			wantIP:   net.ParseIP("2001:db8::1"),
			wantPort: 8080,
			wantErr:  assert.NoError,
		},
		{
			name:     "IPv6 without port",
			addr:     "[2001:db8::1]",
			wantIP:   net.ParseIP("2001:db8::1"),
			wantPort: 53,
			wantErr:  assert.NoError,
		},
		{
			name:     "Invalid IPv4 address",
			addr:     "invalid:53",
			wantIP:   nil,
			wantPort: 0,
			wantErr:  assert.Error,
		},
		{
			name:     "Invalid IPv6 address",
			addr:     "invalid]:53",
			wantIP:   nil,
			wantPort: 0,
			wantErr:  assert.Error,
		},
		{
			name:     "Invalid port",
			addr:     "192.168.1.1:invalid",
			wantIP:   nil,
			wantPort: 0,
			wantErr:  assert.Error,
		},
		{
			name:     "Port out of range",
			addr:     "192.168.1.1:99999",
			wantIP:   nil,
			wantPort: 0,
			wantErr:  assert.Error,
		},
		{
			"missing closing bracket for IPv6 address",
			"[2001:db8::1:8080",
			nil,
			0,
			assert.Error,
		},
		// IPv6 without brackets - the main bug from issue #20
		{
			name:     "IPv6 without brackets or port (issue #20)",
			addr:     "2606:4700:4700::1111",
			wantIP:   net.ParseIP("2606:4700:4700::1111"),
			wantPort: 53,
			wantErr:  assert.NoError,
		},
		{
			name:     "IPv6 loopback without brackets",
			addr:     "::1",
			wantIP:   net.ParseIP("::1"),
			wantPort: 53,
			wantErr:  assert.NoError,
		},
		{
			name:     "IPv6 loopback with brackets",
			addr:     "[::1]",
			wantIP:   net.ParseIP("::1"),
			wantPort: 53,
			wantErr:  assert.NoError,
		},
		{
			name:     "IPv6 loopback with brackets and port",
			addr:     "[::1]:5353",
			wantIP:   net.ParseIP("::1"),
			wantPort: 5353,
			wantErr:  assert.NoError,
		},
		{
			name:     "Compressed IPv6 link-local without brackets",
			addr:     "fe80::1",
			wantIP:   net.ParseIP("fe80::1"),
			wantPort: 53,
			wantErr:  assert.NoError,
		},
		{
			name:     "Compressed IPv6 link-local with brackets and port",
			addr:     "[fe80::1]:8053",
			wantIP:   net.ParseIP("fe80::1"),
			wantPort: 8053,
			wantErr:  assert.NoError,
		},
		{
			name:     "Full form IPv6 without brackets",
			addr:     "2001:0db8:0000:0000:0000:0000:0000:0001",
			wantIP:   net.ParseIP("2001:0db8:0000:0000:0000:0000:0000:0001"),
			wantPort: 53,
			wantErr:  assert.NoError,
		},
		{
			name:     "Full form IPv6 with brackets and port",
			addr:     "[2001:0db8:0000:0000:0000:0000:0000:0001]:5353",
			wantIP:   net.ParseIP("2001:0db8:0000:0000:0000:0000:0000:0001"),
			wantPort: 5353,
			wantErr:  assert.NoError,
		},
		{
			name:     "IPv6 all zeros compressed",
			addr:     "::",
			wantIP:   net.ParseIP("::"),
			wantPort: 53,
			wantErr:  assert.NoError,
		},
		{
			name:     "IPv6 with trailing compression",
			addr:     "2001:db8::",
			wantIP:   net.ParseIP("2001:db8::"),
			wantPort: 53,
			wantErr:  assert.NoError,
		},
		{
			name:     "IPv6 with leading compression",
			addr:     "::ffff:192.0.2.1",
			wantIP:   net.ParseIP("::ffff:192.0.2.1"),
			wantPort: 53,
			wantErr:  assert.NoError,
		},
		{
			name:     "Invalid IPv6 - too many colons",
			addr:     "2001:db8:::1",
			wantIP:   nil,
			wantPort: 0,
			wantErr:  assert.Error,
		},
		{
			name:     "Invalid IPv6 - invalid hex",
			addr:     "gggg::1",
			wantIP:   nil,
			wantPort: 0,
			wantErr:  assert.Error,
		},
		{
			name:     "Invalid IPv6 - incomplete",
			addr:     "2606:4700:",
			wantIP:   nil,
			wantPort: 0,
			wantErr:  assert.Error,
		},
		{
			name:     "IPv6 with port but no brackets - ambiguous (should fail)",
			addr:     "::1:99999",
			wantIP:   nil,
			wantPort: 0,
			wantErr:  assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotNameserver, err := parseNameserverAddr(tt.addr)
			if !tt.wantErr(t, err, fmt.Sprintf("parseNameserverAddr(%v)", tt.addr)) {
				return
			}
			assert.Equalf(t, tt.wantIP, gotNameserver.IP, "parseNameserverAddr(%v)", tt.addr)
			assert.Equalf(t, tt.wantPort, gotNameserver.Port, "parseNameserverAddr(%v)", tt.addr)
		})
	}
}
