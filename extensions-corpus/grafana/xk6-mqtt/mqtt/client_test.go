package mqtt

import (
	"io"
	"log/slog"
	"testing"

	extensionapi "go.k6.io/k6-extension-api"
)

func newTestClient(t *testing.T, _ any, vu extensionapi.VU, mm *mqttMetrics) *client {
	t.Helper()

	client := newClient(slog.New(slog.NewTextHandler(io.Discard, nil)), vu, mm)

	client.clientOpts = new(clientOptions)

	go client.loop()

	return client
}
