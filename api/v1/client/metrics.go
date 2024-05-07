package client

import (
	"context"
	"net/http"
	"net/url"

	v1 "github.com/liuxd6825/k6server/api/v1"
)

// Metrics returns the current metrics summary.
func (c *Client) Metrics(ctx context.Context) (ret []v1.Metric, err error) {
	var resp v1.MetricsJSONAPI

	err = c.CallAPI(ctx, http.MethodGet, &url.URL{Path: "/v1/metrics"}, nil, &resp)
	if err != nil {
		return ret, err
	}

	return resp.Metrics(), nil
}
