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

func TestDialerAddrWithDNSResolver(t *testing.T) {
	dialer := newDialerWithResolver(net.Dialer{}, newResolver())

	addr, err := dialer.dialAddr("example.com:80")
	assert.NoError(t, err)
	assert.Equal(t, "1.2.3.4:80", addr)
}

func TestDialerAddrWithCachedHost(t *testing.T) {
	dialer := newDialerWithResolver(net.Dialer{}, newResolver())
	dialer.Hosts = map[string]lib.IPPort{
		"example.com":      {IP: net.ParseIP("3.4.5.6"), Port: ""},
		"example.com:443":  {IP: net.ParseIP("3.4.5.6"), Port: "8443"},
		"example.com:8080": {IP: net.ParseIP("3.4.5.6"), Port: "9090"},
	}

	addr, err := dialer.dialAddr("example.com:80")
	assert.NoError(t, err)
	assert.Equal(t, "3.4.5.6:80", addr)

	addr, err = dialer.dialAddr("example.com:443")
	assert.NoError(t, err)
	assert.Equal(t, "3.4.5.6:8443", addr)

	addr, err = dialer.dialAddr("example.com:8080")
	assert.NoError(t, err)
	assert.Equal(t, "3.4.5.6:9090", addr)
}

func newResolver() testResolver {
	return testResolver{
		hosts: map[string]net.IP{
			"example.com": net.ParseIP("1.2.3.4"),
		},
	}
}
