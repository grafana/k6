/**
 * Copyright (c) Microsoft Corporation.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"context"

	"errors"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/emulation"
	cdppage "github.com/chromedp/cdproto/page"
	"github.com/dop251/goja"
	"github.com/grafana/xk6-browser/api"
	k6common "go.k6.io/k6/js/common"
)

// Ensure ElementHandle implements the api.ElementHandle and api.JSHandle interfaces.
var _ api.ElementHandle = &ElementHandle{}
var _ api.JSHandle = &ElementHandle{}

type ElementHandleActionFn func(context.Context, *ElementHandle) (interface{}, error)
type ElementHandlePointerActionFn func(context.Context, *ElementHandle, *Position) (interface{}, error)

func elementHandleActionFn(h *ElementHandle, states []string, fn ElementHandleActionFn, force, noWaitAfter bool, timeout time.Duration) func(apiCtx context.Context, resultCh chan interface{}, errCh chan error) {
	// All or a subset of the following actionability checks are made before performing the actual action:
	// 1. Attached to DOM
	// 2. Visible
	// 3. Stable
	// 4. Enabled

	return func(apiCtx context.Context, resultCh chan interface{}, errCh chan error) {
		var result interface{}
		var err error

		// Check if we should run actionability checks
		if !force {
			_, err = h.waitForElementState(apiCtx, states, timeout)
			if err != nil {
				errCh <- err
				return
			}
		}

		b := NewBarrier()
		h.frame.manager.addBarrier(b)
		defer h.frame.manager.removeBarrier(b)

		result, err = fn(apiCtx, h)
		if err != nil {
			errCh <- err
			return
		}

		// Do we need to wait for navigation to happen
		if !noWaitAfter {
			if err := b.Wait(apiCtx); err != nil {
				errCh <- err
			}
		}

		resultCh <- result
	}
}

func elementHandlePointerActionFn(h *ElementHandle, checkEnabled bool, fn ElementHandlePointerActionFn, opts *ElementHandleBasePointerOptions) func(apiCtx context.Context, resultCh chan interface{}, errCh chan error) {
	// All or a subset of the following actionability checks are made before performing the actual action:
	// 1. Attached to DOM
	// 2. Visible
	// 3. Stable
	// 4. Enabled
	// 5. Receives events

	return func(apiCtx context.Context, resultCh chan interface{}, errCh chan error) {
		var result interface{}
		var err error

		// Check if we should run actionability checks
		if !opts.Force {
			states := []string{"visible", "stable"}
			if checkEnabled {
				states = append(states, "enabled")
			}
			_, err = h.waitForElementState(apiCtx, states, opts.Timeout)
			if err != nil {
				errCh <- err
				return
			}
		}

		// Decide position where a mouse down should happen if needed by action
		p := opts.Position

		// Scroll element into view
		var rect *dom.Rect
		if p != nil {
			rect = &dom.Rect{X: p.X, Y: p.Y, Width: 0, Height: 0}
		}
		err = h.scrollRectIntoViewIfNeeded(apiCtx, rect)
		if err != nil {
			errCh <- err
			return
		}

		if p != nil {
			p, err = h.offsetPosition(apiCtx, opts.Position)
			if err != nil {
				errCh <- err
				return
			}
		} else {
			p, err = h.clickablePoint()
			if err != nil {
				errCh <- err
				return
			}
		}

		// Do a final actionability check to see if element can receive events at mouse position in question
		if !opts.Force {
			if ok, localErr := h.checkHitTargetAt(apiCtx, *p); !ok {
				errCh <- localErr
				return
			}
		}

		// Are we only "trialing" the action (ie. running the actionability checks) but not actually performing it
		if !opts.Trial {
			b := NewBarrier()
			h.frame.manager.addBarrier(b)
			defer h.frame.manager.removeBarrier(b)

			result, err = fn(apiCtx, h, p)
			if err != nil {
				errCh <- err
				return
			}

			// Do we need to wait for navigation to happen
			if !opts.NoWaitAfter {
				if err := b.Wait(apiCtx); err != nil {
					errCh <- err
				}
			}
		}

		resultCh <- result
	}
}

// ElementHandle represents a HTML element JS object inside an execution context
type ElementHandle struct {
	BaseJSHandle

	frame *Frame
}

func (h *ElementHandle) boundingBox() (*api.Rect, error) {
	var box *dom.BoxModel
	var err error
	action := dom.GetBoxModel().WithObjectID(h.remoteObject.ObjectID)
	if box, err = action.Do(cdp.WithExecutor(h.ctx, h.session)); err != nil {
		return nil, fmt.Errorf("unable to get box model of DOM node: %w", err)
	}

	quad := box.Border
	x := math.Min(quad[0], math.Min(quad[2], math.Min(quad[4], quad[6])))
	y := math.Min(quad[1], math.Min(quad[3], math.Min(quad[5], quad[7])))
	width := math.Max(quad[0], math.Max(quad[2], math.Max(quad[4], quad[6]))) - x
	height := math.Max(quad[1], math.Max(quad[3], math.Max(quad[5], quad[7]))) - y
	position := h.frame.position()

	return &api.Rect{X: x + position.X, Y: y + position.Y, Width: width, Height: height}, nil
}

func (h *ElementHandle) checkHitTargetAt(apiCtx context.Context, p Position) (bool, error) {
	frame := h.ownerFrame(apiCtx)
	if frame != nil && frame.parentFrame != nil {
		element := h.frame.FrameElement().(*ElementHandle)
		box, err := element.boundingBox()
		if err != nil {
			return false, err
		}
		if box == nil {
			return false, errors.New("unable to get bounding box of element")
		}

		// Translate from viewport coordinates to frame coordinates.
		p.X = p.X - box.X
		p.Y = p.Y - box.Y
	}

	rt := k6common.GetRuntime(h.ctx)
	injected, err := h.execCtx.getInjectedScript(apiCtx)
	if err != nil {
		return false, err
	}
	pageFn := rt.ToValue(`
		(injected, node, point) => {
			return injected.checkHitTargetAt(node, point);
		}
	`)
	result, err := h.execCtx.evaluate(
		apiCtx, true, true, pageFn, []goja.Value{
			rt.ToValue(injected),
			rt.ToValue(h),
			rt.ToValue(p),
		}...)
	if err != nil {
		return false, err
	}

	value := result.(goja.Value)
	switch value.ExportType().Kind() {
	case reflect.String: // Either we're done or an error happened (returned as "error:..." from JS)
		if value.String() == "done" {
			return true, nil
		}
		return false, errorFromDOMError(value.String())
	}
	return true, nil // We got a { hitTargetDescription: ... } result
}

func (h *ElementHandle) checkElementState(apiCtx context.Context, state string) (*bool, error) {
	rt := k6common.GetRuntime(apiCtx)
	injected, err := h.execCtx.getInjectedScript(apiCtx)
	if err != nil {
		return nil, err
	}
	pageFn := rt.ToValue(`
		(injected, node, state) => {
			return injected.checkElementState(node, state);
		}
	`)
	result, err := h.execCtx.evaluate(
		apiCtx, true, true, pageFn, []goja.Value{
			rt.ToValue(injected),
			rt.ToValue(h),
			rt.ToValue(state),
		}...)
	if err != nil {
		return nil, err
	}

	value := result.(goja.Value)
	switch value.ExportType().Kind() {
	case reflect.String: // An error happened (returned as "error:..." from JS)
		return nil, errorFromDOMError(value.String())
	case reflect.Bool:
		returnVal := new(bool)
		*returnVal = value.ToBoolean()
		return returnVal, nil
	}
	return nil, fmt.Errorf("unable to check state %q of element: %q", state, reflect.TypeOf(result))
}

func (h *ElementHandle) click(p *Position, opts *MouseClickOptions) error {
	return h.frame.page.Mouse.click(p.X, p.Y, opts)
}

func (h *ElementHandle) clickablePoint() (*Position, error) {
	var quads []dom.Quad
	var layoutViewport *cdppage.LayoutViewport
	var err error

	action := dom.GetContentQuads().
		WithObjectID(h.remoteObject.ObjectID)
	if quads, err = action.Do(cdp.WithExecutor(h.ctx, h.session)); err != nil {
		return nil, fmt.Errorf("unable to request node content quads %T: %w", action, err)
	}
	if len(quads) == 0 {
		return nil, fmt.Errorf("node is either not visible or not an HTMLElement: %w", err)
	}

	action2 := cdppage.GetLayoutMetrics()
	if layoutViewport, _, _, _, _, _, err = action2.Do(cdp.WithExecutor(h.ctx, h.session)); err != nil {
		return nil, fmt.Errorf("unable to get page layout metrics %T: %w", action, err)
	}

	// Filter out quads that have too small area to click into.
	clientWidth := layoutViewport.ClientWidth
	clientHeight := layoutViewport.ClientHeight
	var filteredQuads []dom.Quad
	for _, q := range quads {
		nq := q
		for i := 0; i < len(q); i += 2 {
			nq[i] = math.Min(math.Max(q[i], 0), float64(clientWidth))
			nq[i+1] = math.Min(math.Max(q[i+1], 0), float64(clientHeight))
		}

		// Compute sum of all directed areas of adjacent triangles
		// https://en.wikipedia.org/wiki/Polygon#Area
		var area float64 = 0
		for i := 0; i < len(q); i += 2 {
			p2 := (i + 2) % (len(q) / 2)
			area += (q[i]*q[p2+1] - q[p2]*q[i+1]) / 2
		}

		if math.Abs(area) > 1 {
			filteredQuads = append(filteredQuads, q)
		}
	}

	if len(filteredQuads) == 0 {
		return nil, fmt.Errorf("node is either not visible or not an HTMLElement: %w", err)
	}

	// Return the middle point of the first quad.
	content := filteredQuads[0]
	c := len(content)
	var x, y float64
	for i := 0; i < c; i += 2 {
		x += content[i]
		y += content[i+1]
	}
	x /= float64(c / 2)
	y /= float64(c / 2)
	p := Position{X: x, Y: y}

	// Firefox internally uses integer coordinates, so 8.5 is converted to 9 when clicking.
	//
	// This does not work nicely for small elements. For example, 1x1 square with corners
	// (8;8) and (9;9) is targeted when clicking at (8;8) but not when clicking at (9;9).
	// So, clicking at (8.5;8.5) will effectively click at (9;9) and miss the target.
	//
	// Therefore, we skew half-integer values from the interval (8.49, 8.51) towards
	// (8.47, 8.49) that is rounded towards 8. This means clicking at (8.5;8.5) will
	// be replaced with (8.48;8.48) and will effectively click at (8;8).
	//
	// Other browsers use float coordinates, so this change should not matter.
	remainderX := p.X - math.Floor(p.X)
	if remainderX > 0.49 && remainderX < 0.51 {
		p.X -= 0.02
	}
	remainderY := p.Y - math.Floor(p.Y)
	if remainderY > 0.49 && remainderY < 0.51 {
		p.Y -= 0.02
	}

	return &p, nil
}

func (h *ElementHandle) dblClick(p *Position, opts *MouseClickOptions) error {
	return h.frame.page.Mouse.click(p.X, p.Y, opts)
}

func (h *ElementHandle) defaultTimeout() time.Duration {
	return time.Duration(h.frame.manager.timeoutSettings.timeout()) * time.Second
}

func (h *ElementHandle) dispatchEvent(apiCtx context.Context, typ string, eventInit goja.Value) (interface{}, error) {
	rt := k6common.GetRuntime(apiCtx)
	injected, err := h.execCtx.getInjectedScript(apiCtx)
	if err != nil {
		return nil, err
	}
	pageFn := rt.ToValue(`
		(injected, node, type, eventInit) => {
			injected.dispatchEvent(node, type, eventInit);
		}
	`)
	_, err = h.execCtx.evaluate(
		apiCtx, true, true, pageFn, []goja.Value{
			rt.ToValue(injected),
			rt.ToValue(h),
			rt.ToValue(typ),
			eventInit,
		}...)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func (h *ElementHandle) fill(apiCtx context.Context, value string) (interface{}, error) {
	rt := k6common.GetRuntime(apiCtx)
	injected, err := h.execCtx.getInjectedScript(apiCtx)
	if err != nil {
		return nil, err
	}
	pageFn := rt.ToValue(`
			(injected, node, value) => {
				return injected.fill(node, value);
			}
		`)
	result, err := h.execCtx.evaluate(
		apiCtx, true, true, pageFn, []goja.Value{
			rt.ToValue(injected),
			rt.ToValue(h),
			rt.ToValue(value),
		}...)
	if err != nil {
		return nil, err
	}
	switch result := result.(type) {
	case string: // Either we're done or an error happened (returned as "error:..." from JS)
		if result != "done" {
			return nil, errorFromDOMError(result)
		}
	}
	return nil, nil
}

func (h *ElementHandle) focus(apiCtx context.Context, resetSelectionIfNotFocused bool) error {
	rt := k6common.GetRuntime(apiCtx)
	injected, err := h.execCtx.getInjectedScript(apiCtx)
	if err != nil {
		return err
	}
	pageFn := rt.ToValue(`
		(injected, node, resetSelectionIfNotFocused) => {
			return injected.focusNode(node, resetSelectionIfNotFocused);
		}
	`)
	result, err := h.execCtx.evaluate(
		apiCtx, true, true, pageFn, []goja.Value{
			rt.ToValue(injected),
			rt.ToValue(h),
			rt.ToValue(resetSelectionIfNotFocused),
		}...)
	if err != nil {
		return err
	}
	switch result := result.(type) {
	case string: // Either we're done or an error happened (returned as "error:..." from JS)
		if result != "done" {
			return errorFromDOMError(result)
		}
	}
	return nil
}

func (h *ElementHandle) getAttribute(apiCtx context.Context, name string) (interface{}, error) {
	js := `(element) => {
		return element.getAttribute('` + name + `');
	}`
	rt := k6common.GetRuntime(apiCtx)
	return h.execCtx.evaluate(apiCtx, true, true, rt.ToValue(js), rt.ToValue(h))
}

func (h *ElementHandle) hover(apiCtx context.Context, p *Position) error {
	return h.frame.page.Mouse.move(p.X, p.Y, NewMouseMoveOptions())
}

func (h *ElementHandle) innerHTML(apiCtx context.Context) (interface{}, error) {
	rt := k6common.GetRuntime(apiCtx)
	js := `(element) => {
		return element.innerHTML;
	}`
	return h.execCtx.evaluate(apiCtx, true, true, rt.ToValue(js), rt.ToValue(h))
}

func (h *ElementHandle) innerText(apiCtx context.Context) (interface{}, error) {
	rt := k6common.GetRuntime(apiCtx)
	js := `(element) => {
		return element.innerText;
	}`
	return h.execCtx.evaluate(apiCtx, true, true, rt.ToValue(js), rt.ToValue(h))
}

func (h *ElementHandle) inputValue(apiCtx context.Context) (interface{}, error) {
	rt := k6common.GetRuntime(apiCtx)
	js := `(element) => {
		if (element.nodeType !== Node.ELEMENT_NODE || (element.nodeName !== 'INPUT' && element.nodeName !== 'TEXTAREA' && element.nodeName !== 'SELECT')) {
        	throw Error('Node is not an <input>, <textarea> or <select> element');
		}
		return element.value;
	}`
	return h.execCtx.evaluate(apiCtx, true, true, rt.ToValue(js), rt.ToValue(h))
}

func (h *ElementHandle) isChecked(apiCtx context.Context, timeout time.Duration) (bool, error) {
	return h.waitForElementState(apiCtx, []string{"checked"}, timeout)
}

func (h *ElementHandle) isDisabled(apiCtx context.Context, timeout time.Duration) (bool, error) {
	return h.waitForElementState(apiCtx, []string{"disabled"}, timeout)
}

func (h *ElementHandle) isEditable(apiCtx context.Context, timeout time.Duration) (bool, error) {
	return h.waitForElementState(apiCtx, []string{"editable"}, timeout)
}

func (h *ElementHandle) isEnabled(apiCtx context.Context, timeout time.Duration) (bool, error) {
	return h.waitForElementState(apiCtx, []string{"enabled"}, timeout)
}

func (h *ElementHandle) isHidden(apiCtx context.Context, timeout time.Duration) (bool, error) {
	return h.waitForElementState(apiCtx, []string{"hidden"}, timeout)
}

func (h *ElementHandle) isVisible(apiCtx context.Context, timeout time.Duration) (bool, error) {
	return h.waitForElementState(apiCtx, []string{"visible"}, timeout)
}

func (h *ElementHandle) offsetPosition(apiCtx context.Context, offset *Position) (*Position, error) {
	rt := k6common.GetRuntime(apiCtx)
	box := h.BoundingBox()
	injected, err := h.execCtx.getInjectedScript(apiCtx)
	if err != nil {
		return nil, err
	}
	pageFn := rt.ToValue(`
		(injected, node) => {
			return injected.getElementBorderWidth(node);
		}
	`)
	result, err := h.execCtx.evaluate(
		apiCtx, true, true, pageFn, []goja.Value{
			rt.ToValue(injected),
			rt.ToValue(h),
		}...)
	if err != nil {
		return nil, err
	}

	var border struct{ Top, Left float64 }
	switch result := result.(type) {
	case goja.Value:
		if result != nil && !goja.IsUndefined(result) && !goja.IsNull(result) {
			obj := result.ToObject(rt)
			for _, k := range obj.Keys() {
				switch k {
				case "left":
					border.Left = obj.Get(k).ToFloat()
				case "top":
					border.Top = obj.Get(k).ToFloat()
				}
			}
		}
	}

	if box == nil || (border.Left == 0 && border.Top == 0) {
		return nil, errorFromDOMError("error:notvisible")
	}

	// Make point relative to the padding box to align with offsetX/offsetY.
	return &Position{
		X: box.X + border.Left + offset.X,
		Y: box.Y + border.Top + offset.Y,
	}, nil
}

func (h *ElementHandle) ownerFrame(apiCtx context.Context) *Frame {
	frameId := h.frame.page.getOwnerFrame(apiCtx, h)
	if frameId == "" {
		return nil
	}
	frame := h.frame.page.frameManager.getFrameByID(frameId)
	if frame != nil {
		return frame
	}
	for _, page := range h.frame.page.browserCtx.browser.pages {
		frame = page.frameManager.getFrameByID(frameId)
		if frame != nil {
			return frame
		}
	}
	return nil
}

func (h *ElementHandle) scrollRectIntoViewIfNeeded(apiCtx context.Context, rect *dom.Rect) error {
	action := dom.ScrollIntoViewIfNeeded().WithObjectID(h.remoteObject.ObjectID).WithRect(rect)
	err := action.Do(cdp.WithExecutor(apiCtx, h.session))
	if err != nil {
		if strings.Contains(err.Error(), "Node does not have a layout object") {
			return errorFromDOMError("error:notvisible")
		}
		if strings.Contains(err.Error(), "Node is detached from document") {
			return errorFromDOMError("error:notconnected")
		}
		return err
	}
	return nil
}

func (h *ElementHandle) press(apiCtx context.Context, key string, opts *KeyboardOptions) error {
	err := h.focus(apiCtx, true)
	if err != nil {
		return err
	}
	err = h.frame.page.Keyboard.press(key, opts)
	if err != nil {
		return err
	}
	return nil
}

func (h *ElementHandle) selectOption(apiCtx context.Context, values goja.Value) (interface{}, error) {
	rt := k6common.GetRuntime(apiCtx)
	injected, err := h.execCtx.getInjectedScript(apiCtx)
	if err != nil {
		return nil, err
	}

	convertSelectOptionValues := func(values goja.Value) ([]interface{}, error) {
		if goja.IsNull(values) || goja.IsUndefined(values) {
			return nil, nil
		}

		opts := make([]interface{}, 0)

		t := values.Export()
		switch values.ExportType().Kind() {
		case reflect.Map:
			s := reflect.ValueOf(t)
			for i := 0; i < s.Len(); i++ {
				item := s.Index(i)
				switch item.Kind() {
				case reflect.TypeOf(nil).Kind():
					return nil, fmt.Errorf("options[%d]: expected object, got null", i)
				case reflect.TypeOf(&ElementHandle{}).Kind():
					opts = append(opts, t.(*ElementHandle))
				case reflect.TypeOf(goja.Object{}).Kind():
					obj := values.ToObject(rt)
					opt := SelectOption{}
					for _, k := range obj.Keys() {
						switch k {
						case "value":
							opt.Value = new(string)
							*opt.Value = obj.Get(k).String()
						case "label":
							opt.Label = new(string)
							*opt.Label = obj.Get(k).String()
						case "index":
							opt.Index = new(int64)
							*opt.Index = obj.Get(k).ToInteger()
						}
					}
					opts = append(opts, &opt)
				case reflect.String:
					opt := SelectOption{Value: new(string)}
					*opt.Value = item.String()
					opts = append(opts, &opt)
				}
			}
		case reflect.TypeOf(&ElementHandle{}).Kind():
			opts = append(opts, t.(*ElementHandle))
		case reflect.TypeOf(goja.Object{}).Kind():
			obj := values.ToObject(rt)
			opt := SelectOption{}
			for _, k := range obj.Keys() {
				switch k {
				case "value":
					opt.Value = new(string)
					*opt.Value = obj.Get(k).String()
				case "label":
					opt.Label = new(string)
					*opt.Label = obj.Get(k).String()
				case "index":
					opt.Index = new(int64)
					*opt.Index = obj.Get(k).ToInteger()
				}
			}
			opts = append(opts, &opt)
		case reflect.String:
			opt := SelectOption{Value: new(string)}
			*opt.Value = t.(string)
			opts = append(opts, &opt)
		}

		return opts, nil
	}
	convValues, err := convertSelectOptionValues(values)
	if err != nil {
		return nil, err
	}

	pageFn := rt.ToValue(`
			(injected, node, values) => {
				return injected.selectOptions(node, values);
			}
		`)
	result, err := h.execCtx.evaluate(
		apiCtx, true, false, pageFn, []goja.Value{
			rt.ToValue(injected),
			rt.ToValue(h),
			rt.ToValue(convValues),
		}...)
	if err != nil {
		return nil, err
	}
	switch result := result.(type) {
	case string: // An error happened (returned as "error:..." from JS)
		if result != "done" {
			return nil, errorFromDOMError(result)
		}
	}
	return result, nil
}

func (h *ElementHandle) selectText(apiCtx context.Context) error {
	rt := k6common.GetRuntime(apiCtx)
	injected, err := h.execCtx.getInjectedScript(apiCtx)
	if err != nil {
		return err
	}
	pageFn := rt.ToValue(`
			(injected, node) => {
				return injected.selectText(node);
			}
		`)
	result, err := h.execCtx.evaluate(
		apiCtx, true, true, pageFn, []goja.Value{
			rt.ToValue(injected),
			rt.ToValue(h),
		}...)
	if err != nil {
		return err
	}
	switch result := result.(type) {
	case string: // Either we're done or an error happened (returned as "error:..." from JS)
		if result != "done" {
			return errorFromDOMError(result)
		}
	}
	return nil
}

func (h *ElementHandle) setChecked(apiCtx context.Context, checked bool, p *Position) error {
	state, err := h.checkElementState(apiCtx, "checked")
	if err != nil {
		return err
	}
	if checked == *state {
		return nil
	}

	err = h.click(p, NewMouseClickOptions())
	if err != nil {
		return err
	}

	state, err = h.checkElementState(apiCtx, "checked")
	if err != nil {
		return err
	}
	if checked != *state {
		return errors.New("clicking the checkbox did not change its state")
	}

	return nil
}

func (h *ElementHandle) tap(apiCtx context.Context, p *Position) error {
	return h.frame.page.Touchscreen.tap(p.X, p.X)
}

func (h *ElementHandle) textContent(apiCtx context.Context) (interface{}, error) {
	rt := k6common.GetRuntime(apiCtx)
	js := `(element) => {
		return element.textContent;
	}`
	return h.execCtx.evaluate(apiCtx, true, true, rt.ToValue(js), rt.ToValue(h))
}

func (h *ElementHandle) typ(apiCtx context.Context, text string, opts *KeyboardOptions) error {
	err := h.focus(apiCtx, true)
	if err != nil {
		return err
	}
	err = h.frame.page.Keyboard.typ(text, opts)
	if err != nil {
		return err
	}
	return nil
}

func (h *ElementHandle) waitForElementState(apiCtx context.Context, states []string, timeout time.Duration) (bool, error) {
	rt := k6common.GetRuntime(apiCtx)
	injected, err := h.execCtx.getInjectedScript(apiCtx)
	if err != nil {
		return false, err
	}
	pageFn := rt.ToValue(`
		(injected, node, states, timeout) => {
			return injected.waitForElementStates(node, states, timeout);
		}
	`)
	result, err := h.execCtx.evaluate(
		apiCtx, true, true, pageFn, []goja.Value{
			rt.ToValue(injected),
			rt.ToValue(h),
			rt.ToValue(states),
			rt.ToValue(timeout.Milliseconds()),
		}...)
	if err != nil {
		return false, err
	}

	value := result.(goja.Value)
	switch value.ExportType().Kind() {
	case reflect.String: // Either we're done or an error happened (returned as "error:..." from JS)
		if value.String() == "done" {
			return true, nil
		}
		return false, errorFromDOMError(value.String())
	case reflect.Bool:
		return value.ToBoolean(), nil
	}
	return false, fmt.Errorf("unable to check states %v of element: %q", states, reflect.TypeOf(result))
}

func (h *ElementHandle) waitForSelector(apiCtx context.Context, selector string, opts *FrameWaitForSelectorOptions) (*ElementHandle, error) {
	rt := k6common.GetRuntime(apiCtx)
	injected, err := h.execCtx.getInjectedScript(apiCtx)
	if err != nil {
		return nil, err
	}

	parsedSelector, err := NewSelector(selector)
	if err != nil {
		return nil, err
	}

	pageFn := rt.ToValue(`
		(injected, selector, scope, strict, state, timeout, ...args) => {
			return injected.waitForSelector(selector, scope, strict, state, 'raf', timeout, ...args);
		}
	`)
	result, err := h.execCtx.evaluate(
		apiCtx, true, false, pageFn, []goja.Value{
			rt.ToValue(injected),
			rt.ToValue(parsedSelector),
			rt.ToValue(h),
			rt.ToValue(opts.Strict),
			rt.ToValue(opts.State.String()),
			rt.ToValue(opts.Timeout.Milliseconds()),
		}...)
	if err != nil {
		return nil, err
	}
	switch r := result.(type) {
	case *ElementHandle:
		return r, nil
	default:
		return nil, nil
	}
}

// AsElement returns this element handle
func (h *ElementHandle) AsElement() api.ElementHandle {
	return h
}

// BoundingBox returns this element's bounding box
func (h *ElementHandle) BoundingBox() *api.Rect {
	bbox, err := h.boundingBox()
	if err != nil {
		return nil // Don't throw an exception here, just return nil
	}
	return bbox
}

// Check scrolls element into view and clicks in the center of the element
func (h *ElementHandle) Check(opts goja.Value) {
	h.SetChecked(true, opts)
}

// Click scrolls element into view and clicks in the center of the element
// TODO: look into making more robust using retries (see: https://github.com/microsoft/playwright/blob/master/src/server/dom.ts#L298)
func (h *ElementHandle) Click(opts goja.Value) {
	rt := k6common.GetRuntime(h.ctx)
	actionOpts := NewElementHandleClickOptions(h.defaultTimeout())
	if err := actionOpts.Parse(h.ctx, opts); err != nil {
		k6common.Throw(rt, fmt.Errorf("failed parsing options: %w", err))
	}
	fn := func(apiCtx context.Context, handle *ElementHandle, p *Position) (interface{}, error) {
		return nil, handle.click(p, actionOpts.ToMouseClickOptions())
	}
	pointerFn := elementHandlePointerActionFn(h, true, fn, &actionOpts.ElementHandleBasePointerOptions)
	_, err := callApiWithTimeout(h.ctx, pointerFn, actionOpts.Timeout)
	if err != nil {
		k6common.Throw(rt, err)
	}
	applySlowMo(h.ctx)
}

func (h *ElementHandle) ContentFrame() api.Frame {
	rt := k6common.GetRuntime(h.ctx)

	var node *cdp.Node
	var err error
	action := dom.DescribeNode().WithObjectID(h.remoteObject.ObjectID)
	if node, err = action.Do(cdp.WithExecutor(h.ctx, h.session)); err != nil {
		k6common.Throw(rt, fmt.Errorf("unable to describe DOM node: %w", err))
	}

	if node == nil || node.FrameID == "" {
		return nil
	}

	return h.frame.manager.getFrameByID(node.FrameID)
}

func (h *ElementHandle) Dblclick(opts goja.Value) {
	rt := k6common.GetRuntime(h.ctx)
	actionOpts := NewElementHandleDblclickOptions(h.defaultTimeout())
	if err := actionOpts.Parse(h.ctx, opts); err != nil {
		k6common.Throw(rt, fmt.Errorf("failed parsing options: %w", err))
	}
	fn := func(apiCtx context.Context, handle *ElementHandle, p *Position) (interface{}, error) {
		return nil, handle.dblClick(p, actionOpts.ToMouseClickOptions())
	}
	pointerFn := elementHandlePointerActionFn(h, true, fn, &actionOpts.ElementHandleBasePointerOptions)
	_, err := callApiWithTimeout(h.ctx, pointerFn, actionOpts.Timeout)
	if err != nil {
		k6common.Throw(rt, err)
	}
	applySlowMo(h.ctx)
}

func (h *ElementHandle) DispatchEvent(typ string, eventInit goja.Value) {
	rt := k6common.GetRuntime(h.ctx)
	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.dispatchEvent(apiCtx, typ, eventInit)
	}
	opts := NewElementHandleBaseOptions(h.defaultTimeout())
	actFn := elementHandleActionFn(h, []string{}, fn, opts.Force, opts.NoWaitAfter, opts.Timeout)
	_, err := callApiWithTimeout(h.ctx, actFn, opts.Timeout)
	if err != nil {
		k6common.Throw(rt, err)
	}
	applySlowMo(h.ctx)
}

func (h *ElementHandle) Fill(value string, opts goja.Value) {
	rt := k6common.GetRuntime(h.ctx)
	actionOpts := NewElementHandleBaseOptions(h.defaultTimeout())
	if err := actionOpts.Parse(h.ctx, opts); err != nil {
		k6common.Throw(rt, fmt.Errorf("failed parsing options: %w", err))
	}
	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.fill(apiCtx, value)
	}
	actFn := elementHandleActionFn(h, []string{"visible", "enabled", "editable"}, fn, actionOpts.Force, actionOpts.NoWaitAfter, actionOpts.Timeout)
	_, err := callApiWithTimeout(h.ctx, actFn, actionOpts.Timeout)
	if err != nil {
		k6common.Throw(rt, err)
	}
	applySlowMo(h.ctx)
}

// Focus scrolls element into view and focuses the element
func (h *ElementHandle) Focus() {
	rt := k6common.GetRuntime(h.ctx)
	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return nil, handle.focus(apiCtx, false)
	}
	opts := NewElementHandleBaseOptions(h.defaultTimeout())
	actFn := elementHandleActionFn(h, []string{}, fn, opts.Force, opts.NoWaitAfter, opts.Timeout)
	_, err := callApiWithTimeout(h.ctx, actFn, opts.Timeout)
	if err != nil {
		k6common.Throw(rt, err)
	}
	applySlowMo(h.ctx)
}

// GetAttribute retrieves the value of specified element attribute
func (h *ElementHandle) GetAttribute(name string) goja.Value {
	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.getAttribute(apiCtx, name)
	}
	opts := NewElementHandleBaseOptions(h.defaultTimeout())
	actFn := elementHandleActionFn(h, []string{}, fn, opts.Force, opts.NoWaitAfter, opts.Timeout)
	value, err := callApiWithTimeout(h.ctx, actFn, opts.Timeout)
	if err != nil {
		k6Throw(h.ctx, "GetAttribute(%q): %q", name, err)
	}
	applySlowMo(h.ctx)
	return value.(goja.Value)
}

// Hover scrolls element into view and hovers over its center point
func (h *ElementHandle) Hover(opts goja.Value) {
	rt := k6common.GetRuntime(h.ctx)
	actionOpts := NewElementHandleHoverOptions(h.defaultTimeout())
	if err := actionOpts.Parse(h.ctx, opts); err != nil {
		k6common.Throw(rt, fmt.Errorf("failed parsing options: %w", err))
	}
	fn := func(apiCtx context.Context, handle *ElementHandle, p *Position) (interface{}, error) {
		return nil, handle.hover(apiCtx, p)
	}
	pointerFn := elementHandlePointerActionFn(h, true, fn, &actionOpts.ElementHandleBasePointerOptions)
	_, err := callApiWithTimeout(h.ctx, pointerFn, actionOpts.Timeout)
	if err != nil {
		k6common.Throw(rt, err)
	}
	applySlowMo(h.ctx)
}

// InnerHTML returns the inner HTML of the element
func (h *ElementHandle) InnerHTML() string {
	rt := k6common.GetRuntime(h.ctx)
	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.innerHTML(apiCtx)
	}
	opts := NewElementHandleBaseOptions(h.defaultTimeout())
	actFn := elementHandleActionFn(h, []string{}, fn, opts.Force, opts.NoWaitAfter, opts.Timeout)
	value, err := callApiWithTimeout(h.ctx, actFn, opts.Timeout)
	if err != nil {
		k6common.Throw(rt, err)
	}
	applySlowMo(h.ctx)
	return value.(goja.Value).String()
}

// InnerText returns the inner text of the element
func (h *ElementHandle) InnerText() string {
	rt := k6common.GetRuntime(h.ctx)
	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.innerText(apiCtx)
	}
	opts := NewElementHandleBaseOptions(h.defaultTimeout())
	actFn := elementHandleActionFn(h, []string{}, fn, opts.Force, opts.NoWaitAfter, opts.Timeout)
	value, err := callApiWithTimeout(h.ctx, actFn, opts.Timeout)
	if err != nil {
		k6common.Throw(rt, err)
	}
	applySlowMo(h.ctx)
	return value.(goja.Value).String()
}

func (h *ElementHandle) InputValue(opts goja.Value) string {
	rt := k6common.GetRuntime(h.ctx)
	actionOpts := NewElementHandleBaseOptions(h.defaultTimeout())
	if err := actionOpts.Parse(h.ctx, opts); err != nil {
		k6common.Throw(rt, fmt.Errorf("failed parsing options: %w", err))
	}
	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.inputValue(apiCtx)
	}
	actFn := elementHandleActionFn(h, []string{}, fn, actionOpts.Force, actionOpts.NoWaitAfter, actionOpts.Timeout)
	value, err := callApiWithTimeout(h.ctx, actFn, actionOpts.Timeout)
	if err != nil {
		k6common.Throw(rt, err)
	}
	applySlowMo(h.ctx)
	return value.(goja.Value).String()
}

// IsChecked checks if a checkbox or radio is checked
func (h *ElementHandle) IsChecked() bool {
	rt := k6common.GetRuntime(h.ctx)
	result, err := h.isChecked(h.ctx, 0)
	if err != nil && err != ErrTimedOut { // We don't care anout timeout errors here!
		k6common.Throw(rt, err)
	}
	return result
}

// IsDisabled checks if the element is disabled
func (h *ElementHandle) IsDisabled() bool {
	rt := k6common.GetRuntime(h.ctx)
	result, err := h.isDisabled(h.ctx, 0)
	if err != nil && err != ErrTimedOut { // We don't care anout timeout errors here!
		k6common.Throw(rt, err)
	}
	return result
}

// IsEditable checks if the element is editable
func (h *ElementHandle) IsEditable() bool {
	rt := k6common.GetRuntime(h.ctx)
	result, err := h.isEditable(h.ctx, 0)
	if err != nil && err != ErrTimedOut { // We don't care anout timeout errors here!
		k6common.Throw(rt, err)
	}
	return result
}

// IsEnabled checks if the element is enabled
func (h *ElementHandle) IsEnabled() bool {
	rt := k6common.GetRuntime(h.ctx)
	result, err := h.isEnabled(h.ctx, 0)
	if err != nil && err != ErrTimedOut { // We don't care anout timeout errors here!
		k6common.Throw(rt, err)
	}
	return result
}

// IsHidden checks if the element is hidden
func (h *ElementHandle) IsHidden() bool {
	rt := k6common.GetRuntime(h.ctx)
	result, err := h.isHidden(h.ctx, 0)
	if err != nil && err != ErrTimedOut { // We don't care anout timeout errors here!
		k6common.Throw(rt, err)
	}
	return result
}

// IsVisible checks if the element is visible
func (h *ElementHandle) IsVisible() bool {
	rt := k6common.GetRuntime(h.ctx)
	result, err := h.isVisible(h.ctx, 0)
	if err != nil && err != ErrTimedOut { // We don't care anout timeout errors here!
		k6common.Throw(rt, err)
	}
	return result
}

// OwnerFrame returns the frame containing this element
func (h *ElementHandle) OwnerFrame() api.Frame {
	rt := k6common.GetRuntime(h.ctx)
	injected, err := h.execCtx.getInjectedScript(h.ctx)
	if err != nil {
		k6common.Throw(rt, fmt.Errorf("unable to run injection script: %w", err))
	}
	pageFn := rt.ToValue(`
		(injected, node) => {
			return injected.getDocumentElement(node);
		}
	`)
	res, err := h.execCtx.evaluate(h.ctx, true, false, pageFn, []goja.Value{rt.ToValue(injected), rt.ToValue(h)}...)
	if err != nil {
		k6common.Throw(rt, fmt.Errorf("failed getting document element: %w", err))
	}
	if res == nil {
		return nil
	}

	documentHandle := res.(*ElementHandle)
	defer documentHandle.Dispose()
	if documentHandle.remoteObject.ObjectID == "" {
		return nil
	}

	var node *cdp.Node
	action := dom.DescribeNode().WithObjectID(documentHandle.remoteObject.ObjectID)
	if node, err = action.Do(cdp.WithExecutor(h.ctx, h.session)); err != nil {
		k6common.Throw(rt, fmt.Errorf("unable to describe DOM node: %w", err))
	}

	if node == nil || node.FrameID == "" {
		return nil
	}

	return h.frame.manager.getFrameByID(node.FrameID)
}

func (h *ElementHandle) Press(key string, opts goja.Value) {
	rt := k6common.GetRuntime(h.ctx)
	parsedOpts := NewElementHandlePressOptions(h.defaultTimeout())
	parsedOpts.Parse(h.ctx, opts)
	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return nil, handle.press(apiCtx, key, NewKeyboardOptions())
	}
	actFn := elementHandleActionFn(h, []string{}, fn, false, parsedOpts.NoWaitAfter, parsedOpts.Timeout)
	_, err := callApiWithTimeout(h.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		k6common.Throw(rt, err)
	}
	applySlowMo(h.ctx)
}

// Query runs "element.querySelector" within the page. If no element matches the selector,
// the return value resolves to "null"
func (h *ElementHandle) Query(selector string) api.ElementHandle {
	rt := k6common.GetRuntime(h.ctx)
	parsedSelector, err := NewSelector(selector)
	if err != nil {
		k6common.Throw(rt, err)
	}

	injected, err := h.execCtx.getInjectedScript(h.ctx)
	if err != nil {
		k6common.Throw(rt, err)
	}
	pageFn := rt.ToValue(`
		(injected, selector, scope) => {
			return injected.querySelector(selector, scope || document, false);
		}
	`)
	result, err := h.execCtx.evaluate(
		h.ctx, true, false, pageFn, []goja.Value{
			rt.ToValue(injected),
			rt.ToValue(parsedSelector),
			rt.ToValue(h),
		}...)
	if err != nil {
		k6common.Throw(rt, err)
	}
	if result == nil {
		return nil
	}

	handle := result.(api.JSHandle)
	element := handle.AsElement()
	applySlowMo(h.ctx)
	if element != nil {
		return element
	}
	handle.Dispose()
	return nil
}

// QueryAll queries element subtree for matching elements. If no element matches the selector,
// the return value resolves to "null"
func (h *ElementHandle) QueryAll(selector string) []api.ElementHandle {
	rt := k6common.GetRuntime(h.ctx)
	parsedSelector, err := NewSelector(selector)
	if err != nil {
		k6common.Throw(rt, err)
	}

	injected, err := h.execCtx.getInjectedScript(h.ctx)
	if err != nil {
		k6common.Throw(rt, err)
	}
	pageFn := rt.ToValue(`
		(injected, selector, scope) => {
			return injected.querySelectorAll(selector, scope || document, false);
		}
	`)
	result, err := h.execCtx.evaluate(
		h.ctx, true, false, pageFn, []goja.Value{
			rt.ToValue(injected),
			rt.ToValue(parsedSelector),
			rt.ToValue(h),
		}...)
	if err != nil {
		k6common.Throw(rt, err)
	}
	if result == nil {
		return nil
	}

	arrayHandle := result.(api.JSHandle)
	defer arrayHandle.Dispose()
	properties := arrayHandle.GetProperties()
	elements := make([]api.ElementHandle, len(properties))
	for _, property := range properties {
		elementHandle := property.AsElement()
		if elementHandle != nil {
			result = append(elements, elementHandle)
		} else {
			property.Dispose()
		}
	}
	applySlowMo(h.ctx)
	return elements
}

func (h *ElementHandle) Screenshot(opts goja.Value) goja.ArrayBuffer {
	// TODO: https://github.com/microsoft/playwright/blob/master/src/server/screenshotter.ts#L92
	rt := k6common.GetRuntime(h.ctx)
	parsedOpts := NewElementHandleScreenshotOptions(h.defaultTimeout())
	if err := parsedOpts.Parse(h.ctx, opts); err != nil {
		k6common.Throw(rt, fmt.Errorf("failed parsing options: %w", err))
	}

	var buf []byte
	var clip cdppage.Viewport
	format := parsedOpts.Format

	bbox, err := h.boundingBox()
	if err != nil {
		k6common.Throw(rt, fmt.Errorf("node is either not visible or not an HTMLElement: %w", err))
	}
	if bbox.Width <= 0 {
		k6common.Throw(rt, errors.New("node has 0 width"))
	}
	if bbox.Height <= 0 {
		k6common.Throw(rt, errors.New("node has 0 height"))
	}

	// Infer file format by path
	if parsedOpts.Path != "" && parsedOpts.Format != "png" && parsedOpts.Format != "jpeg" {
		if strings.HasSuffix(parsedOpts.Path, ".jpg") || strings.HasSuffix(parsedOpts.Path, ".jpeg") {
			format = "jpeg"
		}
	}

	var capture *cdppage.CaptureScreenshotParams

	// Setup viewport or full page screenshot capture based on options
	_, _, contentSize, _, _, _, err := cdppage.GetLayoutMetrics().Do(cdp.WithExecutor(h.ctx, h.session))
	if err != nil {
		k6common.Throw(rt, fmt.Errorf("unable to get layout metrics: %w", err))
	}
	width, height := int64(math.Ceil(contentSize.Width)), int64(math.Ceil(contentSize.Height))
	action := emulation.SetDeviceMetricsOverride(width, height, 1, false).
		WithScreenOrientation(&emulation.ScreenOrientation{
			Type:  emulation.OrientationTypePortraitPrimary,
			Angle: 0,
		})
	if err = action.Do(cdp.WithExecutor(h.ctx, h.session)); err != nil {
		k6common.Throw(rt, fmt.Errorf("unable to set screen width and height: %w", err))
	}
	clip = cdppage.Viewport{
		X:      contentSize.X,
		Y:      contentSize.Y,
		Width:  contentSize.Width,
		Height: contentSize.Height,
		Scale:  1,
	}

	if clip.Width > 0 && clip.Height > 0 {
		capture = capture.WithClip(&clip)
	}

	// Add common options
	capture.WithQuality(parsedOpts.Quality)
	switch format {
	case "jpeg":
		capture.WithFormat(cdppage.CaptureScreenshotFormatJpeg)
	default:
		capture.WithFormat(cdppage.CaptureScreenshotFormatPng)
	}

	// Make background transparent for PNG captures if requested
	if parsedOpts.OmitBackground && format == "png" {
		action := emulation.SetDefaultBackgroundColorOverride().
			WithColor(&cdp.RGBA{R: 0, G: 0, B: 0, A: 0})
		if err := action.Do(cdp.WithExecutor(h.ctx, h.session)); err != nil {
			k6common.Throw(rt, fmt.Errorf("unable to set screenshot background transparency: %w", err))
		}
	}

	// Capture screenshot
	buf, err = capture.Do(cdp.WithExecutor(h.ctx, h.session))
	if err != nil {
		k6common.Throw(rt, fmt.Errorf("unable to capture screenshot of page '%s': %w", h.frame.manager.MainFrame().URL(), err))
	}

	// Reset background
	if parsedOpts.OmitBackground && format == "png" {
		action := emulation.SetDefaultBackgroundColorOverride()
		if err := action.Do(cdp.WithExecutor(h.ctx, h.session)); err != nil {
			k6common.Throw(rt, fmt.Errorf("unable to reset screenshot background color: %w", err))
		}
	}

	// TODO: Reset viewport

	// Save screenshot capture to file
	// TODO: we should not write to disk here but put it on some queue for async disk writes
	if parsedOpts.Path != "" {
		dir := filepath.Dir(parsedOpts.Path)
		if err := os.MkdirAll(dir, 0775); err != nil {
			k6common.Throw(rt, fmt.Errorf("unable to create directory for screenshot of page '%s': %w", h.frame.manager.MainFrame().URL(), err))
		}
		if err := ioutil.WriteFile(parsedOpts.Path, buf, 0664); err != nil {
			k6common.Throw(rt, fmt.Errorf("unable to save screenshot of page '%s' to file: %w", h.frame.manager.MainFrame().URL(), err))
		}
	}

	return rt.NewArrayBuffer(buf)
}

func (h *ElementHandle) ScrollIntoViewIfNeeded(opts goja.Value) {
	rt := k6common.GetRuntime(h.ctx)
	actionOpts := NewElementHandleBaseOptions(h.defaultTimeout())
	if err := actionOpts.Parse(h.ctx, opts); err != nil {
		k6common.Throw(rt, fmt.Errorf("failed parsing options: %w", err))
	}
	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		pageFn := rt.ToValue(`(element) => {
			element.scrollIntoViewIfNeeded(true);
			return [window.scrollX, window.scrollY];
		}`)
		return h.execCtx.evaluate(apiCtx, true, true, pageFn, rt.ToValue(h))
	}
	actFn := elementHandleActionFn(h, []string{"visible", "stable"}, fn, actionOpts.Force, actionOpts.NoWaitAfter, actionOpts.Timeout)
	_, err := callApiWithTimeout(h.ctx, actFn, actionOpts.Timeout)
	if err != nil {
		k6common.Throw(rt, err)
	}
	applySlowMo(h.ctx)
}

func (h *ElementHandle) SelectOption(values goja.Value, opts goja.Value) []string {
	rt := k6common.GetRuntime(h.ctx)
	actionOpts := NewElementHandleBaseOptions(h.defaultTimeout())
	if err := actionOpts.Parse(h.ctx, opts); err != nil {
		k6common.Throw(rt, fmt.Errorf("failed parsing options: %w", err))
	}
	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.selectOption(apiCtx, values)
	}
	actFn := elementHandleActionFn(h, []string{}, fn, actionOpts.Force, actionOpts.NoWaitAfter, actionOpts.Timeout)
	selectedOptions, err := callApiWithTimeout(h.ctx, actFn, actionOpts.Timeout)
	if err != nil {
		k6common.Throw(rt, err)
	}
	var returnVal []string
	if err := rt.ExportTo(selectedOptions.(goja.Value), &returnVal); err != nil {
		k6common.Throw(rt, fmt.Errorf("unable to unpack selected options: %w", err))
	}
	applySlowMo(h.ctx)
	return returnVal
}

func (h *ElementHandle) SelectText(opts goja.Value) {
	rt := k6common.GetRuntime(h.ctx)
	actionOpts := NewElementHandleBaseOptions(h.defaultTimeout())
	if err := actionOpts.Parse(h.ctx, opts); err != nil {
		k6common.Throw(rt, err)
	}
	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return nil, handle.selectText(apiCtx)
	}
	actFn := elementHandleActionFn(h, []string{}, fn, actionOpts.Force, actionOpts.NoWaitAfter, actionOpts.Timeout)
	_, err := callApiWithTimeout(h.ctx, actFn, actionOpts.Timeout)
	if err != nil {
		k6common.Throw(rt, err)
	}
	applySlowMo(h.ctx)
}

// SetChecked checks or unchecks an element.
func (h *ElementHandle) SetChecked(checked bool, opts goja.Value) {
	rt := k6common.GetRuntime(h.ctx)
	parsedOpts := NewElementHandleSetCheckedOptions(h.defaultTimeout())
	err := parsedOpts.Parse(h.ctx, opts)
	if err != nil {
		k6common.Throw(rt, fmt.Errorf("failed parsing options: %w", err))
	}

	fn := func(apiCtx context.Context, handle *ElementHandle, p *Position) (interface{}, error) {
		return nil, handle.setChecked(apiCtx, checked, p)
	}
	pointerFn := elementHandlePointerActionFn(h, true, fn, &parsedOpts.ElementHandleBasePointerOptions)
	_, err = callApiWithTimeout(h.ctx, pointerFn, parsedOpts.Timeout)
	if err != nil {
		k6common.Throw(rt, err)
	}
	applySlowMo(h.ctx)
}

func (h *ElementHandle) SetInputFiles(files goja.Value, opts goja.Value) {
	// TODO: implement
	rt := k6common.GetRuntime(h.ctx)
	k6common.Throw(rt, errors.New("ElementHandle.setInputFiles() has not been implemented yet!"))
}

func (h *ElementHandle) Tap(opts goja.Value) {
	rt := k6common.GetRuntime(h.ctx)
	parsedOpts := NewElementHandleTapOptions(h.defaultTimeout())
	err := parsedOpts.Parse(h.ctx, opts)
	if err != nil {
		k6common.Throw(rt, fmt.Errorf("failed parsing options: %w", err))
	}

	fn := func(apiCtx context.Context, handle *ElementHandle, p *Position) (interface{}, error) {
		return nil, handle.tap(apiCtx, p)
	}
	pointerFn := elementHandlePointerActionFn(h, true, fn, &parsedOpts.ElementHandleBasePointerOptions)
	_, err = callApiWithTimeout(h.ctx, pointerFn, parsedOpts.Timeout)
	if err != nil {
		k6common.Throw(rt, err)
	}
	applySlowMo(h.ctx)
}

func (h *ElementHandle) TextContent() string {
	rt := k6common.GetRuntime(h.ctx)
	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.textContent(apiCtx)
	}
	opts := NewElementHandleBaseOptions(h.defaultTimeout())
	actFn := elementHandleActionFn(h, []string{}, fn, opts.Force, opts.NoWaitAfter, opts.Timeout)
	value, err := callApiWithTimeout(h.ctx, actFn, opts.Timeout)
	if err != nil {
		k6common.Throw(rt, err)
	}
	applySlowMo(h.ctx)
	return value.(goja.Value).String()
}

// Type scrolls element into view, focuses element and types text
func (h *ElementHandle) Type(text string, opts goja.Value) {
	rt := k6common.GetRuntime(h.ctx)
	parsedOpts := NewElementHandleTypeOptions(h.defaultTimeout())
	parsedOpts.Parse(h.ctx, opts)
	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return nil, handle.typ(apiCtx, text, NewKeyboardOptions())
	}
	actFn := elementHandleActionFn(h, []string{}, fn, false, parsedOpts.NoWaitAfter, parsedOpts.Timeout)
	_, err := callApiWithTimeout(h.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		k6common.Throw(rt, err)
	}
	applySlowMo(h.ctx)
}

// Uncheck scrolls element into view and clicks in the center of the element
func (h *ElementHandle) Uncheck(opts goja.Value) {
	h.SetChecked(false, opts)
}

func (h *ElementHandle) WaitForElementState(state string, opts goja.Value) {
	rt := k6common.GetRuntime(h.ctx)
	parsedOpts := NewElementHandleWaitForElementStateOptions(time.Duration(h.frame.manager.timeoutSettings.timeout()) * time.Second)
	err := parsedOpts.Parse(h.ctx, opts)
	if err != nil {
		k6common.Throw(rt, fmt.Errorf("failed parsing options: %w", err))
	}
	_, err = h.waitForElementState(h.ctx, []string{state}, parsedOpts.Timeout)
	if err != nil {
		k6common.Throw(rt, fmt.Errorf("error while waiting for state: %w", err))
	}
}

func (h *ElementHandle) WaitForSelector(selector string, opts goja.Value) api.ElementHandle {
	rt := k6common.GetRuntime(h.ctx)
	parsedOpts := NewFrameWaitForSelectorOptions(h.defaultTimeout())
	if err := parsedOpts.Parse(h.ctx, opts); err != nil {
		k6common.Throw(rt, fmt.Errorf("failed parsing options: %w", err))
	}

	handle, err := h.waitForSelector(h.ctx, selector, parsedOpts)
	if err != nil {
		k6common.Throw(rt, err)
	}

	return handle
}
