package mqtt

import (
	"net"
	"net/url"
	"os"
	"testing"

	"github.com/grafana/sobek"
	"github.com/grafana/xk6-mqtt/internal/broker"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/lib/netext"
)

func TestClientConnect(t *testing.T) {
	t.Parallel()

	runtime := newTestRuntime(t)
	mm := newMqttMetrics(runtime.VU)
	logger := runtime.VU.InitEnv().Logger

	runtime.MoveToVUContext(newTestVUState(t)) // runtime.VU.InitEnv() will return nil after this

	client := newTestClient(t, logger, runtime.VU, mm)

	toValue := runtime.VU.Runtime().ToValue

	handlerCalled := false

	client.on("connect", func(_ sobek.Value, _ ...sobek.Value) (sobek.Value, error) {
		require.NoError(t, client.end(nil))

		handlerCalled = true

		return sobek.Undefined(), nil
	})

	err := runtime.EventLoop.Start(func() error {
		require.NoError(t, client.connect(toValue(os.Getenv(broker.EnvBrokerAddress)), nil)) //nolint:forbidigo // test reads the embedded broker address from env

		return nil
	})

	require.NoError(t, err)

	runtime.EventLoop.WaitOnRegistered()

	require.True(t, handlerCalled)
}

func TestClientConnectAuthenticated(t *testing.T) {
	t.Parallel()

	server := broker.New(false)

	t.Cleanup(func() {
		require.NoError(t, server.Close())
	})

	tcpListener, ok := server.Listeners.Get("tcp")

	require.True(t, ok)

	addr := "tcp://" + tcpListener.Address()

	runtime := newTestRuntime(t)
	mm := newMqttMetrics(runtime.VU)
	logger := runtime.VU.InitEnv().Logger

	runtime.MoveToVUContext(newTestVUState(t)) // runtime.VU.InitEnv() will return nil after this

	toValue := runtime.VU.Runtime().ToValue

	client := newTestClient(t, logger, runtime.VU, mm)
	client.clientOpts.Username = toValue("test-user")
	client.clientOpts.Password = toValue("test-password")

	handlerCalled := false

	client.on("connect", func(_ sobek.Value, _ ...sobek.Value) (sobek.Value, error) {
		require.NoError(t, client.end(nil))

		handlerCalled = true

		return sobek.Undefined(), nil
	})

	err := runtime.EventLoop.Start(func() error {
		require.NoError(t, client.connect(toValue(addr), nil))

		return nil
	})

	require.NoError(t, err)

	runtime.EventLoop.WaitOnRegistered()

	require.True(t, handlerCalled)
}

func TestClientConnectBlacklisted(t *testing.T) {
	t.Parallel()

	runtime := newTestRuntime(t)
	mm := newMqttMetrics(runtime.VU)
	logger := runtime.VU.InitEnv().Logger

	runtime.MoveToVUContext(newTestVUState(t))

	dialer, ok := runtime.VU.StateField.Dialer.(*netext.Dialer)
	require.True(t, ok)

	u, err := url.Parse(os.Getenv(broker.EnvBrokerAddress)) //nolint:forbidigo // test reads the embedded broker address from env
	require.NoError(t, err)

	dialer.Blacklist = []*lib.IPNet{
		{
			IPNet: net.IPNet{
				IP:   net.ParseIP(u.Hostname()),
				Mask: net.CIDRMask(24, 32),
			},
		},
	}

	client := newTestClient(t, logger, runtime.VU, mm)

	toValue := runtime.VU.Runtime().ToValue

	err = runtime.EventLoop.Start(func() error {
		return client.connect(toValue(os.Getenv(broker.EnvBrokerAddress)), nil) //nolint:forbidigo // test reads the embedded broker address from env
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "is in a blacklisted range")

	require.NoError(t, client.end(nil))

	runtime.EventLoop.WaitOnRegistered()
}

func TestClientReconnectWithoutConnect(t *testing.T) {
	t.Parallel()

	runtime := newTestRuntime(t)
	mm := newMqttMetrics(runtime.VU)
	logger := runtime.VU.InitEnv().Logger

	runtime.MoveToVUContext(newTestVUState(t))

	client := newTestClient(t, logger, runtime.VU, mm)

	err := runtime.EventLoop.Start(func() error {
		return client.reconnect()
	})

	require.Error(t, err)

	require.NoError(t, client.end(nil))

	runtime.EventLoop.WaitOnRegistered()
}
