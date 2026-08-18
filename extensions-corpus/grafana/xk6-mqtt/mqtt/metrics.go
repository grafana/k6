package mqtt

import (
	"fmt"

	extensionapi "go.k6.io/k6-extension-api"
)

const (
	mqttMessagesSent     = "mqtt_messages_sent"
	mqttMessagesReceived = "mqtt_messages_received"
	mqttErrors           = "mqtt_errors"
	mqttCalls            = "mqtt_calls"
)

type mqttMetrics struct {
	host                 extensionapi.Metrics
	dataSent             extensionapi.Metric
	dataReceived         extensionapi.Metric
	mqttMessagesSent     extensionapi.Metric
	mqttMessagesReceived extensionapi.Metric
	mqttErrors           extensionapi.Metric
	mqttCalls            extensionapi.Metric
}

func newMqttMetrics(vu extensionapi.VU) *mqttMetrics {
	host, ok := vu.(extensionapi.Metrics)
	if !ok {
		panic("extension API metrics capability is unavailable")
	}
	register := func(name string) extensionapi.Metric {
		metric, err := host.RegisterMetric(extensionapi.MetricSpec{Name: name, Kind: extensionapi.MetricCounter})
		if err != nil {
			panic(fmt.Errorf("register MQTT metric %q: %w", name, err))
		}
		return metric
	}
	dataSent, _ := host.BuiltinMetric(extensionapi.BuiltinDataSent)
	dataReceived, _ := host.BuiltinMetric(extensionapi.BuiltinDataReceived)
	return &mqttMetrics{host: host, dataSent: dataSent, dataReceived: dataReceived,
		mqttMessagesSent: register(mqttMessagesSent), mqttMessagesReceived: register(mqttMessagesReceived),
		mqttErrors: register(mqttErrors), mqttCalls: register(mqttCalls)}
}
