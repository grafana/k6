package mqtt

import (
	"testing"

	"github.com/sirupsen/logrus"
	"go.k6.io/k6/v2/js/modules"
)

func newTestClient(t *testing.T, logger logrus.FieldLogger, vu modules.VU, mm *mqttMetrics) *client {
	t.Helper()

	client := newClient(logger, vu, mm)

	client.clientOpts = new(clientOptions)

	go client.loop()

	return client
}
