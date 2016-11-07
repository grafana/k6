package js

import (
	"errors"
	"github.com/robertkrimen/otto"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCheck(t *testing.T) {
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
	vm := otto.New()
	vm.Set("fn", func() {
		throw(vm, errors.New("This is a test error"))
	})
	_, err := vm.Eval(`fn()`)
	assert.EqualError(t, err, "Error: This is a test error")
}
