/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2020 Load Impact
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

package data

import (
	"context"

	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/js/internal/modules"
	"github.com/loadimpact/k6/lib"
	"github.com/pkg/errors"
)

type data struct{}

func init() {
	modules.Register("k6/data", new(data))
}

const sharedArrayNamePrefix = "k6/data/SharedArray."

// XSharedArray is a constructor returning a shareable read-only array
// indentified by the name and having their contents be whatever the call returns
func (d *data) XSharedArray(ctx context.Context, name string, call goja.Callable) (goja.Value, error) {
	if lib.GetState(ctx) != nil {
		return nil, errors.New("new SharedArray must be called in the init context")
	}

	initEnv := common.GetInitEnv(ctx)
	if initEnv == nil {
		return nil, errors.New("missing init environment")
	}
	if len(name) == 0 {
		return nil, errors.New("empty name provided to SharedArray's constructor")
	}

	name = sharedArrayNamePrefix + name
	value := initEnv.SharedObjects.GetOrCreateShare(name, func() interface{} {
		return getShareArrayFromCall(common.GetRuntime(ctx), call)
	})
	array, ok := value.(sharedArray)
	if !ok { // TODO more info in the error?
		return nil, errors.New("wrong type of shared object")
	}

	return array.wrap(&ctx, common.GetRuntime(ctx)), nil
}

func getShareArrayFromCall(rt *goja.Runtime, call goja.Callable) sharedArray {
	gojaValue, err := call(goja.Undefined())
	if err != nil {
		common.Throw(rt, err)
	}
	obj := gojaValue.ToObject(rt)
	if obj.ClassName() != "Array" {
		common.Throw(rt, errors.New("only arrays can be made into SharedArray")) // TODO better error
	}
	arr := make([]string, obj.Get("length").ToInteger())

	cal, err := rt.RunString(`(function(input, output) {
		for (var i = 0; i < input.length; i++) {
			output[i] = JSON.stringify(input[i])
		}
	})`)
	if err != nil {
		common.Throw(rt, err)
	}
	newCall, _ := goja.AssertFunction(cal)
	_, err = newCall(goja.Undefined(), gojaValue, rt.ToValue(arr))
	if err != nil {
		common.Throw(rt, err)
	}
	return sharedArray{arr: arr}
}
