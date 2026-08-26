package mqtt

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"go.k6.io/k6/v2/metrics"
)

var errWrongNumberOfArgs = errors.New("wrong number of arguments")

func (c *client) currentTags() *metrics.TagSet {
	return c.vu.State().Tags.GetCurrentValues().Tags
}

func addToTagSet(ts *metrics.TagSet, tags map[string]string) *metrics.TagSet {
	if tags == nil {
		return ts
	}

	keys := make([]string, 0, len(tags))

	for k := range tags {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for _, k := range keys {
		ts = ts.With(k, tags[k])
	}

	return ts
}

func (c *client) tags() *metrics.TagSet {
	tags := c.currentTags()

	tags = tags.With("proto", "MQTT/3.1.1")

	if c.pahoClient != nil {
		opts := c.pahoClient.OptionsReader()

		if cid := opts.ClientID(); cid != "" {
			tags = tags.With("client_id", cid)
		}

		if c.url != "" {
			tags = tags.With("url", c.url)
		}
	}

	tags = addToTagSet(tags, c.clientOpts.Tags)
	tags = addToTagSet(tags, c.connOpts.Tags)

	return tags
}

func (c *client) tagsForMethod(method string, dict map[string]string, nv ...string) *metrics.TagSet {
	if len(nv)%2 != 0 {
		panic(fmt.Errorf("%w: expected even number of tags", errWrongNumberOfArgs))
	}

	tags := c.tags().With("method", method)
	tags = addToTagSet(tags, dict)

	for i := 0; i < len(nv)-1; i += 2 {
		tags = tags.With(nv[i], nv[i+1])
	}

	return tags
}

func (c *client) addErrorMetrics(method string, tags map[string]string, nv ...string) {
	metrics.PushIfNotDone(c.vu.Context(), c.vu.State().Samples, metrics.Samples{
		metrics.Sample{
			TimeSeries: metrics.TimeSeries{
				Metric: c.metrics.mqttErrors,
				Tags:   c.tagsForMethod(method, tags, nv...),
			},
			Time:  time.Now(),
			Value: float64(1),
		},
	})
}

func (c *client) addCallMetrics(method string, tags map[string]string, nv ...string) {
	c.log.Debug("Calling " + method)

	metrics.PushIfNotDone(c.vu.Context(), c.vu.State().Samples, metrics.Samples{
		metrics.Sample{
			TimeSeries: metrics.TimeSeries{
				Metric: c.metrics.mqttCalls,
				Tags:   c.tagsForMethod(method, tags, nv...),
			},
			Time:  time.Now(),
			Value: float64(1),
		},
	})
}
