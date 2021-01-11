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
)

// TODO fix it not working really well with setupData or just make it more broken
// TODO fix it working with console.log
type sharedArray struct {
	arr []string
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

	// we specifically use JSON.parse to get the json to an object inside as otherwise we won't be
	// able to freeze it as goja doesn't let us unless it is a pure goja object and this is the
	// easiest way to get one.
	return s.arr[index], nil
}

func (s sharedArray) Length() int {
	return len(s.arr)
}

/* This implementation is commented as with it - it is harder to deepFreeze it with this implementation.
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
*/

const arrayWrapperCode = `(function(val) {
	function deepFreeze(o) {
		Object.freeze(o);
		if (o === undefined) {
			return o;
		}

		Object.getOwnPropertyNames(o).forEach(function (prop) {
			if (o[prop] !== null
				&& (typeof o[prop] === "object" || typeof o[prop] === "function")
				&& !Object.isFrozen(o[prop])) {
				deepFreeze(o[prop]);
			}
		});

		return o;
	};

	var arrayHandler = {
		get: function(target, property, receiver) {
			switch (property){
			case "length":
				return target.length();
			case Symbol.iterator:
				return function(){
					var index = 0;
					return {
						"next": function() {
							if (index >= target.length()) {
								return {done: true}
							}
							var result = {value: deepFreeze(JSON.parse(target.get(index)))};
							index++;
							return result;
						}
					}
				}
			}
			var i = parseInt(property);
			if (isNaN(i)) {
				return undefined;
			}

			return deepFreeze(JSON.parse(target.get(i)));
		}
	};
	return new Proxy(val, arrayHandler);
})`
