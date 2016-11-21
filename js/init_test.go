package js

import (
	"fmt"
	"github.com/loadimpact/speedboat/stats"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewMetric(t *testing.T) {
	tpl := `
		import %s from "speedboat/metrics";
		let myMetric = new %s(%s"my_metric", %s);
		export default function() {};
	`

	types := map[string]stats.MetricType{
		"Counter": stats.Counter,
		"Gauge":   stats.Gauge,
		"Trend":   stats.Trend,
		"Rate":    stats.Rate,
	}

	for s, tp := range types {
		t.Run("t="+s, func(t *testing.T) {
			// name: [import, type, arg0]
			imports := map[string][]string{
				"wrapper,direct": []string{
					fmt.Sprintf("{ %s }", s),
					s,
					"",
				},
				"wrapper,module": []string{
					"metrics",
					fmt.Sprintf("metrics.%s", s),
					"",
				},
				"const,direct": []string{
					fmt.Sprintf("{ Metric, %sType }", s),
					"Metric",
					fmt.Sprintf("%sType, ", s),
				},
				"const,module": []string{
					"metrics",
					"metrics.Metric",
					fmt.Sprintf("metrics.%sType, ", s),
				},
			}

			for name, imp := range imports {
				t.Run("import="+name, func(t *testing.T) {
					isTimes := map[string]bool{
						"undefined": false,
						"false":     false,
						"true":      true,
					}

					for arg2, isTime := range isTimes {
						t.Run("isTime="+arg2, func(t *testing.T) {
							vt := stats.Default
							if isTime {
								vt = stats.Time
							}

							src := fmt.Sprintf(tpl, imp[0], imp[1], imp[2], arg2)
							r, err := newSnippetRunner(src)
							if !assert.NoError(t, err) {
								t.Log(src)
								return
							}

							assert.Contains(t, r.Runtime.Metrics, "my_metric")
							m := r.Runtime.Metrics["my_metric"]
							assert.Equal(t, tp, m.Type, "wrong metric type")
							assert.Equal(t, vt, m.Contains, "wrong value type")
						})
					}
				})
			}
		})
	}
}
