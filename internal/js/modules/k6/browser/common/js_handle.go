package common

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.k6.io/k6/internal/js/modules/k6/browser/log"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/runtime"
)

// JSHandleAPI is the interface of an in-page JS object.
//
// TODO: Find a way to move this to a concrete type. It's too difficult to
// do that right now because of the tests and the way we're using the
// JSHandleAPI interface.
type JSHandleAPI interface {
	AsElement() *ElementHandle
	Dispose() error
	Evaluate(pageFunc string, args ...any) (any, error)
	EvaluateHandle(pageFunc string, args ...any) (JSHandleAPI, error)
	GetProperties() (map[string]JSHandleAPI, error)
	JSONValue() (string, error)
	ObjectID() runtime.RemoteObjectID
}

type jsHandle interface {
	JSHandleAPI
	dispose() error
	getProperties() (map[string]jsHandle, error)
}

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
func (h *BaseJSHandle) AsElement() *ElementHandle {
	return nil
}

// Dispose releases the remote object.
func (h *BaseJSHandle) Dispose() error {
	err := h.dispose()
	if err == nil { // no error
		return nil
	}

	// We do not want to return an error when the error is a closed
	// context. The reason the context would be closed is due to the
	// iteration ending and therefore the associated browser and its assets
	// will be automatically deleted.
	if errors.Is(err, context.Canceled) {
		h.logger.Debugf("BaseJSHandle:Dispose", "%v", err)
		return nil
	}
	// The following error indicates that the object we're trying to release
	// cannot be found, which would mean that the object has already been
	// removed/deleted. This can occur when a navigation occurs, usually when
	// a page contains an iframe.
	if strings.Contains(err.Error(), "Cannot find context with specified id") {
		h.logger.Debugf("BaseJSHandle:Dispose", "%v", err)
		return nil
	}

	return fmt.Errorf("disposing element with ID %s: %w", h.remoteObject.ObjectID, err)
}

// dispose sends a command to the browser to release the remote object.
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
func (h *BaseJSHandle) Evaluate(pageFunc string, args ...any) (any, error) {
	args = append([]any{h}, args...)
	res, err := h.execCtx.Eval(h.ctx, pageFunc, args...)
	if err != nil {
		return nil, fmt.Errorf("evaluating element: %w", err)
	}

	return res, nil
}

// EvaluateHandle will evaluate provided page function within an execution context.
func (h *BaseJSHandle) EvaluateHandle(pageFunc string, args ...any) (JSHandleAPI, error) {
	args = append([]any{h}, args...)
	eh, err := h.execCtx.EvalHandle(h.ctx, pageFunc, args...)
	if err != nil {
		return nil, fmt.Errorf("evaluating handle for element: %w", err)
	}

	return eh, nil
}

// GetProperties retreives the JS handle's properties.
func (h *BaseJSHandle) GetProperties() (map[string]JSHandleAPI, error) {
	handles, err := h.getProperties()
	if err != nil {
		return nil, err
	}

	jsHandles := make(map[string]JSHandleAPI, len(handles))
	for k, v := range handles {
		jsHandles[k] = v
	}

	return jsHandles, nil
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

// JSONValue returns a JSON version of this JS handle.
func (h *BaseJSHandle) JSONValue() (string, error) {
	remoteObject := h.remoteObject
	if remoteObject.ObjectID != "" {
		var err error
		action := runtime.CallFunctionOn("function() { return this; }").
			WithReturnByValue(true).
			WithAwaitPromise(true).
			WithObjectID(h.remoteObject.ObjectID)
		if remoteObject, _, err = action.Do(cdp.WithExecutor(h.ctx, h.session)); err != nil {
			return "", fmt.Errorf("retrieving json value: %w", err)
		}
	}

	res, err := parseConsoleRemoteObject(h.logger, remoteObject)
	if err != nil {
		return "", fmt.Errorf("extracting json value (remote object id: %v): %w", remoteObject.ObjectID, err)
	}

	return res, nil
}

// ObjectID returns the remote object ID.
func (h *BaseJSHandle) ObjectID() runtime.RemoteObjectID {
	return h.remoteObject.ObjectID
}
