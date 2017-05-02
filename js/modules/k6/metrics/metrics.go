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
	"errors"
	"time"

	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/stats"
)

type Metric struct {
	metric *stats.Metric
}

func newMetric(ctxPtr *context.Context, name string, t stats.MetricType, isTime []bool) (interface{}, error) {
	if common.GetState(*ctxPtr) != nil {
		return nil, errors.New("Metrics must be declared in the init context")
	}

	valueType := stats.Default
	if len(isTime) > 0 && isTime[0] {
		valueType = stats.Time
	}

	rt := common.GetRuntime(*ctxPtr)
	return common.Bind(rt, Metric{stats.New(name, t, valueType)}, ctxPtr), nil
}

func (m Metric) Add(ctx context.Context, v goja.Value, addTags ...map[string]string) {
	state := common.GetState(ctx)

	tags := map[string]string{
		"group": state.Group.Path,
	}
	for _, ts := range addTags {
		for k, v := range ts {
			tags[k] = v
		}
	}

	vfloat := v.ToFloat()
	if vfloat == 0 && v.ToBoolean() {
		vfloat = 1.0
	}

	state.Samples = append(state.Samples,
		stats.Sample{Time: time.Now(), Metric: m.metric, Value: vfloat, Tags: tags},
	)
}

type Metrics struct{}

func (*Metrics) XCounter(ctx *context.Context, name string, isTime ...bool) (interface{}, error) {
	return newMetric(ctx, name, stats.Counter, isTime)
}

func (*Metrics) XGauge(ctx *context.Context, name string, isTime ...bool) (interface{}, error) {
	return newMetric(ctx, name, stats.Gauge, isTime)
}

func (*Metrics) XTrend(ctx *context.Context, name string, isTime ...bool) (interface{}, error) {
	return newMetric(ctx, name, stats.Trend, isTime)
}

func (*Metrics) XRate(ctx *context.Context, name string, isTime ...bool) (interface{}, error) {
	return newMetric(ctx, name, stats.Rate, isTime)
}
