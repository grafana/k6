package mqtt

import (
	"fmt"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/grafana/sobek"
	extensionapi "go.k6.io/k6-extension-api"
)

var events = map[string]struct{}{ //nolint:gochecknoglobals
	"connect":   {},
	"reconnect": {},
	"end":       {},
	"error":     {},
	"message":   {},
}

func (c *client) on(event string, handler sobek.Callable) {
	if _, ok := events[event]; !ok {
		c.log.Warn("Unknown event type", "event", event)

		return
	}

	if _, ok := c.handlers.Load(event); ok {
		c.log.Warn("Event handler already registered, overriding", "event", event)
	}

	c.log.Debug("Event handler registered", "event", event)

	c.handlers.Store(event, handler)
}

func (c *client) fire(event string, args ...any) bool {
	f, ok := c.handlers.Load(event)
	if !ok {
		return false
	}

	fn, ok := f.(sobek.Callable)
	if !ok {
		return false
	}

	c.log.Debug("Queuing event handler", "event", event)

	call := func() error {
		c.log.Debug("Firing event handler", "event", event)
		values := make([]sobek.Value, len(args))
		for index, arg := range args {
			values[index] = c.vu.Runtime().ToValue(arg)
		}
		_, err := fn(sobek.Undefined(), values...)

		return err
	}

	select {
	case c.callChan <- call:
		return true
	case <-c.stop:
		return false
	}
}

func (c *client) messageHandler(_ paho.Client, msg paho.Message) {
	c.log.Debug("Received MQTT message", "topic", msg.Topic(), "message_id", msg.MessageID())

	now := time.Now()
	bytes := float64(len(msg.Payload()))
	tags := c.tags().With(map[string]string{"topic": msg.Topic()})
	c.emit(
		extensionapi.Sample{Metric: c.metrics.mqttCalls, Time: now, Value: 1,
			Tags: c.tagsForMethod("message", nil, "topic", msg.Topic())},
		extensionapi.Sample{Metric: c.metrics.mqttMessagesReceived, Time: now, Value: 1, Tags: tags},
		extensionapi.Sample{Metric: c.metrics.dataReceived, Time: now, Value: bytes, Tags: c.currentTags()},
	)
	c.fire("message", msg.Topic(), append([]byte(nil), msg.Payload()...))
}

func (c *client) connectHandler(_ paho.Client) {
	c.log.Debug("Connected to MQTT broker")

	c.fire("connect")
}

func (c *client) reconnectHandler(_ paho.Client, _ *paho.ClientOptions) {
	c.log.Debug("Reconnecting to MQTT broker")

	c.fire("reconnect")
}

func (c *client) handleError(err error, method string, tags map[string]string, nv ...string) error {
	c.log.Error("MQTT error occurred", "error", err, "method", method)

	c.addErrorMetrics(method, tags, nv...)

	wrapped := newMQTTError(err, method)

	if c.fire("error", wrapped) {
		return nil
	}

	return wrapped
}

// MQTTError represents an error that occurred during an MQTT operation.
type MQTTError struct { //nolint:revive
	Name    string
	Method  string
	Message string
}

func newMQTTError(err error, method string) *MQTTError {
	return &MQTTError{
		Name:    "MQTTError",
		Method:  method,
		Message: err.Error(),
	}
}

func (e *MQTTError) Error() string {
	return fmt.Sprintf("MQTT error during %s: %v", e.Method, e.Message)
}
