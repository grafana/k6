package mqtt

import (
	"context"
	"crypto/tls"
	"net"
	"os"
	"testing"

	"github.com/grafana/xk6-mqtt/internal/broker"
	"github.com/stretchr/testify/require"
	extensionapi "go.k6.io/k6-extension-api"
	extensionapitest "go.k6.io/k6-extension-api/test"
)

func Test_module(t *testing.T) {
	runtime := newTestRuntime(t)
	exports := New().NewModuleInstance(runtime.VU).Exports()
	require.Nil(t, exports.Default)
	require.Contains(t, exports.Named, "Client")
}

func newTestRuntime(t *testing.T) *extensionapitest.Runtime {
	t.Helper()
	runtime := extensionapitest.NewRuntime()
	runtime.VU.RegisterBuiltinMetric(extensionapi.BuiltinDataSent, "data_sent")
	runtime.VU.RegisterBuiltinMetric(extensionapi.BuiltinDataReceived, "data_received")
	runtime.VU.LookupEnvFunc = func(key string) (string, bool) {
		if key == broker.EnvBrokerAddress {
			return os.Getenv(key), true //nolint:forbidigo // test reads the embedded broker address
		}
		return "", false
	}
	dialer := &net.Dialer{}
	runtime.VU.LookupHostFunc = func(ctx context.Context, host string) ([]string, error) {
		return net.DefaultResolver.LookupHost(ctx, host)
	}
	runtime.VU.DialContextFunc = dialer.DialContext
	runtime.VU.TLSClientFunc = func(ctx context.Context, conn net.Conn, config *tls.Config) (net.Conn, error) {
		tlsConn := tls.Client(conn, config)
		return tlsConn, tlsConn.HandshakeContext(ctx)
	}
	return runtime
}

func newTestMetrics(vu extensionapi.VU) *mqttMetrics { return newMqttMetrics(vu) }
