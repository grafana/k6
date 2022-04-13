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
	_ "embed"
	"errors"
	"fmt"
	"regexp"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
	"github.com/dop251/goja"
	k6modules "go.k6.io/k6/js/modules"

	"github.com/grafana/xk6-browser/api"
)

const evaluationScriptURL = "__xk6_browser_evaluation_script__"

var sourceURLRegex = regexp.MustCompile(`^(?s)[\040\t]*//[@#] sourceURL=\s*(\S*?)\s*$`)

type executionWorld string

const (
	mainWorld    executionWorld = "main"
	utilityWorld executionWorld = "utility"
)

func (ew executionWorld) valid() bool {
	return ew == mainWorld || ew == utilityWorld
}

type evalOptions struct {
	forceCallable, returnByValue bool
}

func (ea evalOptions) String() string {
	return fmt.Sprintf("forceCallable:%t returnByValue:%t", ea.forceCallable, ea.returnByValue)
}

// ExecutionContext represents a JS execution context.
type ExecutionContext struct {
	ctx            context.Context
	logger         *Logger
	session        session
	frame          *Frame
	id             runtime.ExecutionContextID
	injectedScript api.JSHandle
	vu             k6modules.VU

	// Used for logging
	sid  target.SessionID // Session ID
	stid cdp.FrameID      // Session TargetID
	fid  cdp.FrameID      // Frame ID
	furl string           // Frame URL
}

// NewExecutionContext creates a new JS execution context.
func NewExecutionContext(
	ctx context.Context, s session, f *Frame, id runtime.ExecutionContextID, l *Logger,
) *ExecutionContext {
	e := &ExecutionContext{
		ctx:            ctx,
		session:        s,
		frame:          f,
		id:             id,
		injectedScript: nil,
		vu:             GetVU(ctx),
		logger:         l,
	}
	if s != nil {
		e.sid = s.ID()
		e.stid = cdp.FrameID(s.TargetID())
	}
	if f != nil {
		e.fid = cdp.FrameID(f.ID())
		e.furl = f.URL()
	}
	l.Debugf(
		"NewExecutionContext",
		"sid:%s stid:%s fid:%s ectxid:%d furl:%q",
		e.sid, e.stid, e.fid, id, e.furl)

	return e
}

// Adopts specified backend node into this execution context from another execution context.
func (e *ExecutionContext) adoptBackendNodeID(backendNodeID cdp.BackendNodeID) (*ElementHandle, error) {
	e.logger.Debugf(
		"ExecutionContext:adoptBackendNodeID",
		"sid:%s stid:%s fid:%s ectxid:%d furl:%q bnid:%d",
		e.sid, e.stid, e.fid, e.id, e.furl, backendNodeID)

	var (
		remoteObj *runtime.RemoteObject
		err       error
	)

	action := dom.ResolveNode().
		WithBackendNodeID(backendNodeID).
		WithExecutionContextID(e.id)

	if remoteObj, err = action.Do(cdp.WithExecutor(e.ctx, e.session)); err != nil {
		return nil, fmt.Errorf("cannot resolve DOM node: %w", err)
	}

	return NewJSHandle(e.ctx, e.session, e, e.frame, remoteObj, e.logger).AsElement().(*ElementHandle), nil
}

// Adopts the specified element handle into this execution context from another execution context.
func (e *ExecutionContext) adoptElementHandle(eh *ElementHandle) (*ElementHandle, error) {
	var (
		efid cdp.FrameID
		esid target.SessionID
	)
	if eh.frame != nil {
		efid = cdp.FrameID(eh.frame.ID())
	}
	if eh.session != nil {
		esid = eh.session.ID()
	}
	e.logger.Debugf(
		"ExecutionContext:adoptElementHandle",
		"sid:%s stid:%s fid:%s ectxid:%d furl:%q ehtid:%s ehsid:%s",
		e.sid, e.stid, e.fid, e.id, e.furl,
		efid, esid)

	if eh.execCtx == e {
		panic("Cannot adopt handle that already belongs to this execution context")
	}
	if e.frame == nil {
		panic("Cannot adopt handle without frame owner")
	}

	var node *cdp.Node
	var err error

	action := dom.DescribeNode().WithObjectID(eh.remoteObject.ObjectID)
	if node, err = action.Do(cdp.WithExecutor(e.ctx, e.session)); err != nil {
		return nil, fmt.Errorf("cannot describe DOM node: %w", err)
	}

	return e.adoptBackendNodeID(node.BackendNodeID)
}

// eval will evaluate provided callable within this execution context and return by value or handle.
func (e *ExecutionContext) eval(
	apiCtx context.Context, opts evalOptions, js string, args ...interface{},
) (interface{}, error) {
	e.logger.Debugf(
		"ExecutionContext:eval",
		"sid:%s stid:%s fid:%s ectxid:%d furl:%q %s",
		e.sid, e.stid, e.fid, e.id, e.furl, opts)

	suffix := `//# sourceURL=` + evaluationScriptURL

	var action interface {
		Do(context.Context) (*runtime.RemoteObject, *runtime.ExceptionDetails, error)
	}

	if !opts.forceCallable {
		if !sourceURLRegex.Match([]byte(js)) {
			js += "\n" + suffix
		}

		action = runtime.Evaluate(js).
			WithContextID(e.id).
			WithReturnByValue(opts.returnByValue).
			WithAwaitPromise(true).
			WithUserGesture(true)
	} else {
		var arguments []*runtime.CallArgument
		for _, arg := range args {
			result, err := convertArgument(apiCtx, e, arg)
			if err != nil {
				return nil, fmt.Errorf("cannot convert argument (%q) "+
					"in execution context (%d) in frame (%v): %w",
					arg, e.id, e.Frame().ID(), err)
			}
			arguments = append(arguments, result)
		}

		js += "\n" + suffix + "\n"
		action = runtime.CallFunctionOn(js).
			WithArguments(arguments).
			WithExecutionContextID(e.id).
			WithReturnByValue(opts.returnByValue).
			WithAwaitPromise(true).
			WithUserGesture(true)
	}

	var (
		remoteObject     *runtime.RemoteObject
		exceptionDetails *runtime.ExceptionDetails
		err              error
	)
	if remoteObject, exceptionDetails, err = action.Do(cdp.WithExecutor(apiCtx, e.session)); err != nil {
		return nil, fmt.Errorf("cannot call function on expression (%q) "+
			"in execution context (%d) in frame (%v) with session (%v): %w",
			js, e.id, e.Frame().ID(), e.session.ID(), err)
	}
	if exceptionDetails != nil {
		return nil, fmt.Errorf("cannot call function on expression (%q) "+
			"in execution context (%d) in frame (%v) with session (%v): %s",
			js, e.id, e.Frame().ID(), e.session.ID(),
			parseExceptionDetails(exceptionDetails))
	}
	var res interface{}
	if remoteObject == nil {
		e.logger.Debugf(
			"ExecutionContext:eval",
			"sid:%s stid:%s fid:%s ectxid:%d furl:%q remoteObject is nil",
			e.sid, e.stid, e.fid, e.id, e.furl)
		return res, nil
	}

	if opts.returnByValue {
		res, err = valueFromRemoteObject(apiCtx, remoteObject)
		if err != nil {
			return nil, fmt.Errorf("cannot extract value from remote object (%s) "+
				"using (%s) in execution context (%d) in frame (%v): %w",
				remoteObject.ObjectID, js, e.id, e.Frame().ID(), err)
		}
	} else if remoteObject.ObjectID != "" {
		// Note: we don't use the passed in apiCtx here as it could be tied to a timeout
		res = NewJSHandle(e.ctx, e.session, e, e.frame, remoteObject, e.logger)
	}

	return res, nil
}

// Based on: https://github.com/microsoft/playwright/blob/master/src/server/injected/injectedScript.ts
//go:embed js/injected_script.js
var injectedScriptSource string

// getInjectedScript returns a JS handle to the injected script of helper functions.
func (e *ExecutionContext) getInjectedScript(apiCtx context.Context) (api.JSHandle, error) {
	e.logger.Debugf(
		"ExecutionContext:getInjectedScript",
		"sid:%s stid:%s fid:%s ectxid:%d efurl:%s",
		e.sid, e.stid, e.fid, e.id, e.furl)

	if e.injectedScript != nil {
		return e.injectedScript, nil
	}

	var (
		suffix                  = `//# sourceURL=` + evaluationScriptURL
		source                  = fmt.Sprintf(`(() => {%s; return new InjectedScript();})()`, injectedScriptSource)
		expression              = source
		expressionWithSourceURL = expression
	)
	if !sourceURLRegex.Match([]byte(expression)) {
		expressionWithSourceURL = expression + "\n" + suffix
	}
	handle, err := e.eval(
		apiCtx,
		evalOptions{forceCallable: false, returnByValue: false},
		expressionWithSourceURL,
	)
	if err != nil {
		return nil, fmt.Errorf("cannot get injected script: %w", err)
	}
	if handle == nil {
		return nil, errors.New("cannot get injected script: handle is nil")
	}
	injectedScript, ok := handle.(api.JSHandle)
	if !ok {
		return nil, fmt.Errorf("cannot get injected script: %w", ErrJSHandleInvalid)
	}
	e.injectedScript = injectedScript

	return e.injectedScript, nil
}

// Eval will evaluate provided page function within this execution context.
func (e *ExecutionContext) Eval(
	apiCtx context.Context, js goja.Value, args ...goja.Value,
) (interface{}, error) {
	opts := evalOptions{
		forceCallable: true,
		returnByValue: true,
	}
	evalArgs := make([]interface{}, 0, len(args))
	for _, a := range args {
		evalArgs = append(evalArgs, a.Export())
	}
	return e.eval(apiCtx, opts, js.ToString().String(), evalArgs...)
}

// EvalHandle will evaluate provided page function within this execution context.
func (e *ExecutionContext) EvalHandle(
	apiCtx context.Context, js goja.Value, args ...goja.Value,
) (api.JSHandle, error) {
	opts := evalOptions{
		forceCallable: true,
		returnByValue: false,
	}
	evalArgs := make([]interface{}, 0, len(args))
	for _, a := range args {
		evalArgs = append(evalArgs, a.Export())
	}
	res, err := e.eval(apiCtx, opts, js.ToString().String(), evalArgs...)
	if err != nil {
		return nil, err
	}
	return res.(api.JSHandle), nil
}

// Frame returns the frame that this execution context belongs to.
func (e *ExecutionContext) Frame() *Frame {
	return e.frame
}

// ID returns the CDP runtime ID of this execution context.
func (e *ExecutionContext) ID() runtime.ExecutionContextID {
	return e.id
}
