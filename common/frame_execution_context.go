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
	"context"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/runtime"
	"github.com/dop251/goja"
	"github.com/grafana/xk6-browser/api"
)

// frameExecutionContext represents a JS execution context that belongs to Frame.
type frameExecutionContext interface {
	// adoptBackendNodeId adopts specified backend node into this execution
	// context from another execution context.
	adoptBackendNodeId(backendNodeID cdp.BackendNodeID) (*ElementHandle, error)

	// adoptElementHandle adopts the specified element handle into this
	// execution context from another execution context.
	adoptElementHandle(elementHandle *ElementHandle) (*ElementHandle, error)

	// evaluate will evaluate provided callable within this execution
	// context and return by value or handle.
	evaluate(
		apiCtx context.Context,
		forceCallable bool, returnByValue bool,
		pageFunc goja.Value, args ...goja.Value,
	) (res interface{}, err error)

	// getInjectedScript returns a JS handle to the injected script of helper
	// functions.
	getInjectedScript(apiCtx context.Context) (api.JSHandle, error)

	// Evaluate will evaluate provided page function within this execution
	// context.
	Evaluate(
		apiCtx context.Context,
		pageFunc goja.Value, args ...goja.Value,
	) (interface{}, error)

	// EvaluateHandle will evaluate provided page function within this
	// execution context.
	EvaluateHandle(
		apiCtx context.Context,
		pageFunc goja.Value, args ...goja.Value,
	) (api.JSHandle, error)

	// Frame returns the frame that this execution context belongs to.
	Frame() *Frame

	// id returns the CDP runtime ID of this execution context.
	ID() runtime.ExecutionContextID
}
