// Package common contains helpers for interacting with the JavaScript runtime.
package common

import (
	"bytes"
	"fmt"
	"io"

	"github.com/grafana/sobek"
)

// Throw a JS error; avoids re-wrapping GoErrors.
func Throw(rt *sobek.Runtime, err error) {
	if e, ok := err.(*sobek.Exception); ok { //nolint:errorlint // we don't really want to unwrap here
		panic(e)
	}
	panic(rt.NewGoError(err)) // this catches the stack unlike rt.ToValue
}

// GetReader tries to return an io.Reader value from an exported Sobek value.
func GetReader(data interface{}) (io.Reader, error) {
	switch r := data.(type) {
	case string:
		return bytes.NewBufferString(r), nil
	case []byte:
		return bytes.NewBuffer(r), nil
	case io.Reader:
		return r, nil
	case sobek.ArrayBuffer:
		return bytes.NewBuffer(r.Bytes()), nil
	default:
		return nil, fmt.Errorf("invalid type %T, it needs to be a string or ArrayBuffer", data)
	}
}

// ToBytes tries to return a byte slice from compatible types.
func ToBytes(data interface{}) ([]byte, error) {
	switch dt := data.(type) {
	case []byte:
		return dt, nil
	case string:
		return []byte(dt), nil
	case sobek.ArrayBuffer:
		return dt.Bytes(), nil
	default:
		return nil, fmt.Errorf("invalid type %T, expected string, []byte or ArrayBuffer", data)
	}
}

// ToString tries to return a string from compatible types.
func ToString(data interface{}) (string, error) {
	switch dt := data.(type) {
	case []byte:
		return string(dt), nil
	case string:
		return dt, nil
	case sobek.ArrayBuffer:
		return string(dt.Bytes()), nil
	default:
		return "", fmt.Errorf("invalid type %T, expected string, []byte or ArrayBuffer", data)
	}
}

// IsNullish checks if the given value is nullish, i.e. nil, undefined or null.
func IsNullish(v sobek.Value) bool {
	return v == nil || sobek.IsUndefined(v) || sobek.IsNull(v)
}

// IsAsyncFunction checks if the provided value is an AsyncFunction
func IsAsyncFunction(rt *sobek.Runtime, val sobek.Value) bool {
	if IsNullish(val) {
		return false
	}
	return val.ToObject(rt).Get("constructor").ToObject(rt).Get("name").String() == "AsyncFunction"
}
