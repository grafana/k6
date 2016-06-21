package js

import (
	"errors"
	"github.com/robertkrimen/otto"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestBodyFromValueUndefined(t *testing.T) {
	body, err := bodyFromValue(otto.UndefinedValue())
	assert.NoError(t, err)
	assert.Equal(t, "", body)
}

func TestBodyFromValueNull(t *testing.T) {
	body, err := bodyFromValue(otto.NullValue())
	assert.NoError(t, err)
	assert.Equal(t, "", body)
}

func TestBodyFromValueString(t *testing.T) {
	val, err := otto.ToValue("abc123")
	assert.NoError(t, err)
	body, err := bodyFromValue(val)
	assert.NoError(t, err)
	assert.Equal(t, "abc123", body)
}

func TestBodyFromValueObject(t *testing.T) {
	vm := otto.New()
	val, err := vm.ToValue(map[string]string{"a": "b"})
	assert.NoError(t, err)
	body, err := bodyFromValue(val)
	assert.NoError(t, err)
	assert.Equal(t, "a=b", body)
}

func TestPutBodyInURL(t *testing.T) {
	assert.Equal(t, "http://example.com/?a=b", putBodyInURL("http://example.com/", "a=b"))
}

func TestPutBodyInURLWithQuery(t *testing.T) {
	assert.Equal(t, "http://example.com/?aa=bb&a=b", putBodyInURL("http://example.com/?aa=bb", "a=b"))
}

func TestMake(t *testing.T) {
	vm := otto.New()

	_, err := vm.Eval(`function MyType() { this.a = 'b'; };`)
	assert.NoError(t, err, "couldn't set up VM")

	obj, err := Make(vm, "MyType")
	assert.NoError(t, err)
	assert.NotNil(t, obj)
	assert.Equal(t, "Object", obj.Class())

	aVal, err := obj.Get("a")
	assert.NoError(t, err, "couldn't get 'a'")
	a, err := aVal.ToString()
	assert.NoError(t, err, "couldn't turn a into a string")
	assert.Equal(t, "b", a, "a != 'b'")
}

func TestJSCustomError(t *testing.T) {
	vm := otto.New()
	vm.Set("fn", func(call otto.FunctionCall) otto.Value {
		e := jsCustomError(vm, "CustomError", errors.New("test error"))
		str, err := e.ToString()
		assert.NoError(t, err)
		assert.Equal(t, "CustomError: test error", str)
		return otto.UndefinedValue()
	})
	_, err := vm.Eval("fn()")
	assert.NoError(t, err)
}
