/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package js

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/loadimpact/k6/stats"
	"github.com/stretchr/testify/assert"
)

func TestNewMetric(t *testing.T) {
	if testing.Short() {
		return
	}

	tpl := `
		import %s from "k6/metrics";
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
				"wrapper,direct": {
					fmt.Sprintf("{ %s }", s),
					s,
					"",
				},
				"wrapper,module": {
					"metrics",
					fmt.Sprintf("metrics.%s", s),
					"",
				},
				"const,direct": {
					fmt.Sprintf("{ Metric, %sType }", s),
					"Metric",
					fmt.Sprintf("%sType, ", s),
				},
				"const,module": {
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

func TestOpen(t *testing.T) {
	tmp := os.TempDir()
	path := filepath.Join(tmp, "k6_init_test.json")
	assert.NoError(t, ioutil.WriteFile(path, []byte(`{"a": 1}`), 0775))
	defer func() { _ = os.Remove(path) }()

	t.Run("existing", func(t *testing.T) {
		assert.NoError(t, runSnippet(fmt.Sprintf(`
		let data = open('%s');
		export default function() {
			if (data !== '{"a": 1}') { throw new Error(); }
		}
		`, path)))
	})

	t.Run("nonexistent", func(t *testing.T) {
		assert.EqualError(t, runSnippet(`
		// If you have a file called this, this test will fail.
		// I will also have several questions for you.
		let data = open('/dfghuibiuafeuieawfba.txt');
		export default function() {}
		`), "Error: open /dfghuibiuafeuieawfba.txt: no such file or directory")
	})

	t.Run("runtime prohibited", func(t *testing.T) {
		assert.EqualError(t, runSnippet(fmt.Sprintf(`
		export default function() {
			let data = open('%s');
		}
		`, path)), "Error: open() is only permitted during initialization")
	})
}
