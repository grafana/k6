/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2021 Load Impact
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
package jstest

import (
	"context"
	"fmt"
	"time"

	"go.k6.io/k6/lib"
	"go.k6.io/k6/stats"

	"go.k6.io/k6/js/modules"
)

func init() {
	modules.Register("k6/x/jsexttest", new(JSTest))
}

// JSTest is meant to test xk6 and the JS extension sub-system of k6.
type JSTest struct{}

// Foo emits a foo metric
func (j JSTest) Foo(ctx context.Context, arg float64) (bool, error) {
	state := lib.GetState(ctx)
	if state == nil {
		return false, fmt.Errorf("called in init context")
	}

	allTheFoos := stats.New("foos", stats.Counter)
	tags := state.CloneTags()
	tags["foo"] = "bar"
	stats.PushIfNotDone(ctx, state.Samples, stats.Sample{
		Time:   time.Now(),
		Metric: allTheFoos, Tags: stats.IntoSampleTags(&tags),
		Value: arg,
	})

	return true, nil
}
