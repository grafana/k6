package netext

import (
	"context"
	"crypto/tls"
	"net"
	"testing"

	"github.com/quic-go/quic-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/types"
)

func TestQUICDialBlacklistedIP(t *testing.T) {
	t.Parallel()
	dialer := NewDialer(net.Dialer{}, newResolver())
	ipNet, err := lib.ParseCIDR("8.9.10.0/24")
	require.NoError(t, err)
	dialer.Blacklist = []*lib.IPNet{ipNet}

	_, err = dialer.QUICDial(
		context.Background(),
		"example-deny-resolver.com:443",
		&tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		&quic.Config{},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "blacklisted")
}

func TestQUICDialBlockedHostname(t *testing.T) {
	t.Parallel()
	dialer := NewDialer(net.Dialer{}, newResolver())
	trie, err := types.NewHostnameTrie([]string{"*.blocked.com"})
	require.NoError(t, err)
	dialer.BlockedHostnames = trie

	_, err = dialer.QUICDial(
		context.Background(),
		"test.blocked.com:443",
		&tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		&quic.Config{},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "blocked")
}

func TestQUICDialResolvesAddress(t *testing.T) {
	t.Parallel()
	dialer := NewDialer(net.Dialer{}, newResolver())

	// This should resolve via the mock resolver but will fail to actually
	// connect since there's no QUIC server - we just verify it gets past
	// the resolution stage.
	_, err := dialer.QUICDial(
		context.Background(),
		"example-resolver.com:443",
		&tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		&quic.Config{},
	)
	// The error should be a connection error (not a DNS or blacklist error)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "blacklisted")
	assert.NotContains(t, err.Error(), "blocked")
	assert.NotContains(t, err.Error(), "no such host")
}
