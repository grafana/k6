package common

import (
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFrozenObject(t *testing.T) {
	t.Parallel()

	rt := goja.New()
	obj := rt.NewObject()
	require.NoError(t, obj.Set("foo", "bar"))
	require.NoError(t, rt.Set("obj", obj))

	v, err := rt.RunString(`obj.foo`)
	require.NoError(t, err)
	require.Equal(t, "bar", v.String())

	// Set a nested object
	_, err = rt.RunString(`obj.nested = {propkey: "value1"}`)
	require.NoError(t, err)

	// Not yet frozen
	v, err = rt.RunString(`Object.isFrozen(obj)`)
	require.NoError(t, err)
	require.False(t, v.ToBoolean())

	require.NoError(t, FreezeObject(rt, obj))

	// It has been frozen
	v, err = rt.RunString(`Object.isFrozen(obj)`)
	require.NoError(t, err)
	require.True(t, v.ToBoolean())

	// It has deeply frozen the properties
	vfoo, err := rt.RunString(`Object.isFrozen(obj.foo)`)
	require.NoError(t, err)
	require.True(t, vfoo.ToBoolean())

	// And deeply frozen the nested objects
	vnested, err := rt.RunString(`Object.isFrozen(obj.nested)`)
	require.NoError(t, err)
	require.True(t, vnested.ToBoolean())

	nestedProp, err := rt.RunString(`Object.isFrozen(obj.nested.propkey)`)
	require.NoError(t, err)
	require.True(t, nestedProp.ToBoolean())

	// The assign is silently ignored
	_, err = rt.RunString(`obj.foo = "bad change"`)
	require.NoError(t, err)

	v, err = rt.RunString(`obj.foo`)
	require.NoError(t, err)
	assert.Equal(t, "bar", v.String())

	// If the strict mode is enabled then it fails
	v, err = rt.RunString(`'use strict'; obj.foo = "bad change"`)
	require.NotNil(t, err)
	assert.Contains(t, err.Error(), "Cannot assign to read only property 'foo'")
	assert.Nil(t, v)
}
