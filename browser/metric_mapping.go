package browser

import (
	"fmt"

	"github.com/grafana/xk6-browser/common"
)

// mapMetric to the JS module.
func mapMetric(vu moduleVU, cm *common.ExportedMetric) (mapping, error) {
	rt := vu.VU.Runtime()

	// We're setting up the function in the Sobek context that will be reused
	// for this VU.
	_, err := rt.RunString(`
	function _k6BrowserURLGroupingTest(pattern, url) {
		let r = pattern;
		if (typeof pattern === 'string') {
			r = new RegExp(pattern);
		}
		return r.test(url);
	}`)
	if err != nil {
		return nil, fmt.Errorf("evaluating url grouping: %w", err)
	}

	return mapping{
		"Tag": func(urls common.URLGroups) error {
			callback := func(pattern, url string) (bool, error) {
				js := fmt.Sprintf(`_k6BrowserURLGroupingTest(%s, '%s')`, pattern, url)

				val, err := rt.RunString(js)
				if err != nil {
					return false, fmt.Errorf("evaluating metric tag url grouping: %w", err)
				}

				return val.ToBoolean(), nil
			}

			return cm.Tag(callback, urls)
		},
	}, nil
}
