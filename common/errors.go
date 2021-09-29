/*
 *
 * xk6-browser - a browser automation extension for k6
 * Copyright (C) 2021 Load Impact
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
	"fmt"

	"github.com/chromedp/cdproto/runtime"
)

// Error is a common package error.
type Error string

// Error satisfies the builtin error interface.
func (e Error) Error() string {
	return string(e)
}

// Error types.
const (
	ErrUnexpectedRemoteObjectWithID Error = "cannot extract value when remote object ID is given"
	ErrChannelClosed                Error = "channel closed"
	ErrFrameDetached                Error = "frame detached"
	ErrJSHandleDisposed             Error = "JS handle is disposed"
	ErrTargetCrashed                Error = "Target has crashed"
	ErrTimedOut                     Error = "timed out"
	ErrWebsocketClosed              Error = "websocket closed"
	ErrWrongExecutionContext        Error = "JS handles can be evaluated only in the context they were created"
)

type BigIntParseError struct {
	err error
}

// Error satisfies the builtin error interface.
func (e BigIntParseError) Error() string {
	return fmt.Sprintf("unable to parse bigint: %v", e.err)
}

// Is satisfies the builtin error Is interface.
func (e BigIntParseError) Is(target error) bool {
	switch target.(type) {
	case BigIntParseError:
		return true
	}
	return false
}

// Unwrap satisfies the builtin error Unwrap interface.
func (e BigIntParseError) Unwrap() error {
	return e.err
}

type UnserializableValueError struct {
	UnserializableValue runtime.UnserializableValue
}

// Error satisfies the builtin error interface.
func (e UnserializableValueError) Error() string {
	return fmt.Sprintf("unsupported unserializable value: %s", e.UnserializableValue)
}
