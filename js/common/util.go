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

	"github.com/loadimpact/k6/js/compiler"
)

// RunString Runs an string in the given runtime. Use this if writing ES5 in tests isn't a problem.
func RunString(rt *goja.Runtime, src string) (goja.Value, error) {
	return rt.RunString(src)
}

// RunES6String Runs an ES6 string in the given runtime. Use this rather than writing ES5 in tests.
func RunES6String(rt *goja.Runtime, src string) (goja.Value, error) {
	var err error
	c := compiler.New()
	src, _, err = c.Transform(src, "__string__")
	if err != nil {
		return goja.Undefined(), err
	}
	return RunString(rt, src)
}

// Throws a JS error; avoids re-wrapping GoErrors.
func Throw(rt *goja.Runtime, err error) {
	if e, ok := err.(*goja.Exception); ok {
		panic(e)
	}
	panic(rt.NewGoError(err))
}
