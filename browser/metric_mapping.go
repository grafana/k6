package browser

import (
	"fmt"

	"github.com/grafana/xk6-browser/common"
)

// mapMetric to the JS module.
func mapMetric(vu moduleVU, cm *common.ExportedMetric) (mapping, error) {
	rt := vu.Runtime()

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
		"groupURLTag": func(urls common.URLGroups) error {
			return cm.GroupURLTag(urls)
		},
	}, nil
}
