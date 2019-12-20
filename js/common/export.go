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

package common

import (
	"github.com/dop251/goja"
)

func ExportBytes(rt *goja.Runtime, val goja.Value) interface {} {
	// Inspect val to see if it is a Uint8Array. If along the way anything doesn't look right, bail out
	// with val.Export().
	if val == nil {
		return val
	}
	obj, isObj := val.(*goja.Object)
	if !isObj {
		return val.Export()
	}
	// fmt.Printf("ClassName: %s\n", obj.ClassName())
	constructor := obj.Get("constructor")
	if constructor == nil {
		return val.Export()
	}
	consObj := constructor.ToObject(rt)
	if consObj == nil {
		return val.Export()
	}
	consName := consObj.Get("name")
	if consName == nil {
		return val.Export()
	}
	consNameToStr := consName.ToString()
	if consNameToStr == nil {
		return val.Export()
	}
	consNameStr := consNameToStr.String()
	if consNameStr != "Uint8Array" {
		return val.Export()
	}
	byteLengthVal := obj.Get("byteLength")
	forEachVal := obj.Get("forEach")
	if byteLengthVal == nil || forEachVal == nil {
		return val.Export()
	}

	// Setup a buffer and associated accumulator callback, then invoke obj.forEach to accumulate the bytes.
	buf := make([]byte, byteLengthVal.ToInteger())
	forEachFunc, goodForEach := forEachVal.Export().(func(goja.FunctionCall) goja.Value)
	if !goodForEach {
		return val.Export()
	}
	accumulatorIndex := 0
	accumulator := func(call goja.FunctionCall) goja.Value {
		buf[accumulatorIndex] = byte(call.Argument(0).ToInteger())
		accumulatorIndex++
		return nil
	}
	forEachFunc(goja.FunctionCall{
		This:      obj,
		Arguments: []goja.Value{rt.ToValue(accumulator)},
	})

	// alternative implementation without forEach
	// for i:=0; i< len(buf); i++ {
	// 	buf[i] = byte(obj.Get(strconv.Itoa(i)).ToInteger())
	// }
	return buf
}

