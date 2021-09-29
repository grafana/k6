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

package http

import (
	"context"
	"errors"
	"fmt"

	"github.com/dop251/goja"
	"go.k6.io/k6/js/common"
)

//nolint:gochecknoglobals
var defaultExpectedStatuses = expectedStatuses{
	minmax: [][2]int{{200, 399}},
}

// expectedStatuses is specifically totally unexported so it can't be used for anything else but
// SetResponseCallback and nothing can be done from the js side to modify it or make an instance of
// it except using ExpectedStatuses
type expectedStatuses struct {
	minmax [][2]int
	exact  []int
}

func (e expectedStatuses) match(status int) bool {
	for _, v := range e.exact {
		if v == status {
			return true
		}
	}

	for _, v := range e.minmax {
		if v[0] <= status && status <= v[1] {
			return true
		}
	}
	return false
}

// ExpectedStatuses returns expectedStatuses object based on the provided arguments.
// The arguments must be either integers or object of `{min: <integer>, max: <integer>}`
// kind. The "integer"ness is checked by the Number.isInteger.
func (*HTTP) ExpectedStatuses(ctx context.Context, args ...goja.Value) *expectedStatuses { //nolint: golint
	rt := common.GetRuntime(ctx)

	if len(args) == 0 {
		common.Throw(rt, errors.New("no arguments"))
	}
	var result expectedStatuses

	jsIsInt, _ := goja.AssertFunction(rt.GlobalObject().Get("Number").ToObject(rt).Get("isInteger"))
	isInt := func(a goja.Value) bool {
		v, err := jsIsInt(goja.Undefined(), a)
		return err == nil && v.ToBoolean()
	}

	errMsg := "argument number %d to expectedStatuses was neither an integer nor an object like {min:100, max:329}"
	for i, arg := range args {
		o := arg.ToObject(rt)
		if o == nil {
			common.Throw(rt, fmt.Errorf(errMsg, i+1))
		}

		if isInt(arg) {
			result.exact = append(result.exact, int(o.ToInteger()))
		} else {
			min := o.Get("min")
			max := o.Get("max")
			if min == nil || max == nil {
				common.Throw(rt, fmt.Errorf(errMsg, i+1))
			}
			if !(isInt(min) && isInt(max)) {
				common.Throw(rt, fmt.Errorf("both min and max need to be integers for argument number %d", i+1))
			}

			result.minmax = append(result.minmax, [2]int{int(min.ToInteger()), int(max.ToInteger())})
		}
	}
	return &result
}

// SetResponseCallback sets the responseCallback to the value provided. Supported values are
// expectedStatuses object or a `null` which means that metrics shouldn't be tagged as failed and
// `http_req_failed` should not be emitted - the behaviour previous to this
func (h *HTTP) SetResponseCallback(ctx context.Context, val goja.Value) {
	if val != nil && !goja.IsNull(val) {
		// This is done this way as ExportTo exports functions to empty structs without an error
		if es, ok := val.Export().(*expectedStatuses); ok {
			h.responseCallback = es.match
		} else {
			//nolint:golint
			common.Throw(common.GetRuntime(ctx), fmt.Errorf("unsupported argument, expected http.expectedStatuses"))
		}
	} else {
		h.responseCallback = nil
	}
}
