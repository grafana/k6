package browser

import (
	"github.com/grafana/xk6-browser/common"
)

// mapMetric to the JS module.
func mapMetric(vu moduleVU, cm *common.ExportedMetric) (mapping, error) {
	return mapping{
		"groupURLTag": func(urls common.URLGroups) error {
			return cm.GroupURLTag(urls)
		},
	}, nil
}
