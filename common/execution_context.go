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
	"regexp"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/runtime"
	"github.com/dop251/goja"
	"github.com/grafana/xk6-browser/api"
	k6common "go.k6.io/k6/js/common"
)

const evaluationScriptURL = "__xk6_browser_evaluation_script__"

var /* const */ sourceURLRegex = regexp.MustCompile(`^(?s)[\040\t]*//[@#] sourceURL=\s*(\S*?)\s*$`)

// ExecutionContext represents a JS execution context
type ExecutionContext struct {
	ctx            context.Context
	logger         *Logger
	session        *Session
	frame          *Frame
	id             runtime.ExecutionContextID
	injectedScript api.JSHandle
}

// NewExecutionContext creates a new JS execution context
func NewExecutionContext(
	ctx context.Context, session *Session, frame *Frame,
	id runtime.ExecutionContextID, logger *Logger,
) *ExecutionContext {
	return &ExecutionContext{
		ctx:            ctx,
		session:        session,
		frame:          frame,
		id:             id,
		injectedScript: nil,
		logger:         logger,
	}
}

// Adopts specified backend node into this execution context from another execution context
func (e *ExecutionContext) adoptBackendNodeId(backendNodeID cdp.BackendNodeID) (*ElementHandle, error) {
	var remoteObj *runtime.RemoteObject
	var err error

	action := dom.ResolveNode().
		WithBackendNodeID(backendNodeID).
		WithExecutionContextID(e.id)
	if remoteObj, err = action.Do(cdp.WithExecutor(e.ctx, e.session)); err != nil {
		return nil, fmt.Errorf("unable to get DOM node: %w", err)
	}

	return NewJSHandle(e.ctx, e.session, e, e.frame, remoteObj, e.logger).AsElement().(*ElementHandle), nil
}

// Adopts the specified element handle into this execution context from another execution context
func (e *ExecutionContext) adoptElementHandle(elementHandle *ElementHandle) (*ElementHandle, error) {
	if elementHandle.execCtx == e {
		panic("Cannot adopt handle that already belongs to this execution context")
	}
	if e.frame == nil {
		panic("Cannot adopt handle without frame owner")
	}

	var node *cdp.Node
	var err error

	action := dom.DescribeNode().WithObjectID(elementHandle.remoteObject.ObjectID)
	if node, err = action.Do(cdp.WithExecutor(e.ctx, e.session)); err != nil {
		return nil, fmt.Errorf("unable to get DOM node: %w", err)
	}

	return e.adoptBackendNodeId(node.BackendNodeID)
}

// evaluate will evaluate provided callable within this execution context and return by value or handle
func (e *ExecutionContext) evaluate(apiCtx context.Context, forceCallable bool, returnByValue bool, pageFunc goja.Value, args ...goja.Value) (res interface{}, err error) {
	suffix := `//# sourceURL=` + evaluationScriptURL

	isCallable := forceCallable
	if !forceCallable {
		_, isCallable = goja.AssertFunction(pageFunc.(goja.Value))
	}

	if !isCallable {
		expression := pageFunc.ToString().String()
		expressionWithSourceURL := expression
		if !sourceURLRegex.Match([]byte(expression)) {
			expressionWithSourceURL = expression + "\n" + suffix
		}

		var remoteObject *runtime.RemoteObject
		var exceptionDetails *runtime.ExceptionDetails
		action := runtime.Evaluate(expressionWithSourceURL).
			WithContextID(e.id).
			WithReturnByValue(returnByValue).
			WithAwaitPromise(true).
			WithUserGesture(true)
		if remoteObject, exceptionDetails, err = action.Do(cdp.WithExecutor(apiCtx, e.session)); err != nil {
			return nil, fmt.Errorf("unable to evaluate expression: %w", err)
		}
		if exceptionDetails != nil {
			return nil, fmt.Errorf("unable to evaluate expression: %w", exceptionDetails)
		}
		if remoteObject != nil {
			if returnByValue {
				res, err = valueFromRemoteObject(apiCtx, remoteObject)
				if err != nil {
					return nil, fmt.Errorf("unable to extract value from remote object: %w", err)
				}
			} else if remoteObject.ObjectID != "" {
				// Note: we don't use the passed in apiCtx here as it could be tied to a timeout
				res = NewJSHandle(e.ctx, e.session, e, e.frame, remoteObject, e.logger)
			}
		}
	} else {
		functionText := pageFunc.ToString().String()
		var arguments []*runtime.CallArgument
		for _, arg := range args {
			result, err := convertArgument(apiCtx, e, arg)
			if err != nil {
				return nil, fmt.Errorf("unable to convert argument: %w", err)
			}
			arguments = append(arguments, result)
		}

		var remoteObject *runtime.RemoteObject
		var exceptionDetails *runtime.ExceptionDetails
		action := runtime.CallFunctionOn(functionText + "\n" + suffix + "\n").
			WithArguments(arguments).
			WithExecutionContextID(e.id).
			WithReturnByValue(returnByValue).
			WithAwaitPromise(true).
			WithUserGesture(true)
		if remoteObject, exceptionDetails, err = action.Do(cdp.WithExecutor(apiCtx, e.session)); err != nil {
			return nil, fmt.Errorf("unable to evaluate expression: %w", err)
		}
		if exceptionDetails != nil {
			return nil, fmt.Errorf("unable to evaluate expression: %w", exceptionDetails)
		}
		if remoteObject != nil {
			if returnByValue {
				res, err = valueFromRemoteObject(apiCtx, remoteObject)
				if err != nil {
					return nil, fmt.Errorf("unable to extract value from remote object: %w", err)
				}
			} else if remoteObject.ObjectID != "" {
				// Note: we don't use the passed in apiCtx here as it could be tied to a timeout
				res = NewJSHandle(e.ctx, e.session, e, e.frame, remoteObject, e.logger)
			}
		}
	}

	return res, err
}

// getInjectedScript returns a JS handle to the injected script of helper functions
func (e *ExecutionContext) getInjectedScript(apiCtx context.Context) (api.JSHandle, error) {
	if e.injectedScript == nil {
		rt := k6common.GetRuntime(e.ctx)
		suffix := `//# sourceURL=` + evaluationScriptURL
		source := fmt.Sprintf(`(() => {%s; return new InjectedScript();})()`, injectedScriptSource)
		expression := source
		expressionWithSourceURL := expression
		if !sourceURLRegex.Match([]byte(expression)) {
			expressionWithSourceURL = expression + "\n" + suffix
		}

		handle, err := e.evaluate(apiCtx, false, false, rt.ToValue(expressionWithSourceURL))
		if handle == nil || err != nil {
			return nil, err
		}
		e.injectedScript = handle.(api.JSHandle)
	}
	return e.injectedScript, nil
}

// Evaluate will evaluate provided page function within this execution context
func (e *ExecutionContext) Evaluate(apiCtx context.Context, pageFunc goja.Value, args ...goja.Value) (interface{}, error) {
	return e.evaluate(apiCtx, true, true, pageFunc, args...)
}

// EvaluateHandle will evaluate provided page function within this execution context
func (e *ExecutionContext) EvaluateHandle(apiCtx context.Context, pageFunc goja.Value, args ...goja.Value) (api.JSHandle, error) {
	res, err := e.evaluate(apiCtx, true, false, pageFunc, args...)
	if err != nil {
		return nil, err
	}
	return res.(api.JSHandle), nil
}

// Frame returns the frame that this execution context belongs to
func (e *ExecutionContext) Frame() *Frame {
	return e.frame
}

// ID returns the CDP runtime ID of this execution context.
func (e *ExecutionContext) ID() runtime.ExecutionContextID {
	return e.id
}
