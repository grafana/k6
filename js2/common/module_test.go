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
	"context"
	"strconv"
	"testing"

	"github.com/dop251/goja"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

type testModule struct {
	Counter int
}

func (*testModule) unexported() bool { return true }

func (*testModule) Func() {}

func (*testModule) Error() error { return errors.New("error") }

func (*testModule) Add(a, b int) int { return a + b }

func (*testModule) AddWithError(a, b int) (int, error) {
	res := a + b
	if res < 0 {
		return 0, errors.New("answer is negative")
	}
	return res, nil
}

func (m *testModule) Count() int {
	m.Counter++
	return m.Counter
}

func (*testModule) Context(ctx context.Context) {}

func (*testModule) ContextAdd(ctx context.Context, a, b int) int {
	return a + b
}

func (*testModule) ContextAddWithError(ctx context.Context, a, b int) (int, error) {
	res := a + b
	if res < 0 {
		return 0, errors.New("answer is negative")
	}
	return res, nil
}

func TestModuleExport(t *testing.T) {
	impl := &testModule{}
	mod := Module{Impl: impl}
	rt := goja.New()
	rt.Set("mod", mod.Export(rt))

	t.Run("unexported", func(t *testing.T) {
		_, err := RunString(rt, `mod.unexported()`)
		assert.EqualError(t, err, "TypeError: Object has no member 'unexported'")
	})
	t.Run("Func", func(t *testing.T) {
		_, err := RunString(rt, `mod.func()`)
		assert.NoError(t, err)
	})
	t.Run("Error", func(t *testing.T) {
		_, err := RunString(rt, `mod.error()`)
		assert.EqualError(t, err, "GoError: error")
	})
	t.Run("Add", func(t *testing.T) {
		v, err := RunString(rt, `mod.add(1, 2)`)
		assert.NoError(t, err)
		assert.Equal(t, int64(3), v.Export())
	})
	t.Run("AddWithError", func(t *testing.T) {
		v, err := RunString(rt, `mod.addWithError(1, 2)`)
		assert.NoError(t, err)
		assert.Equal(t, int64(3), v.Export())

		t.Run("Negative", func(t *testing.T) {
			_, err := RunString(rt, `mod.addWithError(0, -1)`)
			assert.EqualError(t, err, "GoError: answer is negative")
		})
	})
	t.Run("Count", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			t.Run(strconv.Itoa(i), func(t *testing.T) {
				v, err := RunString(rt, `mod.count()`)
				assert.NoError(t, err)
				assert.Equal(t, int64(i+1), v.Export())
				assert.Equal(t, i+1, impl.Counter)
			})
		}
	})
	t.Run("Context", func(t *testing.T) {
		_, err := RunString(rt, `mod.context()`)
		assert.EqualError(t, err, "GoError: Context needs a valid VU context")

		t.Run("Valid", func(t *testing.T) {
			mod.Context = context.Background()
			defer func() { mod.Context = nil }()

			_, err := RunString(rt, `mod.context()`)
			assert.NoError(t, err)
		})

		t.Run("Expired", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			mod.Context = ctx
			defer func() { mod.Context = nil }()

			_, err := RunString(rt, `mod.context()`)
			assert.EqualError(t, err, "GoError: test has ended")
		})
	})
	t.Run("ContextAdd", func(t *testing.T) {
		_, err := RunString(rt, `mod.contextAdd(1, 2)`)
		assert.EqualError(t, err, "GoError: ContextAdd needs a valid VU context")

		t.Run("Valid", func(t *testing.T) {
			mod.Context = context.Background()
			defer func() { mod.Context = nil }()

			v, err := RunString(rt, `mod.contextAdd(1, 2)`)
			assert.NoError(t, err)
			assert.Equal(t, int64(3), v.Export())
		})

		t.Run("Expired", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			mod.Context = ctx
			defer func() { mod.Context = nil }()

			_, err := RunString(rt, `mod.contextAdd(1, 2)`)
			assert.EqualError(t, err, "GoError: test has ended")
		})
	})
	t.Run("ContextAddWithError", func(t *testing.T) {
		_, err := RunString(rt, `mod.contextAddWithError(1, 2)`)
		assert.EqualError(t, err, "GoError: ContextAddWithError needs a valid VU context")

		t.Run("Valid", func(t *testing.T) {
			mod.Context = context.Background()
			defer func() { mod.Context = nil }()

			v, err := RunString(rt, `mod.contextAddWithError(1, 2)`)
			assert.NoError(t, err)
			assert.Equal(t, int64(3), v.Export())

			t.Run("Negative", func(t *testing.T) {
				_, err := RunString(rt, `mod.contextAddWithError(0, -1)`)
				assert.EqualError(t, err, "GoError: answer is negative")
			})
		})
		t.Run("Expired", func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			mod.Context = ctx
			defer func() { mod.Context = nil }()

			_, err := RunString(rt, `mod.contextAddWithError(1, 2)`)
			assert.EqualError(t, err, "GoError: test has ended")
		})
	})
}
