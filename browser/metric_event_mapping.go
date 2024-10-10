package browser

import (
	"fmt"

	"github.com/grafana/xk6-browser/common"
)

// mapMetricEvent to the JS module.
func mapMetricEvent(vu moduleVU, event common.PageOnEvent) mapping {
	rt := vu.VU.Runtime()

	return mapping{
		"tag": func(urls common.URLTagPatterns) error {
			callback := func(pattern, url string) (bool, error) {
				js := fmt.Sprintf(`_k6BrowserCheckRegEx(%s, '%s')`, pattern, url)

				matched, err := rt.RunString(js)
				if err != nil {
					return false, fmt.Errorf("matching url with regex: %w", err)
				}

				return matched.ToBoolean(), nil
			}

			return event.Metric.Tag(callback, urls) //nolint:wrapcheck
		},
	}
}
