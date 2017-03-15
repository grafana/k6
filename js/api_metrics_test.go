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
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMetricAdd(t *testing.T) {
	if testing.Short() {
		return
	}

	values := map[string]float64{
		`1234`:   1234.0,
		`1234.5`: 1234.5,
		`true`:   1.0,
		`false`:  0.0,
	}

	for jsV, v := range values {
		t.Run("v="+jsV, func(t *testing.T) {
			tags := map[string]map[string]string{
				`undefined`:     {},
				`{tag:"value"}`: {"tag": "value"},
				`{tag:1234}`:    {"tag": "1234"},
				`{tag:1234.5}`:  {"tag": "1234.5"},
			}

			for jsT, t_ := range tags {
				t.Run("t="+jsT, func(t *testing.T) {
					r, err := newSnippetRunner(fmt.Sprintf(`
						import { _assert } from "k6";
						import { Counter } from "k6/metrics";
						let myMetric = new Counter("my_metric");
						export default function() {
							let v = %s;
							let t = %s;
							_assert(myMetric.add(v, t) === v);
						}
					`, jsV, jsT))

					if !assert.NoError(t, err) {
						return
					}

					vu, err := r.NewVU()
					if !assert.NoError(t, err) {
						return
					}

					samples, err := vu.RunOnce(context.Background())
					if !assert.NoError(t, err) {
						return
					}

					assert.Len(t, samples, 1)
					s := samples[0]
					assert.Equal(t, r.Runtime.Metrics["my_metric"], s.Metric)
					assert.Equal(t, v, s.Value)
					assert.EqualValues(t, t_, s.Tags)
				})
			}
		})
	}
}
