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
	"io/ioutil"
	"testing"

	"github.com/dop251/goja"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modulestest"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/metrics"
	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/stats"
)

type addTestValue struct {
	JS      string
	Float   float64
	isError bool
}

type addTest struct {
	val          addTestValue
	rt           *goja.Runtime
	hook         *testutils.SimpleLogrusHook
	samples      chan stats.SampleContainer
	isThrow      bool
	mtyp         stats.MetricType
	valueType    stats.ValueType
	js           string
	expectedTags map[string]string
}

func (a addTest) run(t *testing.T) {
	_, err := a.rt.RunString(a.js)
	if a.val.isError && a.isThrow {
		if assert.Error(t, err) {
			return
		}
	} else {
		assert.NoError(t, err)
		if a.val.isError && !a.isThrow {
			lines := a.hook.Drain()
			require.Len(t, lines, 1)
			assert.Contains(t, lines[0].Message, "is an invalid value for metric")
			return
		}
	}
	bufSamples := stats.GetBufferedSamples(a.samples)
	if assert.Len(t, bufSamples, 1) {
		sample, ok := bufSamples[0].(stats.Sample)
		require.True(t, ok)

		assert.NotZero(t, sample.Time)
		assert.Equal(t, a.val.Float, sample.Value)
		assert.Equal(t, a.expectedTags, sample.Tags.CloneTags())
		assert.Equal(t, "my_metric", sample.Metric.Name)
		assert.Equal(t, a.mtyp, sample.Metric.Type)
		assert.Equal(t, a.valueType, sample.Metric.Contains)
	}
}

func TestMetrics(t *testing.T) {
	t.Parallel()
	types := map[string]stats.MetricType{
		"Counter": stats.Counter,
		"Gauge":   stats.Gauge,
		"Trend":   stats.Trend,
		"Rate":    stats.Rate,
	}
	values := map[string]addTestValue{
		"Float":                 {JS: `2.5`, Float: 2.5},
		"Int":                   {JS: `5`, Float: 5.0},
		"True":                  {JS: `true`, Float: 1.0},
		"False":                 {JS: `false`, Float: 0.0},
		"null":                  {JS: `null`, isError: true},
		"undefined":             {JS: `undefined`, isError: true},
		"NaN":                   {JS: `NaN`, isError: true},
		"string":                {JS: `"string"`, isError: true},
		"string 5":              {JS: `"5.3"`, Float: 5.3},
		"some object":           {JS: `{something: 3}`, isError: true},
		"another metric object": {JS: `m`, isError: true},
	}
	for fn, mtyp := range types {
		fn, mtyp := fn, mtyp
		t.Run(fn, func(t *testing.T) {
			t.Parallel()
			for isTime, valueType := range map[bool]stats.ValueType{false: stats.Default, true: stats.Time} {
				isTime, valueType := isTime, valueType
				t.Run(fmt.Sprintf("isTime=%v", isTime), func(t *testing.T) {
					t.Parallel()
					test := addTest{
						mtyp:      mtyp,
						valueType: valueType,
					}
					test.rt = goja.New()
					test.rt.SetFieldNameMapper(common.FieldNameMapper{})
					mii := &modulestest.InstanceCore{
						Runtime: test.rt,
						InitEnv: &common.InitEnvironment{Registry: metrics.NewRegistry()},
						Ctx:     context.Background(),
					}
					m, ok := New().NewModuleInstance(mii).(*ModuleInstance)
					require.True(t, ok)
					require.NoError(t, test.rt.Set("metrics", m.GetExports().Named))
					test.samples = make(chan stats.SampleContainer, 1000)
					state := &lib.State{
						Options: lib.Options{},
						Samples: test.samples,
						Tags: lib.NewTagMap(map[string]string{
							"key": "value",
						}),
					}

					isTimeString := ""
					if isTime {
						isTimeString = `, true`
					}
					_, err := test.rt.RunString(fmt.Sprintf(`var m = new metrics.%s("my_metric"%s)`, fn, isTimeString))
					require.NoError(t, err)

					t.Run("ExitInit", func(t *testing.T) {
						mii.State = state
						mii.InitEnv = nil
						_, err := test.rt.RunString(fmt.Sprintf(`new metrics.%s("my_metric")`, fn))
						assert.Contains(t, err.Error(), "metrics must be declared in the init context")
					})
					mii.State = state
					logger := logrus.New()
					logger.Out = ioutil.Discard
					test.hook = &testutils.SimpleLogrusHook{HookedLevels: logrus.AllLevels}
					logger.AddHook(test.hook)
					state.Logger = logger

					for name, val := range values {
						test.val = val
						for _, isThrow := range []bool{false, true} {
							state.Options.Throw.Bool = isThrow
							test.isThrow = isThrow
							t.Run(fmt.Sprintf("%s/isThrow=%v/Simple", name, isThrow), func(t *testing.T) {
								test.js = fmt.Sprintf(`m.add(%v)`, val.JS)
								test.expectedTags = map[string]string{"key": "value"}
								test.run(t)
							})
							t.Run(fmt.Sprintf("%s/isThrow=%v/Tags", name, isThrow), func(t *testing.T) {
								test.js = fmt.Sprintf(`m.add(%v, {a:1})`, val.JS)
								test.expectedTags = map[string]string{"key": "value", "a": "1"}
								test.run(t)
							})
						}
					}
				})
			}
		})
	}
}

func TestMetricGetName(t *testing.T) {
	t.Parallel()
	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})

	mii := &modulestest.InstanceCore{
		Runtime: rt,
		InitEnv: &common.InitEnvironment{Registry: metrics.NewRegistry()},
		Ctx:     context.Background(),
	}
	m, ok := New().NewModuleInstance(mii).(*ModuleInstance)
	require.True(t, ok)
	require.NoError(t, rt.Set("metrics", m.GetExports().Named))
	v, err := rt.RunString(`
		var m = new metrics.Counter("my_metric")
		m.name
	`)
	require.NoError(t, err)
	require.Equal(t, "my_metric", v.String())

	_, err = rt.RunString(`
		"use strict";
		m.name = "something"
	`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "TypeError: Cannot assign to read only property 'name'")
}

func TestMetricDuplicates(t *testing.T) {
	t.Parallel()
	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})

	mii := &modulestest.InstanceCore{
		Runtime: rt,
		InitEnv: &common.InitEnvironment{Registry: metrics.NewRegistry()},
		Ctx:     context.Background(),
	}
	m, ok := New().NewModuleInstance(mii).(*ModuleInstance)
	require.True(t, ok)
	require.NoError(t, rt.Set("metrics", m.GetExports().Named))
	_, err := rt.RunString(`
		var m = new metrics.Counter("my_metric")
	`)
	require.NoError(t, err)

	_, err = rt.RunString(`
		var m2 = new metrics.Counter("my_metric")
	`)
	require.NoError(t, err)

	_, err = rt.RunString(`
		var m3 = new metrics.Gauge("my_metric")
	`)
	require.Error(t, err)

	_, err = rt.RunString(`
		var m4 = new metrics.Counter("my_metric", true)
	`)
	require.Error(t, err)

	v, err := rt.RunString(`
		m.name == m2.name && m.name == "my_metric" && m3 === undefined && m4 === undefined
	`)
	require.NoError(t, err)

	require.True(t, v.ToBoolean())
}
