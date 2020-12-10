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
	"encoding/json"

	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js/common"
)

// TODO fix it not working really well with setupData or just make it more broken
// TODO fix it working with console.log
type sharedArray struct {
	arr [][]byte
}

func (s sharedArray) wrap(ctxPtr *context.Context, rt *goja.Runtime) goja.Value {
	cal, err := rt.RunString(arrayWrapperCode)
	if err != nil {
		common.Throw(rt, err)
	}
	call, _ := goja.AssertFunction(cal)
	wrapped, err := call(goja.Undefined(), rt.ToValue(common.Bind(rt, s, ctxPtr)))
	if err != nil {
		common.Throw(rt, err)
	}

	return wrapped
}

func (s sharedArray) Get(index int) (interface{}, error) {
	if index < 0 || index >= len(s.arr) {
		return goja.Undefined(), nil
	}

	var tmp interface{}
	if err := json.Unmarshal(s.arr[index], &tmp); err != nil {
		return goja.Undefined(), err
	}
	return tmp, nil
}

func (s sharedArray) Length() int {
	return len(s.arr)
}

type sharedArrayIterator struct {
	a     *sharedArray
	index int
}

func (sai *sharedArrayIterator) Next() (interface{}, error) {
	if sai.index == len(sai.a.arr)-1 {
		return map[string]bool{"done": true}, nil
	}
	sai.index++
	var tmp interface{}
	if err := json.Unmarshal(sai.a.arr[sai.index], &tmp); err != nil {
		return goja.Undefined(), err
	}
	return map[string]interface{}{"value": tmp}, nil
}

func (s sharedArray) Iterator() *sharedArrayIterator {
	return &sharedArrayIterator{a: &s, index: -1}
}

const arrayWrapperCode = `(function(val) {
	var arrayHandler = {
		get: function(target, property, receiver) {
			switch (property){
			case "length":
				return target.length();
			case Symbol.iterator:
				return function() {
					return target.iterator();
				};
			}
			var i = parseInt(property);
			if (isNaN(i)) {
				return undefined;
			}

			return target.get(i);
		}
	};
	return new Proxy(val, arrayHandler);
})`
