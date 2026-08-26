package mqtt

import (
	"go.k6.io/k6/v2/js/modules"
	"go.k6.io/k6/v2/metrics"
)

const (
	mqttMessagesSent     = "mqtt_messages_sent"
	mqttMessagesReceived = "mqtt_messages_received"
	mqttErrors           = "mqtt_errors"
	mqttCalls            = "mqtt_calls"
)

type mqttMetrics struct {
	dataSent             *metrics.Metric
	dataReceived         *metrics.Metric
	mqttMessagesSent     *metrics.Metric
	mqttMessagesReceived *metrics.Metric
	mqttErrors           *metrics.Metric
	mqttCalls            *metrics.Metric
}

func newMqttMetrics(vu modules.VU) *mqttMetrics {
	return &mqttMetrics{
		dataSent:             vu.InitEnv().BuiltinMetrics.DataSent,
		dataReceived:         vu.InitEnv().BuiltinMetrics.DataReceived,
		mqttMessagesSent:     vu.InitEnv().Registry.MustNewMetric(mqttMessagesSent, metrics.Counter),
		mqttMessagesReceived: vu.InitEnv().Registry.MustNewMetric(mqttMessagesReceived, metrics.Counter),
		mqttErrors:           vu.InitEnv().Registry.MustNewMetric(mqttErrors, metrics.Counter),
		mqttCalls:            vu.InitEnv().Registry.MustNewMetric(mqttCalls, metrics.Counter),
	}
}
