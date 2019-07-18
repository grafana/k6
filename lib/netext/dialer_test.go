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

func TestResolution(t *testing.T) {
	t.Run("never resolved", func(t *testing.T) {
		dialer := makeTestDialer()
		require.False(t, dialer.IP4["example.com"])
	})

	t.Run("resolution failure", func(t *testing.T) {
		dialer := makeTestDialer()
		_, err := dialer.fetch("example.badtld")
		require.Error(t, err)
		require.False(t, dialer.IP4["example.badtld"])
	})

	t.Run("resolve ipv6", func(t *testing.T) {
		dialer := makeTestDialer()
		ip, err := dialer.fetch("example.com")
		require.NoError(t, err)
		require.True(t, ip.To4() == nil)
		require.False(t, dialer.IP4["example.com"])
	})

	t.Run("resolve ipv4", func(t *testing.T) {
		dialer := makeTestDialer()
		ip, err := dialer.fetch("httpbin.org")
		require.NoError(t, err)
		require.True(t, ip.To4() != nil)
		require.True(t, dialer.IP4["httpbin.org"])
	})
}
