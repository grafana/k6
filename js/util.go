/*

k6 - a next-generation load testing tool
Copyright (C) 2016 Load Impact

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.

*/

package js

import (
	"context"
	"github.com/robertkrimen/otto"
)

func Check(val, arg0 otto.Value) (bool, error) {
	switch {
	case val.IsFunction():
		val, err := val.Call(otto.UndefinedValue(), arg0)
		if err != nil {
			return false, err
		}
		return Check(val, arg0)
	case val.IsBoolean():
		b, err := val.ToBoolean()
		if err != nil {
			return false, err
		}
		return b, nil
	case val.IsNumber():
		f, err := val.ToFloat()
		if err != nil {
			return false, err
		}
		return f != 0, nil
	case val.IsString():
		s, err := val.ToString()
		if err != nil {
			return false, err
		}
		return s != "", nil
	default:
		return false, nil
	}
}

func throw(vm *otto.Otto, v interface{}) {
	if err, ok := v.(error); ok {
		panic(vm.MakeCustomError("Error", err.Error()))
	}
	panic(v)
}

func newSnippetRunner(src string) (*Runner, error) {
	rt, err := New()
	if err != nil {
		return nil, err
	}
	rt.VM.Set("__initapi__", InitAPI{r: rt})
	defer rt.VM.Set("__initapi__", nil)

	exp, err := rt.load("__snippet__", []byte(src))
	if err != nil {
		return nil, err
	}

	return NewRunner(rt, exp)
}

func runSnippet(src string) error {
	r, err := newSnippetRunner(src)
	if err != nil {
		return err
	}
	vu, err := r.NewVU()
	if err != nil {
		return err
	}
	_, err = vu.RunOnce(context.Background())
	if err != nil {
		return err
	}
	return nil
}
