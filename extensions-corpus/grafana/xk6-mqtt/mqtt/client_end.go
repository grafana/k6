package mqtt

import (
	"github.com/grafana/sobek"
)

type endOptions struct {
	Tags map[string]string
}

func (c *client) end(opts *endOptions) error {
	if opts == nil {
		opts = new(endOptions)
	}

	c.log.Debug("Disconnecting from MQTT broker")

	c.fire("end")

	c.addCallMetrics("end", opts.Tags)

	c.disconnect()
	c.stopLoop()

	return nil
}

func (c *client) endAsync(opts *endOptions) (*sobek.Promise, error) {
	promise, resolver := newPromise(c.vu)

	go func() {
		if err := c.end(opts); err != nil {
			resolver.Reject(err)

			return
		}

		resolver.Resolve(nil)
	}()

	return promise, nil
}
