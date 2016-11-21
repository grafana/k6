package js

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMetricAdd(t *testing.T) {
	values := map[string]float64{
		`1234`:   1234.0,
		`1234.5`: 1234.5,
		`true`:   1.0,
		`false`:  0.0,
	}

	for jsV, v := range values {
		t.Run("v="+jsV, func(t *testing.T) {
			tags := map[string]map[string]string{
				`undefined`:     map[string]string{},
				`{tag:"value"}`: map[string]string{"tag": "value"},
				`{tag:1234}`:    map[string]string{"tag": "1234"},
				`{tag:1234.5}`:  map[string]string{"tag": "1234.5"},
			}

			for jsT, t_ := range tags {
				t.Run("t="+jsT, func(t *testing.T) {
					r, err := newSnippetRunner(fmt.Sprintf(`
						import { _assert } from "speedboat";
						import { Counter } from "speedboat/metrics";
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
