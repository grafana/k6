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

package metrics

import (
	"context"
	"fmt"
	"testing"

	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js2/common"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/stats"
	"github.com/stretchr/testify/assert"
)

func TestMetrics(t *testing.T) {
	types := map[string]stats.MetricType{
		"Counter": stats.Counter,
		"Gauge":   stats.Gauge,
		"Trend":   stats.Trend,
		"Rate":    stats.Rate,
	}
	values := map[string]struct {
		JS    string
		Float float64
	}{
		"Float": {`2.5`, 2.5},
		"Int":   {`5`, 5.0},
		"True":  {`true`, 1.0},
		"False": {`false`, 0.0},
	}
	for fn, mtyp := range types {
		t.Run(fn, func(t *testing.T) {
			for isTime, valueType := range map[bool]stats.ValueType{false: stats.Default, true: stats.Time} {
				t.Run(fmt.Sprintf("isTime=%v", isTime), func(t *testing.T) {
					rt := goja.New()
					rt.SetFieldNameMapper(common.FieldNameMapper{})

					ctxPtr := new(context.Context)
					*ctxPtr = common.WithRuntime(context.Background(), rt)
					rt.Set("metrics", common.Bind(rt, &Metrics{}, ctxPtr))

					root, _ := lib.NewGroup("", nil)
					child, _ := root.Group("child")
					state := &common.State{Group: root}

					isTimeString := ""
					if isTime {
						isTimeString = `, true`
					}
					_, err := common.RunString(rt,
						fmt.Sprintf(`let m = new metrics.%s("my_metric"%s)`, fn, isTimeString),
					)
					if !assert.NoError(t, err) {
						return
					}

					t.Run("ExitInit", func(t *testing.T) {
						*ctxPtr = common.WithState(*ctxPtr, state)
						_, err := common.RunString(rt, fmt.Sprintf(`new metrics.%s("my_metric")`, fn))
						assert.EqualError(t, err, "GoError: Metrics must be declared in the init context at apply (native)")
					})

					groups := map[string]*lib.Group{
						"Root":  root,
						"Child": child,
					}
					for name, g := range groups {
						t.Run(name, func(t *testing.T) {
							state.Group = g
							for name, val := range values {
								t.Run(name, func(t *testing.T) {
									t.Run("Simple", func(t *testing.T) {
										state.Samples = nil
										_, err := common.RunString(rt, fmt.Sprintf(`m.add(%v)`, val.JS))
										assert.NoError(t, err)
										if assert.Len(t, state.Samples, 1) {
											assert.NotZero(t, state.Samples[0].Time)
											assert.Equal(t, state.Samples[0].Value, val.Float)
											assert.Equal(t, map[string]string{
												"group": g.Path,
											}, state.Samples[0].Tags)
											assert.Equal(t, "my_metric", state.Samples[0].Metric.Name)
											assert.Equal(t, mtyp, state.Samples[0].Metric.Type)
											assert.Equal(t, valueType, state.Samples[0].Metric.Contains)
										}
									})
									t.Run("Tags", func(t *testing.T) {
										state.Samples = nil
										_, err := common.RunString(rt, fmt.Sprintf(`m.add(%v, {a:1})`, val.JS))
										assert.NoError(t, err)
										if assert.Len(t, state.Samples, 1) {
											assert.NotZero(t, state.Samples[0].Time)
											assert.Equal(t, state.Samples[0].Value, val.Float)
											assert.Equal(t, map[string]string{
												"group": g.Path,
												"a":     "1",
											}, state.Samples[0].Tags)
											assert.Equal(t, "my_metric", state.Samples[0].Metric.Name)
											assert.Equal(t, mtyp, state.Samples[0].Metric.Type)
											assert.Equal(t, valueType, state.Samples[0].Metric.Contains)
										}
									})
								})
							}
						})
					}
				})
			}
		})
	}
}
