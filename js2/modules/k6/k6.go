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

package k6

import (
	"context"

	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js2/common"
)

var Module = common.Module{Impl: &K6{}}

type K6 struct{}

func (*K6) Group(ctx context.Context, name string, fn goja.Callable) (goja.Value, error) {
	state := common.GetState(ctx)

	g, err := state.Group.Group(name)
	if err != nil {
		return goja.Undefined(), err
	}

	old := state.Group
	state.Group = g
	defer func() { state.Group = old }()

	return fn(goja.Undefined())
}

func (*K6) Check(ctx context.Context, arg0, checks goja.Value) (bool, error) {
	// state := common.GetState(ctx)
	rt := common.GetRuntime(ctx)

	succ := true
	obj := checks.ToObject(rt)
	for _, key := range obj.Keys() {
		val := obj.Get(key)

		// Resolve callables into values.
		fn, ok := goja.AssertFunction(val)
		if ok {
			val_, err := fn(goja.Undefined(), arg0)
			if err != nil {
				return false, err
			}
			val = val_
		}

		if !val.ToBoolean() {
			succ = false
		}
	}

	return succ, nil
}
