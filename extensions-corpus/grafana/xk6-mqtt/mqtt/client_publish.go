package mqtt

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/grafana/sobek"
	"go.k6.io/k6/v2/js/promises"
	"go.k6.io/k6/v2/metrics"
)

var errInvalidType = errors.New("invalid type")

type publishOptions struct {
	Qos    byte
	Retain bool
	Tags   map[string]string
}

func (c *client) publish(topic string, message sobek.Value, opts *publishOptions) error {
	topic, data, opts, err := c.publishPrepare(topic, message, opts)
	if err != nil {
		if err := c.handleError(err, "publish", opts.Tags, "topic", topic); err != nil {
			return err
		}

		return nil
	}

	return c.publishExecute(topic, data, opts)
}

func (c *client) publishAsync(topic string, message sobek.Value, opts *publishOptions) (*sobek.Promise, error) {
	topic, data, opts, err := c.publishPrepare(topic, message, opts)
	if err != nil {
		return nil, err
	}

	promise, resolve, reject := promises.New(c.vu)

	go func() {
		err := c.publishExecute(topic, data, opts)
		if err != nil {
			reject(err)

			return
		}

		resolve(sobek.Undefined())
	}()

	return promise, nil
}

func (c *client) publishPrepare(
	topic string, message sobek.Value, opts *publishOptions,
) (string, []byte, *publishOptions, error) {
	if !c.isConnected() {
		return "", nil, nil, errNotConnected
	}

	data, err := stringOrArrayBuffer(message, c.vu.Runtime())
	if err != nil {
		return "", nil, nil, err
	}

	if opts == nil {
		opts = &publishOptions{}
	}

	return topic, data, opts, nil
}

func (c *client) publishExecute(topic string, message []byte, opts *publishOptions) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	c.log.Debug("Publishing message to MQTT broker")

	token := c.pahoClient.Publish(topic, opts.Qos, opts.Retain, message)
	if token.Wait() && token.Error() != nil {
		if err := c.handleError(token.Error(), "publish", opts.Tags, "topic", topic); err != nil {
			return err
		}

		return nil
	}

	now := time.Now()
	bytes := float64(len(message))
	tags := c.tags().With("topic", topic)

	samples := metrics.Samples{
		metrics.Sample{
			TimeSeries: metrics.TimeSeries{
				Metric: c.metrics.mqttCalls,
				Tags:   c.tagsForMethod("publish", opts.Tags, "topic", topic),
			},
			Time:  now,
			Value: float64(1),
		},
		metrics.Sample{
			TimeSeries: metrics.TimeSeries{
				Metric: c.metrics.mqttMessagesSent,
				Tags:   tags,
			},
			Time:  now,
			Value: float64(1),
		},
		metrics.Sample{
			TimeSeries: metrics.TimeSeries{
				Metric: c.metrics.dataSent,
				Tags:   c.currentTags(),
			},
			Time:  now,
			Value: bytes,
		},
	}

	metrics.PushIfNotDone(c.vu.Context(), c.vu.State().Samples, samples)

	return nil
}

func stringOrArrayBuffer(input sobek.Value, runtime *sobek.Runtime) ([]byte, error) {
	var data []byte

	switch input.ExportType() {
	case reflect.TypeFor[string]():
		var str string

		if err := runtime.ExportTo(input, &str); err != nil {
			return nil, err
		}

		data = []byte(str)

	case reflect.TypeFor[[]byte]():
		if err := runtime.ExportTo(input, &data); err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("%w: String or ArrayBuffer expected", errInvalidType)
	}

	return data, nil
}
