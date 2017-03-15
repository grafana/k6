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
	"errors"
	"testing"

	"github.com/robertkrimen/otto"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func newSnippetRunner(src string) (*Runner, error) {
	rt, err := New()
	if err != nil {
		return nil, err
	}

	_ = rt.VM.Set("__initapi__", &InitAPI{r: rt, fs: afero.NewOsFs()})
	exp, err := rt.load("__snippet__", []byte(src))
	_ = rt.VM.Set("__initapi__", nil)
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
	return err
}

func TestCheck(t *testing.T) {
	if testing.Short() {
		return
	}

	vm := otto.New()

	t.Run("String", func(t *testing.T) {
		t.Run("Something", func(t *testing.T) {
			v, err := vm.Eval(`"test"`)
			assert.NoError(t, err)
			b, err := Check(v, otto.UndefinedValue())
			assert.NoError(t, err)
			assert.True(t, b)
		})

		t.Run("Empty", func(t *testing.T) {
			v, err := vm.Eval(`""`)
			assert.NoError(t, err)
			b, err := Check(v, otto.UndefinedValue())
			assert.NoError(t, err)
			assert.False(t, b)
		})
	})

	t.Run("Number", func(t *testing.T) {
		t.Run("Positive", func(t *testing.T) {
			v, err := vm.Eval(`1`)
			assert.NoError(t, err)
			b, err := Check(v, otto.UndefinedValue())
			assert.NoError(t, err)
			assert.True(t, b)
		})
		t.Run("Negative", func(t *testing.T) {
			v, err := vm.Eval(`-1`)
			assert.NoError(t, err)
			b, err := Check(v, otto.UndefinedValue())
			assert.NoError(t, err)
			assert.True(t, b)
		})
		t.Run("Zero", func(t *testing.T) {
			v, err := vm.Eval(`0`)
			assert.NoError(t, err)
			b, err := Check(v, otto.UndefinedValue())
			assert.NoError(t, err)
			assert.False(t, b)
		})
	})

	t.Run("Boolean", func(t *testing.T) {
		t.Run("True", func(t *testing.T) {
			v, err := vm.Eval(`true`)
			assert.NoError(t, err)
			b, err := Check(v, otto.UndefinedValue())
			assert.NoError(t, err)
			assert.True(t, b)
		})
		t.Run("False", func(t *testing.T) {
			v, err := vm.Eval(`false`)
			assert.NoError(t, err)
			b, err := Check(v, otto.UndefinedValue())
			assert.NoError(t, err)
			assert.False(t, b)
		})
	})

	t.Run("Function", func(t *testing.T) {
		fn, err := vm.Eval(`(function(v) { return v === true; })`)
		assert.NoError(t, err)

		t.Run("True", func(t *testing.T) {
			b, err := Check(fn, otto.TrueValue())
			assert.NoError(t, err)
			assert.True(t, b)
		})
		t.Run("False", func(t *testing.T) {
			b, err := Check(fn, otto.FalseValue())
			assert.NoError(t, err)
			assert.False(t, b)
		})
	})
}

func TestThrow(t *testing.T) {
	if testing.Short() {
		return
	}

	vm := otto.New()
	assert.NoError(t, vm.Set("fn", func() {
		throw(vm, errors.New("This is a test error"))
	}))
	_, err := vm.Eval(`fn()`)
	assert.EqualError(t, err, "Error: This is a test error")
}
