package mqtt

import (
	"testing"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/grafana/sobek"
	"github.com/stretchr/testify/require"
)

func Test_clientOptions_toPaho(t *testing.T) {
	t.Parallel()

	pahoOpts := paho.NewClientOptions()

	runtime := sobek.New()

	co := &clientOptions{
		ClientId: runtime.ToValue("test-client"),
		Username: runtime.ToValue("test-user"),
		Password: runtime.ToValue("test-pass"),
		Will: &will{
			Topic:   "test/topic",
			Payload: "test payload",
			Qos:     1,
			Retain:  true,
		},
	}

	co.toPaho(pahoOpts, runtime)

	require.Equal(t, "test-client", pahoOpts.ClientID)
	require.Equal(t, "test-user", pahoOpts.Username)
	require.Equal(t, "test-pass", pahoOpts.Password)
	require.Equal(t, "test/topic", pahoOpts.WillTopic)
	require.Equal(t, "test payload", string(pahoOpts.WillPayload))
	require.Equal(t, byte(1), pahoOpts.WillQos)
	require.True(t, pahoOpts.WillRetained)
}

func Test_clientOptions_toPaho_credentialsProvider(t *testing.T) {
	t.Parallel()

	pahoOpts := paho.NewClientOptions()
	runtime := newTestRuntime(t).VU.RuntimeField

	_, err := runtime.RunString(`
    function provider() {
        return {
            username: "test-user",
            password: "test-pass"
        };
    }
    `)

	require.NoError(t, err)

	fn, ok := sobek.AssertFunction(runtime.Get("provider"))
	require.True(t, ok)

	co := &clientOptions{
		CredentialsProvider: fn,
	}

	co.toPaho(pahoOpts, runtime)

	username, password := pahoOpts.CredentialsProvider()

	require.Equal(t, "test-user", username)
	require.Equal(t, "test-pass", password)
}
