package modules

import (
	"context"
	"net"
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/require"
	extensionapi "go.k6.io/k6-extension-api"
	"go.k6.io/k6/v2/js/common"
	"go.k6.io/k6/v2/lib"
)

type extensionAPITestVU struct {
	state *lib.State
}

func (vu extensionAPITestVU) Context() context.Context { return context.Background() }

func (extensionAPITestVU) Events() common.Events { return common.Events{} }

func (extensionAPITestVU) InitEnv() *common.InitEnvironment { return nil }

func (vu extensionAPITestVU) State() *lib.State { return vu.state }

func (extensionAPITestVU) Runtime() *sobek.Runtime { return sobek.New() }

func (extensionAPITestVU) RegisterCallback() func(func() error) {
	return func(func() error) {}
}

type extensionAPITestDialer struct{}

func (extensionAPITestDialer) DialContext(context.Context, string, string) (net.Conn, error) {
	connection, peer := net.Pipe()
	_ = peer.Close()
	return connection, nil
}

func (extensionAPITestDialer) ResolveAddr(string) (net.IP, int, error) {
	return net.ParseIP("192.0.2.1"), 0, nil
}

func TestExtensionAPIVUNetwork(t *testing.T) {
	t.Parallel()

	vu := extensionAPIVU{vu: extensionAPITestVU{state: &lib.State{Dialer: extensionAPITestDialer{}}}}
	network, ok := any(vu).(extensionapi.Network)
	require.True(t, ok)

	hosts, err := network.LookupHost(context.Background(), "example.test")
	require.NoError(t, err)
	require.Equal(t, []string{"192.0.2.1"}, hosts)

	connection, err := network.DialContext(context.Background(), "tcp", "example.test:443")
	require.NoError(t, err)
	require.NoError(t, connection.Close())
}

func TestExtensionAPIVUNetworkUnavailable(t *testing.T) {
	t.Parallel()

	vu := extensionAPIVU{vu: extensionAPITestVU{}}
	network := any(vu).(extensionapi.Network)

	_, err := network.LookupHost(context.Background(), "example.test")
	require.ErrorIs(t, err, extensionapi.ErrNetworkUnavailable)

	_, err = network.DialContext(context.Background(), "tcp", "example.test:443")
	require.ErrorIs(t, err, extensionapi.ErrNetworkUnavailable)
}
