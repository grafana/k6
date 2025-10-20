package browser

import (
	"fmt"

	"go.k6.io/k6/internal/js/modules/k6/browser/common"
)

// mapMetricEvent to the JS module.
func mapMetricEvent(vu moduleVU, event common.PageOnEvent) mapping {
	rt := vu.Runtime()
	em := event.Metric

	return mapping{
		"tag": func(urls common.TagMatches) error {
			callback := func(pattern, url string) (bool, error) {
				matched, err := rt.RunString(pattern + `.test('` + url + `')`)
				if err != nil {
					return false, fmt.Errorf("matching url with regex: %w", err)
				}
				return matched.ToBoolean(), nil
			}

			return em.Tag(callback, urls) //nolint:wrapcheck
		},
	}
}
