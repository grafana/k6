package mqtt

import (
	"fmt"
	"reflect"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/grafana/sobek"
)

type subscribeOptions struct {
	Qos  byte
	Tags map[string]string
}

func (c *client) subscribe(topic sobek.Value, opts *subscribeOptions) error {
	topics, o, err := c.subscribePrepare(topic, opts)
	if err != nil {
		if e := c.handleError(err, "subscribe", o.Tags, "topic", topic.String()); e != nil {
			return e
		}

		return nil
	}

	return c.subscribeExecute(topics, o)
}

func (c *client) subscribeAsync(topic sobek.Value, opts *subscribeOptions) (*sobek.Promise, error) {
	topics, o, err := c.subscribePrepare(topic, opts)
	if err != nil {
		return nil, err
	}

	promise, resolver := newPromise(c.vu)

	go func() {
		err := c.subscribeExecute(topics, o)
		if err != nil {
			resolver.Reject(err)

			return
		}

		resolver.Resolve(sobek.Undefined())
	}()

	return promise, nil
}

func (c *client) subscribePrepare(
	topic sobek.Value, opts *subscribeOptions,
) (map[string]byte, *subscribeOptions, error) {
	if !c.isConnected() {
		return nil, nil, errNotConnected
	}

	var qos byte

	if opts != nil {
		qos = opts.Qos
	} else {
		opts = new(subscribeOptions)
	}

	topics, err := asSubscribeTopics(topic, qos, c.vu.Runtime())
	if err != nil {
		return nil, nil, err
	}

	return topics, opts, nil
}

func (c *client) subscribeExecute(topics map[string]byte, opts *subscribeOptions) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	c.log.Debug("Subscribing to MQTT topic(s)")

	tokens := make(map[string]paho.Token)

	for t, qos := range topics {
		c.log.Debug("Subscribing to topic", "topic", t, "qos", qos)

		token := c.pahoClient.Subscribe(t, qos, nil)

		tokens[t] = token
	}

	for t, token := range tokens {
		if token.Wait() && token.Error() != nil {
			if err := c.handleError(token.Error(), "subscribe", opts.Tags, "topic", t); err != nil {
				return err
			}

			return nil
		}

		c.addCallMetrics("subscribe", opts.Tags, "topic", t)
	}

	return nil
}

func asSubscribeTopics(value sobek.Value, qos byte, rt *sobek.Runtime) (map[string]byte, error) {
	topics := make(map[string]byte)

	switch value.ExportType() {
	case reflect.TypeFor[string]():
		var topic string
		if err := rt.ExportTo(value, &topic); err != nil {
			return nil, err
		}

		topics[topic] = qos

	case reflect.TypeFor[[]string]():
		var names []string

		if err := rt.ExportTo(value, &names); err != nil {
			return nil, err
		}

		for _, topic := range names {
			topics[topic] = qos
		}

	case reflect.TypeFor[map[string]subscribeOptions]():
		var options map[string]subscribeOptions

		if err := rt.ExportTo(value, &options); err != nil {
			return nil, err
		}

		for topic, opts := range options {
			topics[topic] = opts.Qos
		}

	default:
		return nil, fmt.Errorf("%w: String or Array of String or Object expected", errInvalidType)
	}

	return topics, nil
}
