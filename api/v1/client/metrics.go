package client

import (
	"context"
	"net/url"

	"github.com/loadimpact/k6/api/v1"
)

var MetricsURL = &url.URL{Path: "/v1/metrics"}

func (c *Client) Metrics(ctx context.Context) (ret []v1.Metric, err error) {
	return ret, c.call(ctx, "GET", MetricsURL, nil, &ret)
}
