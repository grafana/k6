package netext

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func makeTestDialer() *Dialer {
	return NewDialer(net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 60 * time.Second,
		DualStack: true,
	})
}

func TestLookup(t *testing.T) {
	t.Run("never resolved", func(t *testing.T) {
		dialer := makeTestDialer()
		require.False(t, dialer.IP4["example.com."])
	})

	t.Run("resolution failure", func(t *testing.T) {
		dialer := makeTestDialer()
		_, _, err := dialer.lookup("example.badtld.")
		require.Error(t, err)
		require.False(t, dialer.IP4["example.badtld."])
	})

	t.Run("find ipv6", func(t *testing.T) {
		dialer := makeTestDialer()
		ip, _, err := dialer.lookup("example.com.")
		require.NoError(t, err)
		require.True(t, ip.To4() == nil)
		require.False(t, dialer.IP4["example.com."])
	})

	t.Run("find ipv4", func(t *testing.T) {
		dialer := makeTestDialer()
		ip, _, err := dialer.lookup("httpbin.org.")
		require.NoError(t, err)
		require.True(t, ip.To4() != nil)
		require.True(t, dialer.IP4["httpbin.org."])
	})
}

func TestResolution(t *testing.T) {
	t.Run("follow CNAMEs", func(t *testing.T) {
		dialer := makeTestDialer()
		ip, err := dialer.resolve("http2.akamai.com", 3)
		require.NoError(t, err)
		require.True(t, ip.To4() == nil)
		cname := dialer.CNAME["http2.akamai.com."]
		require.NotEqual(t, cname, nil)
		require.Equal(t, cname.Name, "http2.edgekey.net.")
	})
}
