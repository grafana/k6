package js

import (
	"github.com/robertkrimen/otto"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestBodyFromValueUndefined(t *testing.T) {
	body, err := bodyFromValue(otto.UndefinedValue())
	assert.Nil(t, err)
	assert.Equal(t, "", body)
}

func TestBodyFromValueNull(t *testing.T) {
	body, err := bodyFromValue(otto.NullValue())
	assert.Nil(t, err)
	assert.Equal(t, "", body)
}

func TestBodyFromValueString(t *testing.T) {
	val, err := otto.ToValue("abc123")
	assert.Nil(t, err)
	body, err := bodyFromValue(val)
	assert.Nil(t, err)
	assert.Equal(t, "abc123", body)
}

func TestBodyFromValueObject(t *testing.T) {
	vm := otto.New()
	val, err := vm.ToValue(map[string]string{"a": "b"})
	assert.Nil(t, err)
	body, err := bodyFromValue(val)
	assert.Nil(t, err)
	assert.Equal(t, "a=b", body)
}

func TestPutBodyInURL(t *testing.T) {
	assert.Equal(t, "http://example.com/?a=b", putBodyInURL("http://example.com/", "a=b"))
}

func TestPutBodyInURLWithQuery(t *testing.T) {
	assert.Equal(t, "http://example.com/?aa=bb&a=b", putBodyInURL("http://example.com/?aa=bb", "a=b"))
}
