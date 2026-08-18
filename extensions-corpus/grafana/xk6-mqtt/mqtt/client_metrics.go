package mqtt

import (
	"errors"
	"fmt"
	"time"

	extensionapi "go.k6.io/k6-extension-api"
)

var errWrongNumberOfArgs = errors.New("wrong number of arguments")

func (c *client) currentTags() extensionapi.Tags {
	return c.metrics.host.CurrentTags()
}

func addToTags(tags extensionapi.Tags, values map[string]string) extensionapi.Tags {
	return tags.With(values)
}

func (c *client) tags() extensionapi.Tags {
	tags := c.metrics.host.WithSystemTags(c.currentTags(), map[extensionapi.SystemTag]string{
		extensionapi.SystemTagProto: "MQTT/3.1.1",
	})

	if c.pahoClient != nil {
		opts := c.pahoClient.OptionsReader()
		if cid := opts.ClientID(); cid != "" {
			tags = tags.With(map[string]string{"client_id": cid})
		}
		if c.url != "" {
			tags = c.metrics.host.WithSystemTags(tags, map[extensionapi.SystemTag]string{
				extensionapi.SystemTagURL: c.url,
			})
		}
	}

	tags = addToTags(tags, c.clientOpts.Tags)
	return addToTags(tags, c.connOpts.Tags)
}

func (c *client) tagsForMethod(method string, values map[string]string, nv ...string) extensionapi.Tags {
	if len(nv)%2 != 0 {
		panic(fmt.Errorf("%w: expected even number of tags", errWrongNumberOfArgs))
	}
	tags := c.metrics.host.WithSystemTags(c.tags(), map[extensionapi.SystemTag]string{
		extensionapi.SystemTagMethod: method,
	})
	tags = addToTags(tags, values)
	for i := 0; i < len(nv)-1; i += 2 {
		tags = tags.With(map[string]string{nv[i]: nv[i+1]})
	}
	return tags
}

func (c *client) emit(samples ...extensionapi.Sample) {
	if err := c.metrics.host.Emit(c.vu.Context(), samples); err != nil && c.vu.Context().Err() == nil {
		c.log.Debug("MQTT metric emission failed", "error", err)
	}
}

func (c *client) addErrorMetrics(method string, tags map[string]string, nv ...string) {
	c.emit(extensionapi.Sample{
		Metric: c.metrics.mqttErrors,
		Time:   time.Now(),
		Value:  1,
		Tags:   c.tagsForMethod(method, tags, nv...),
	})
}

func (c *client) addCallMetrics(method string, tags map[string]string, nv ...string) {
	c.log.Debug("Calling MQTT method", "method", method)
	c.emit(extensionapi.Sample{
		Metric: c.metrics.mqttCalls,
		Time:   time.Now(),
		Value:  1,
		Tags:   c.tagsForMethod(method, tags, nv...),
	})
}
