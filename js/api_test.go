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
	"github.com/loadimpact/k6/lib"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestSleep(t *testing.T) {
	if testing.Short() {
		return
	}

	start := time.Now()
	JSAPI{}.Sleep(0.2)
	assert.True(t, time.Since(start) > 200*time.Millisecond)
	assert.True(t, time.Since(start) < 1*time.Second)
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

	_, err = vu.RunOnce(context.Background())
	assert.NoError(t, err)

	if !assert.Len(t, r.Checks, 1) {
		return
	}
	c := r.Checks[0]
	assert.Equal(t, "v === 3", c.Name)
	assert.Equal(t, r.DefaultGroup, c.Group)
	assert.Equal(t, int64(1), c.Passes)
	assert.Equal(t, int64(0), c.Fails)
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

	_, err = vu.RunOnce(context.Background())
	assert.NoError(t, err)

	assert.Len(t, r.Groups, 2)
	g := r.Groups[1]
	assert.Equal(t, "group", g.Name)

	assert.Len(t, r.Checks, 1)
	c := r.Checks[0]
	assert.Equal(t, "v === 3", c.Name)
	assert.Equal(t, g, c.Group)
	assert.Equal(t, int64(1), c.Passes)
	assert.Equal(t, int64(0), c.Fails)
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

func TestCheckReturnFalseAndTaintsOnFailure(t *testing.T) {
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
	assert.Equal(t, lib.ErrVUWantsTaint, err)
}

func TestTaint(t *testing.T) {
	if testing.Short() {
		return
	}

	r, err := newSnippetRunner(`
	import { taint } from "k6";
	export default function() {
		taint();
	}`)
	assert.NoError(t, err)

	vu, err := r.NewVU()
	assert.NoError(t, err)

	_, err = vu.RunOnce(context.Background())
	assert.Equal(t, lib.ErrVUWantsTaint, err)
}
