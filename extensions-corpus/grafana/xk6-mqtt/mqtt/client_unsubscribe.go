package mqtt

import (
	"fmt"
	"reflect"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/grafana/sobek"
	"go.k6.io/k6/v2/js/promises"
)

type unsubscribeOptions struct {
	Tags map[string]string
}

func (c *client) unsubscribe(topic sobek.Value, opts *unsubscribeOptions) error {
	topics, opts, err := c.unsubscribePrepare(topic, opts)
	if err != nil {
		if e := c.handleError(err, "unsubscribe", opts.Tags, "topic", topic.String()); e != nil {
			return e
		}

		return nil
	}

	return c.unsubscribeExecute(topics, opts)
}

func (c *client) unsubscribeAsync(topic sobek.Value, opts *unsubscribeOptions) (*sobek.Promise, error) {
	topics, opts, err := c.unsubscribePrepare(topic, opts)
	if err != nil {
		return nil, err
	}

	promise, resolve, reject := promises.New(c.vu)

	go func() {
		if err := c.unsubscribeExecute(topics, opts); err != nil {
			reject(err)

			return
		}

		resolve(nil)
	}()

	return promise, nil
}

func (c *client) unsubscribePrepare(
	topic sobek.Value, opts *unsubscribeOptions,
) ([]string, *unsubscribeOptions, error) {
	if !c.isConnected() {
		return nil, nil, errNotConnected
	}

	if opts == nil {
		opts = new(unsubscribeOptions)
	}

	topics, err := asUnsubscribeTopics(topic, c.vu.Runtime())
	if err != nil {
		return nil, nil, err
	}

	return topics, opts, nil
}

func (c *client) unsubscribeExecute(topics []string, opts *unsubscribeOptions) error {
	c.log.Debug("Unsubscribing from MQTT topic(s)")

	tokens := make(map[string]paho.Token)

	for _, topic := range topics {
		c.log.WithField("topic", topic).Debug("Unsubscribing from topic")

		token := c.pahoClient.Unsubscribe(topic)

		tokens[topic] = token
	}

	for t, token := range tokens {
		if token.Wait() && token.Error() != nil {
			if err := c.handleError(token.Error(), "unsubscribe", opts.Tags, "topic", t); err != nil {
				return err
			}

			return nil
		}

		c.addCallMetrics("unsubscribe", opts.Tags, "topic", t)
	}

	return nil
}

func asUnsubscribeTopics(value sobek.Value, rt *sobek.Runtime) ([]string, error) {
	var topics []string

	switch value.ExportType() {
	case reflect.TypeFor[string]():
		var topic string
		if err := rt.ExportTo(value, &topic); err != nil {
			return nil, err
		}

		topics = append(topics, topic)

	case reflect.TypeFor[[]string]():
		var names []string

		if err := rt.ExportTo(value, &names); err != nil {
			return nil, err
		}

		topics = append(topics, names...)

	default:
		return nil, fmt.Errorf("%w: String or Array of String expected", errInvalidType)
	}

	return topics, nil
}
