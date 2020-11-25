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

package js

// TODO move this to another package
// it can possibly be even in a separate repo if the error handling is fixed

import (
	"context"
	"encoding/json"

	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js/common"
)

// TODO rename
// TODO check how it works with setup data
// TODO check how it works if logged
// TODO maybe drop it and leave the sharedArray only for now
type shared struct {
	value *goja.Object
}

func (s shared) Get(ctx context.Context, index goja.Value) (interface{}, error) {
	rt := common.GetRuntime(ctx)
	// TODO other index
	val := s.value.Get(index.String())
	if val == nil {
		return goja.Undefined(), nil
	}
	b, err := val.ToObject(rt).MarshalJSON()
	if err != nil { // cache bytes, pre marshal
		return goja.Undefined(), err
	}
	var tmp interface{}
	if err = json.Unmarshal(b, &tmp); err != nil {
		return goja.Undefined(), err
	}
	return tmp, nil
}

type sharedArray struct {
	arr [][]byte
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
