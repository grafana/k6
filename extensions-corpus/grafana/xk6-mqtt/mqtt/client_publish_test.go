package mqtt

import (
	"os"
	"testing"

	"github.com/grafana/sobek"
	"github.com/grafana/xk6-mqtt/internal/broker"
	"github.com/stretchr/testify/require"
)

func TestClientPublish(t *testing.T) {
	t.Parallel()

	runtime := newTestRuntime(t)
	mm := newMqttMetrics(runtime.VU)
	logger := runtime.VU.InitEnv().Logger

	runtime.MoveToVUContext(newTestVUState(t))

	client := newTestClient(t, logger, runtime.VU, mm)

	toValue := runtime.VU.Runtime().ToValue

	client.on("connect", func(_ sobek.Value, _ ...sobek.Value) (sobek.Value, error) {
		require.NoError(t, client.end(nil))

		return sobek.Undefined(), nil
	})

	err := runtime.EventLoop.Start(func() error {
		require.NoError(t, client.connect(toValue(os.Getenv(broker.EnvBrokerAddress)), nil)) //nolint:forbidigo // test reads the embedded broker address from env

		return nil
	})

	require.NoError(t, err)

	runtime.EventLoop.WaitOnRegistered()
}
