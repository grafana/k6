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

	"github.com/grafana/xk6-browser/api"
	"github.com/grafana/xk6-browser/k6ext"
	"github.com/grafana/xk6-browser/log"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/runtime"
	"github.com/dop251/goja"
)

type jsHandle interface {
	api.JSHandle
	dispose() error
	getProperties() (map[string]jsHandle, error)
}

var _ jsHandle = &BaseJSHandle{}

// BaseJSHandle represents a JS object in an execution context.
type BaseJSHandle struct {
	ctx          context.Context
	logger       *log.Logger
	session      session
	execCtx      *ExecutionContext
	remoteObject *runtime.RemoteObject
	disposed     bool
}

// NewJSHandle creates a new JS handle referencing a remote object.
func NewJSHandle(
	ctx context.Context,
	s session,
	ectx *ExecutionContext,
	f *Frame,
	ro *runtime.RemoteObject,
	l *log.Logger,
) jsHandle {
	eh := &BaseJSHandle{
		ctx:          ctx,
		session:      s,
		execCtx:      ectx,
		remoteObject: ro,
		disposed:     false,
		logger:       l,
	}

	if ro.Subtype == "node" && ectx.Frame() != nil {
		return &ElementHandle{
			BaseJSHandle: *eh,
			frame:        f,
		}
	}

	return eh
}

// AsElement returns an element handle if this JSHandle is a reference to a JS HTML element.
func (h *BaseJSHandle) AsElement() api.ElementHandle {
	return nil
}

// Dispose releases the remote object.
func (h *BaseJSHandle) Dispose() {
	if err := h.dispose(); err != nil {
		k6ext.Panic(h.ctx, "dispose: %w", err)
	}
}

// dispose is like Dispose, but does not panic.
func (h *BaseJSHandle) dispose() error {
	if h.disposed {
		return nil
	}
	h.disposed = true
	if h.remoteObject.ObjectID == "" {
		return nil
	}
	act := runtime.ReleaseObject(h.remoteObject.ObjectID)
	if err := act.Do(cdp.WithExecutor(h.ctx, h.session)); err != nil {
		return fmt.Errorf("disposing element with ID %s: %w",
			h.remoteObject.ObjectID, err)
	}

	return nil
}

// Evaluate will evaluate provided page function within an execution context.
func (h *BaseJSHandle) Evaluate(pageFunc goja.Value, args ...goja.Value) interface{} {
	rt := h.execCtx.vu.Runtime()
	args = append([]goja.Value{rt.ToValue(h)}, args...)
	res, err := h.execCtx.Eval(h.ctx, pageFunc, args...)
	if err != nil {
		k6ext.Panic(h.ctx, "%w", err)
	}
	return res
}

// EvaluateHandle will evaluate provided page function within an execution context.
func (h *BaseJSHandle) EvaluateHandle(pageFunc goja.Value, args ...goja.Value) api.JSHandle {
	rt := h.execCtx.vu.Runtime()
	args = append([]goja.Value{rt.ToValue(h)}, args...)
	res, err := h.execCtx.EvalHandle(h.ctx, pageFunc, args...)
	if err != nil {
		k6ext.Panic(h.ctx, "%w", err)
	}
	return res
}

// GetProperties retreives the JS handle's properties.
func (h *BaseJSHandle) GetProperties() map[string]api.JSHandle {
	handles, err := h.getProperties()
	if err != nil {
		k6ext.Panic(h.ctx, "getProperties: %w", err)
	}

	jsHandles := make(map[string]api.JSHandle, len(handles))
	for k, v := range handles {
		jsHandles[k] = v
	}

	return jsHandles
}

// getProperties is like GetProperties, but does not panic.
func (h *BaseJSHandle) getProperties() (map[string]jsHandle, error) {
	act := runtime.GetProperties(h.remoteObject.ObjectID).WithOwnProperties(true)
	result, _, _, _, err := act.Do(cdp.WithExecutor(h.ctx, h.session)) //nolint:dogsled
	if err != nil {
		return nil, fmt.Errorf("getting properties for element with ID %s: %w",
			h.remoteObject.ObjectID, err)
	}

	props := make(map[string]jsHandle, len(result))
	for _, r := range result {
		if !r.Enumerable {
			continue
		}
		props[r.Name] = NewJSHandle(h.ctx, h.session, h.execCtx, h.execCtx.Frame(), r.Value, h.logger)
	}

	return props, nil
}

// GetProperty retreves a single property of the JS handle.
func (h *BaseJSHandle) GetProperty(propertyName string) api.JSHandle {
	return nil
}

// JSONValue returns a JSON version of this JS handle.
func (h *BaseJSHandle) JSONValue() goja.Value {
	if h.remoteObject.ObjectID != "" {
		var result *runtime.RemoteObject
		var err error
		action := runtime.CallFunctionOn("function() { return this; }").
			WithReturnByValue(true).
			WithAwaitPromise(true).
			WithObjectID(h.remoteObject.ObjectID)
		if result, _, err = action.Do(cdp.WithExecutor(h.ctx, h.session)); err != nil {
			k6ext.Panic(h.ctx, "getting properties for JS handle: %w", err)
		}
		res, err := valueFromRemoteObject(h.ctx, result)
		if err != nil {
			k6ext.Panic(h.ctx, "extracting value from remote object: %w", err)
		}
		return res
	}
	res, err := valueFromRemoteObject(h.ctx, h.remoteObject)
	if err != nil {
		k6ext.Panic(h.ctx, "extracting value from remote object: %w", err)
	}
	return res
}

// ObjectID returns the remote object ID.
func (h *BaseJSHandle) ObjectID() runtime.RemoteObjectID {
	return h.remoteObject.ObjectID
}
