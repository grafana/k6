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
	"time"

	"github.com/loadimpact/k6/lib/metrics"
	"github.com/stretchr/testify/assert"
)

func TestSleep(t *testing.T) {
	if testing.Short() {
		return
	}

	testdata := map[string]struct {
		src string
		min time.Duration
	}{
		"float,sub-1s": {`0.2`, 200 * time.Millisecond},
		"float":        {`1.0`, 1 * time.Second},
		"int":          {`1`, 1 * time.Second},
		"exceeding":    {`5`, 2 * time.Second},
	}
	for name, data := range testdata {
		t.Run(name, func(t *testing.T) {
			r, err := newSnippetRunner(fmt.Sprintf(`
			import { sleep } from "k6";
			export default function() {
				sleep(%s);
			}`, data.src))
			assert.NoError(t, err)

			vu, err := r.NewVU()
			assert.NoError(t, err)

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			start := time.Now()

			_, err = vu.RunOnce(ctx)
			assert.NoError(t, err)
			assert.True(t, time.Since(start) > data.min, "ran too short")
			assert.True(t, time.Since(start) < data.min+1*time.Second, "ran too long")
		})
	}
}

func TestDoGroup(t *testing.T) {
	if testing.Short() {
		return
	}

	r, err := newSnippetRunner(`
	import { group } from "k6";
	export default function() {
		group("test", fn);
	}`)
	assert.NoError(t, err)

	vu_, err := r.NewVU()
	assert.NoError(t, err)
	vu := vu_.(*VU)

	assert.NoError(t, vu.vm.Set("fn", func() {
		assert.Equal(t, "test", vu.group.Name)
	}))

	_, err = vu.RunOnce(context.Background())
	assert.NoError(t, err)
}

func TestDoGroupNested(t *testing.T) {
	if testing.Short() {
		return
	}

	r, err := newSnippetRunner(`
	import { group } from "k6";
	export default function() {
		group("outer", function() {
			group("inner", fn);
		});
	}`)
	assert.NoError(t, err)

	vu_, err := r.NewVU()
	assert.NoError(t, err)
	vu := vu_.(*VU)

	assert.NoError(t, vu.vm.Set("fn", func() {
		assert.Equal(t, "inner", vu.group.Name)
		assert.Equal(t, "outer", vu.group.Parent.Name)
	}))

	_, err = vu.RunOnce(context.Background())
	assert.NoError(t, err)
}

func TestDoGroupReturn(t *testing.T) {
	if testing.Short() {
		return
	}

	r, err := newSnippetRunner(`
	import { group, _assert } from "k6";
	export default function() {
		let v = group("group", function() {
			return 12345;
		});
		_assert(v === 12345);
	}`)
	assert.NoError(t, err)

	vu, err := r.NewVU()
	assert.NoError(t, err)
	_, err = vu.RunOnce(context.Background())
	assert.NoError(t, err)
}

func TestDoGroupReturnTrueByDefault(t *testing.T) {
	if testing.Short() {
		return
	}

	r, err := newSnippetRunner(`
	import { group, _assert } from "k6";
	export default function() {
		let v = group("group", function() {
			// no return
		});
		_assert(v === true);
	}`)
	assert.NoError(t, err)

	vu, err := r.NewVU()
	assert.NoError(t, err)
	_, err = vu.RunOnce(context.Background())
	assert.NoError(t, err)
}

func TestDoCheck(t *testing.T) {
	if testing.Short() {
		return
	}

	r, err := newSnippetRunner(`
	import { check } from "k6";
	export default function() {
		check(3, { "v === 3": (v) => v === 3 });
	}`)
	assert.NoError(t, err)

	vu_, err := r.NewVU()
	assert.NoError(t, err)
	vu := vu_.(*VU)

	samples, err := vu.RunOnce(context.Background())
	assert.NoError(t, err)

	c := r.DefaultGroup.Checks["v === 3"]
	assert.NotNil(t, c)
	assert.Equal(t, "v === 3", c.Name)
	assert.Equal(t, r.DefaultGroup, c.Group)
	assert.Equal(t, int64(1), c.Passes)
	assert.Equal(t, int64(0), c.Fails)

	assert.Len(t, samples, 1)
	sample := samples[0]
	assert.False(t, sample.Time.IsZero(), "sample time is zero")
	assert.Equal(t, metrics.Checks, sample.Metric)
	assert.Equal(t, 1.0, sample.Value)
	assert.EqualValues(t, map[string]string{
		"group": "",
		"check": "v === 3",
	}, sample.Tags)
}

func TestCheckInGroup(t *testing.T) {
	if testing.Short() {
		return
	}

	r, err := newSnippetRunner(`
	import { group, check } from "k6";
	export default function() {
		group("group", function() {
			check(3, { "v === 3": (v) => v === 3 });
		});
	}`)
	assert.NoError(t, err)

	vu_, err := r.NewVU()
	assert.NoError(t, err)
	vu := vu_.(*VU)

	samples, err := vu.RunOnce(context.Background())
	assert.NoError(t, err)

	g := r.DefaultGroup.Groups["group"]
	assert.NotNil(t, g)
	assert.Equal(t, "group", g.Name)

	c := g.Checks["v === 3"]
	assert.NotNil(t, c)
	assert.Equal(t, "v === 3", c.Name)
	assert.Equal(t, g, c.Group)
	assert.Equal(t, int64(1), c.Passes)
	assert.Equal(t, int64(0), c.Fails)

	assert.Len(t, samples, 1)
	sample := samples[0]
	assert.False(t, sample.Time.IsZero(), "sample time is zero")
	assert.Equal(t, metrics.Checks, sample.Metric)
	assert.Equal(t, 1.0, sample.Value)
	assert.EqualValues(t, map[string]string{
		"group": "::group",
		"check": "v === 3",
	}, sample.Tags)
}

func TestCheckReturnTrueOnSuccess(t *testing.T) {
	if testing.Short() {
		return
	}

	r, err := newSnippetRunner(`
	import { check, _assert } from "k6";
	export default function() {
		let succ = check(null, { "true": true });
		_assert(succ === true);
	}`)
	assert.NoError(t, err)

	vu, err := r.NewVU()
	assert.NoError(t, err)
	_, err = vu.RunOnce(context.Background())
	assert.NoError(t, err)
}

func TestCheckReturnFalseOnFailure(t *testing.T) {
	if testing.Short() {
		return
	}

	r, err := newSnippetRunner(`
	import { check, _assert } from "k6";
	export default function() {
		let succ = check(null, { "false": false });
		_assert(succ === false);
	}`)
	assert.NoError(t, err)

	vu, err := r.NewVU()
	assert.NoError(t, err)
	_, err = vu.RunOnce(context.Background())
	assert.NoError(t, err)
}
