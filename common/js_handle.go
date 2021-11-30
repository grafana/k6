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
	"fmt"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/runtime"
	"github.com/dop251/goja"
	"github.com/grafana/xk6-browser/api"
	k6common "go.k6.io/k6/js/common"
)

// Ensure BaseJSHandle implements the api.JSHandle interface.
var _ api.JSHandle = &BaseJSHandle{}

// BaseJSHandle represents a JS object in an execution context
type BaseJSHandle struct {
	ctx          context.Context
	logger       *Logger
	session      *Session
	execCtx      *ExecutionContext
	remoteObject *runtime.RemoteObject
	disposed     bool
}

// NewJSHandle creates a new JS handle referencing a remote object
func NewJSHandle(
	ctx context.Context, session *Session, execCtx *ExecutionContext,
	frame *Frame, remoteObject *runtime.RemoteObject, logger *Logger,
) api.JSHandle {
	eh := &BaseJSHandle{
		ctx:          ctx,
		session:      session,
		execCtx:      execCtx,
		remoteObject: remoteObject,
		disposed:     false,
		logger:       logger,
	}

	if remoteObject.Subtype == "node" && execCtx.Frame() != nil {
		return &ElementHandle{
			BaseJSHandle: *eh,
			frame:        frame,
		}
	}

	return eh
}

// AsElement returns an element handle if this JSHandle is a reference to a JS HTML element
func (h *BaseJSHandle) AsElement() api.ElementHandle {
	return nil
}

// Dispose releases the remote object
func (h *BaseJSHandle) Dispose() {
	rt := k6common.GetRuntime(h.ctx)
	if h.disposed {
		return
	}

	h.disposed = true
	if h.remoteObject.ObjectID == "" {
		return
	}

	action := runtime.ReleaseObject(h.remoteObject.ObjectID)
	if err := action.Do(cdp.WithExecutor(h.ctx, h.session)); err != nil {
		k6common.Throw(rt, fmt.Errorf("unable to dispose element %T: %w", action, err))
	}
}

// Evaluate will evaluate provided page function within an execution context
func (h *BaseJSHandle) Evaluate(pageFunc goja.Value, args ...goja.Value) interface{} {
	rt := k6common.GetRuntime(h.ctx)
	args = append([]goja.Value{rt.ToValue(h)}, args...)
	res, err := h.execCtx.Evaluate(h.ctx, pageFunc, args...)
	if err != nil {
		k6common.Throw(rt, err)
	}
	return res
}

// EvaluateHandle will evaluate provided page function within an execution context
func (h *BaseJSHandle) EvaluateHandle(pageFunc goja.Value, args ...goja.Value) api.JSHandle {
	rt := k6common.GetRuntime(h.ctx)
	args = append([]goja.Value{rt.ToValue(h)}, args...)
	res, err := h.execCtx.EvaluateHandle(h.ctx, pageFunc, args...)
	if err != nil {
		k6common.Throw(rt, err)
	}
	return res
}

// GetProperties retreives the JS handle's properties
func (h *BaseJSHandle) GetProperties() map[string]api.JSHandle {
	rt := k6common.GetRuntime(h.ctx)

	var result []*runtime.PropertyDescriptor
	var err error

	action := runtime.GetProperties(h.remoteObject.ObjectID).
		WithOwnProperties(true)
	if result, _, _, _, err = action.Do(cdp.WithExecutor(h.ctx, h.session)); err != nil {
		k6common.Throw(rt, fmt.Errorf("unable to get properties for JS handle %T: %w", action, err))
	}

	props := make(map[string]api.JSHandle, len(result))
	for i := 0; i < len(result); i++ {
		if !result[i].Enumerable {
			continue
		}
		props[result[i].Name] = NewJSHandle(
			h.ctx, h.session, h.execCtx, h.execCtx.Frame(), result[i].Value, h.logger)
	}
	return props
}

// GetProperty retreves a single property of the JS handle
func (h *BaseJSHandle) GetProperty(propertyName string) api.JSHandle {
	return nil
}

// JSONValue returns a JSON version of this JS handle
func (h *BaseJSHandle) JSONValue() goja.Value {
	rt := k6common.GetRuntime(h.ctx)
	if h.remoteObject.ObjectID != "" {
		var result *runtime.RemoteObject
		var err error
		action := runtime.CallFunctionOn("function() { return this; }").
			WithReturnByValue(true).
			WithAwaitPromise(true).
			WithObjectID(h.remoteObject.ObjectID)
		if result, _, err = action.Do(cdp.WithExecutor(h.ctx, h.session)); err != nil {
			k6common.Throw(rt, fmt.Errorf("unable to get properties for JS handle %T: %w", action, err))
		}
		res, err := valueFromRemoteObject(h.ctx, result)
		if err != nil {
			k6common.Throw(rt, fmt.Errorf("unable to extract value from remote object: %w", err))
		}
		return res
	}
	res, err := valueFromRemoteObject(h.ctx, h.remoteObject)
	if err != nil {
		k6common.Throw(rt, fmt.Errorf("unable to extract value from remote object: %w", err))
	}
	return res
}
