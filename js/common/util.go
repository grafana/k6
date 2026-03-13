// Package common contains helpers for interacting with the JavaScript runtime.
package common

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/grafana/sobek"
)

// JSException represents a Go error that knows how to materialize itself as a JS value.
// Implementors can expose rich JavaScript Error objects (with proper prototypes) so
// they can participate in instanceof checks and other JS semantics.
type JSException interface {
	error
	JSValue(rt *sobek.Runtime) sobek.Value
}

// Throw a JS error; avoids re-wrapping GoErrors.
func Throw(rt *sobek.Runtime, err error) {
	if err == nil {
		return
	}

	if e, ok := err.(*sobek.Exception); ok { //nolint:errorlint // we don't really want to unwrap here
		panic(e)
	}

	var jsErr JSException
	if errors.As(err, &jsErr) {
		panic(jsErr.JSValue(rt))
	}

	panic(rt.NewGoError(err)) // this catches the stack unlike rt.ToValue
}

// GetReader tries to return an io.Reader value from an exported Sobek value.
func GetReader(data any) (io.Reader, error) {
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
func ToBytes(data any) ([]byte, error) {
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
func ToString(data any) (string, error) {
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
