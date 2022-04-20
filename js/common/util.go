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

// Package common contains helpers for interacting with the JavaScript runtime.
package common

import (
	"bytes"
	"fmt"
	"io"
	"runtime/debug"

	"github.com/dop251/goja"
	"github.com/sirupsen/logrus"
)

// Throw a JS error; avoids re-wrapping GoErrors.
func Throw(rt *goja.Runtime, err error) {
	if e, ok := err.(*goja.Exception); ok {
		panic(e)
	}
	panic(rt.ToValue(err))
}

// GetReader tries to return an io.Reader value from an exported goja value.
func GetReader(data interface{}) (io.Reader, error) {
	switch r := data.(type) {
	case string:
		return bytes.NewBufferString(r), nil
	case []byte:
		return bytes.NewBuffer(r), nil
	case io.Reader:
		return r, nil
	case goja.ArrayBuffer:
		return bytes.NewBuffer(r.Bytes()), nil
	default:
		return nil, fmt.Errorf("invalid type %T, it needs to be a string, byte array or an ArrayBuffer", data)
	}
}

// ToBytes tries to return a byte slice from compatible types.
func ToBytes(data interface{}) ([]byte, error) {
	switch dt := data.(type) {
	case []byte:
		return dt, nil
	case string:
		return []byte(dt), nil
	case goja.ArrayBuffer:
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
	case goja.ArrayBuffer:
		return string(dt.Bytes()), nil
	default:
		return "", fmt.Errorf("invalid type %T, expected string, []byte or ArrayBuffer", data)
	}
}

// RunWithPanicCatching catches panic and converts into an InterruptError error that should abort a script
func RunWithPanicCatching(logger logrus.FieldLogger, rt *goja.Runtime, fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			gojaStack := rt.CaptureCallStack(20, nil)

			err = &InterruptError{Reason: fmt.Sprintf("a panic occurred during JS execution: %s", r)}
			// TODO figure out how to use PanicLevel without panicing .. this might require changing
			// the logger we use see
			// https://github.com/sirupsen/logrus/issues/1028
			// https://github.com/sirupsen/logrus/issues/993
			b := new(bytes.Buffer)
			for _, s := range gojaStack {
				s.Write(b)
			}
			logger.Error("panic: ", r, "\n", string(debug.Stack()), "\nGoja stack:\n", b.String())
		}
	}()

	return fn()
}
