package common

import (
	"errors"
	"fmt"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestThrow(t *testing.T) {
	t.Parallel()
	rt := goja.New()
	fn1, ok := goja.AssertFunction(rt.ToValue(func() { Throw(rt, errors.New("aaaa")) }))
	require.True(t, ok, "fn1 is invalid")
	_, err := fn1(goja.Undefined())
	assert.EqualError(t, err, "GoError: aaaa")

	fn2, ok := goja.AssertFunction(rt.ToValue(func() { Throw(rt, err) }))
	require.True(t, ok, "fn2 is invalid")
	_, err = fn2(goja.Undefined())
	assert.EqualError(t, err, "GoError: aaaa")
}

func TestToBytes(t *testing.T) {
	t.Parallel()
	rt := goja.New()
	b := []byte("hello")
	testCases := []struct {
		in     interface{}
		expOut []byte
		expErr string
	}{
		{b, b, ""},
		{"hello", b, ""},
		{rt.NewArrayBuffer(b), b, ""},
		{struct{}{}, nil, "invalid type struct {}, expected string, []byte or ArrayBuffer"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("%T", tc.in), func(t *testing.T) {
			t.Parallel()
			out, err := ToBytes(tc.in)
			if tc.expErr != "" {
				assert.EqualError(t, err, tc.expErr)
				return
			}
			assert.Equal(t, tc.expOut, out)
		})
	}
}

func TestToString(t *testing.T) {
	t.Parallel()
	rt := goja.New()
	s := "hello"
	testCases := []struct {
		in             interface{}
		expOut, expErr string
	}{
		{s, s, ""},
		{"hello", s, ""},
		{rt.NewArrayBuffer([]byte(s)), s, ""},
		{struct{}{}, "", "invalid type struct {}, expected string, []byte or ArrayBuffer"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("%T", tc.in), func(t *testing.T) {
			t.Parallel()
			out, err := ToString(tc.in)
			if tc.expErr != "" {
				assert.EqualError(t, err, tc.expErr)
				return
			}
			assert.Equal(t, tc.expOut, out)
		})
	}
}
