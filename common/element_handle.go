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
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	cdppage "github.com/chromedp/cdproto/page"
	"github.com/dop251/goja"
	k6common "go.k6.io/k6/js/common"

	"github.com/grafana/xk6-browser/api"
)

// Ensure ElementHandle implements the api.ElementHandle and api.JSHandle interfaces.
var _ api.ElementHandle = &ElementHandle{}
var _ api.JSHandle = &ElementHandle{}

type (
	ElementHandleActionFn        func(context.Context, *ElementHandle) (interface{}, error)
	ElementHandlePointerActionFn func(context.Context, *ElementHandle, *Position) (interface{}, error)
)

func elementHandleActionFn(
	h *ElementHandle, states []string, fn ElementHandleActionFn, force, noWaitAfter bool, timeout time.Duration,
) func(apiCtx context.Context, resultCh chan interface{}, errCh chan error) {
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

func elementHandlePointerActionFn(
	h *ElementHandle, checkEnabled bool, fn ElementHandlePointerActionFn, opts *ElementHandleBasePointerOptions,
) func(apiCtx context.Context, resultCh chan interface{}, errCh chan error) {
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

// ElementHandle represents a HTML element JS object inside an execution context.
type ElementHandle struct {
	BaseJSHandle

	frame *Frame
}

func (h *ElementHandle) boundingBox() (*Rect, error) {
	var box *dom.BoxModel
	var err error
	action := dom.GetBoxModel().WithObjectID(h.remoteObject.ObjectID)
	if box, err = action.Do(cdp.WithExecutor(h.ctx, h.session)); err != nil {
		return nil, fmt.Errorf("cannot get bounding box model of DOM node: %w", err)
	}

	quad := box.Border
	x := math.Min(quad[0], math.Min(quad[2], math.Min(quad[4], quad[6])))
	y := math.Min(quad[1], math.Min(quad[3], math.Min(quad[5], quad[7])))
	width := math.Max(quad[0], math.Max(quad[2], math.Max(quad[4], quad[6]))) - x
	height := math.Max(quad[1], math.Max(quad[3], math.Max(quad[5], quad[7]))) - y
	position := h.frame.position()

	return &Rect{X: x + position.X, Y: y + position.Y, Width: width, Height: height}, nil
}

func (h *ElementHandle) checkHitTargetAt(apiCtx context.Context, point Position) (bool, error) {
	frame := h.ownerFrame(apiCtx)
	if frame != nil && frame.parentFrame != nil {
		element := h.frame.FrameElement().(*ElementHandle)
		box, err := element.boundingBox()
		if err != nil {
			return false, err
		}
		if box == nil {
			return false, errors.New("cannot get bounding box of element")
		}
		// Translate from viewport coordinates to frame coordinates.
		point.X = point.X - box.X
		point.Y = point.Y - box.Y
	}
	fn := `
		(node, injected, point) => {
			return injected.checkHitTargetAt(node, point);
		}
	`
	opts := evalOptions{
		forceCallable: true,
		returnByValue: true,
	}
	result, err := h.evalWithScript(h.ctx, opts, fn, point)
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
	fn := `
		(node, injected, state) => {
			return injected.checkElementState(node, state);
		}
	`
	opts := evalOptions{
		forceCallable: true,
		returnByValue: true,
	}
	result, err := h.evalWithScript(h.ctx, opts, fn, state)
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
	return nil, fmt.Errorf("cannot check state %q of element: %q", state, reflect.TypeOf(result))
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
		return nil, fmt.Errorf("cannot request node content quads %T: %w", action, err)
	}
	if len(quads) == 0 {
		return nil, fmt.Errorf("node is either not visible or not an HTMLElement: %w", err)
	}

	action2 := cdppage.GetLayoutMetrics()
	if layoutViewport, _, _, _, _, _, err = action2.Do(cdp.WithExecutor(h.ctx, h.session)); err != nil {
		return nil, fmt.Errorf("cannot get page layout metrics %T: %w", action, err)
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
	fn := `
		(node, injected, type, eventInit) => {
			injected.dispatchEvent(node, type, eventInit);
		}
	`
	opts := evalOptions{
		forceCallable: true,
		returnByValue: true,
	}
	_, err := h.evalWithScript(h.ctx, opts, fn, typ, eventInit)
	return nil, err
}

func (h *ElementHandle) fill(apiCtx context.Context, value string) (interface{}, error) {
	fn := `
		(node, injected, value) => {
			return injected.fill(node, value);
		}
	`
	opts := evalOptions{
		forceCallable: true,
		returnByValue: true,
	}
	result, err := h.evalWithScript(h.ctx, opts, fn, value)
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
	fn := `
		(node, injected, resetSelectionIfNotFocused) => {
			return injected.focusNode(node, resetSelectionIfNotFocused);
		}
	`
	opts := evalOptions{
		forceCallable: true,
		returnByValue: true,
	}
	result, err := h.evalWithScript(apiCtx, opts, fn, resetSelectionIfNotFocused)
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
	js := `
		(element) => {
			return element.getAttribute('` + name + `');
		}
	`
	opts := evalOptions{
		forceCallable: true,
		returnByValue: true,
	}
	return h.eval(apiCtx, opts, js)
}

func (h *ElementHandle) hover(apiCtx context.Context, p *Position) error {
	return h.frame.page.Mouse.move(p.X, p.Y, NewMouseMoveOptions())
}

func (h *ElementHandle) innerHTML(apiCtx context.Context) (interface{}, error) {
	js := `
		(element) => {
			return element.innerHTML;
		}
	`
	opts := evalOptions{
		forceCallable: true,
		returnByValue: true,
	}
	return h.eval(apiCtx, opts, js)
}

func (h *ElementHandle) innerText(apiCtx context.Context) (interface{}, error) {
	js := `
		(element) => {
			return element.innerText;
		}
	`
	opts := evalOptions{
		forceCallable: true,
		returnByValue: true,
	}
	return h.eval(apiCtx, opts, js)
}

func (h *ElementHandle) inputValue(apiCtx context.Context) (interface{}, error) {
	js := `
		(element) => {
			if (element.nodeType !== Node.ELEMENT_NODE || (element.nodeName !== 'INPUT' && element.nodeName !== 'TEXTAREA' && element.nodeName !== 'SELECT')) {
        			throw Error('Node is not an <input>, <textarea> or <select> element');
			}
			return element.value;
		}
	`
	opts := evalOptions{
		forceCallable: true,
		returnByValue: true,
	}
	return h.eval(apiCtx, opts, js)
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
	fn := `
		(node, injected) => {
			return injected.getElementBorderWidth(node);
		}
	`
	opts := evalOptions{
		forceCallable: true,
		returnByValue: true,
	}
	result, err := h.evalWithScript(apiCtx, opts, fn)
	if err != nil {
		return nil, err
	}

	rt := k6common.GetRuntime(apiCtx)
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

	box := h.BoundingBox()
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
	convertSelectOptionValues := func(values goja.Value) ([]interface{}, error) {
		if goja.IsNull(values) || goja.IsUndefined(values) {
			return nil, nil
		}

		var (
			opts []interface{}
			t    = values.Export()
			rt   = k6common.GetRuntime(h.ctx)
		)
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

	fn := `
		(node, injected, values) => {
			return injected.selectOptions(node, values);
		}
	`
	opts := evalOptions{
		forceCallable: true,
		returnByValue: false,
	}
	result, err := h.evalWithScript(apiCtx, opts, fn, convValues)
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
	fn := `
		(node, injected) => {
			return injected.selectText(node);
		}
	`
	opts := evalOptions{
		forceCallable: true,
		returnByValue: true,
	}
	result, err := h.evalWithScript(apiCtx, opts, fn)
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
	js := `
		(element) => {
			return element.textContent;
		}
	`
	opts := evalOptions{
		forceCallable: true,
		returnByValue: true,
	}
	return h.eval(apiCtx, opts, js)
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

func (h *ElementHandle) waitAndScrollIntoViewIfNeeded(apiCtx context.Context, force, noWaitAfter bool, timeout time.Duration) error {
	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		fn := `
			(element) => {
				element.scrollIntoViewIfNeeded(true);
				return [window.scrollX, window.scrollY];
			}
		`
		opts := evalOptions{
			forceCallable: true,
			returnByValue: true,
		}
		return h.eval(apiCtx, opts, fn)
	}
	actFn := elementHandleActionFn(h, []string{"visible", "stable"}, fn, force, noWaitAfter, timeout)
	_, err := callApiWithTimeout(h.ctx, actFn, timeout)
	if err != nil {
		return err
	}
	return nil
}

func (h *ElementHandle) waitForElementState(apiCtx context.Context, states []string, timeout time.Duration) (bool, error) {
	fn := `
		(node, injected, states, timeout) => {
			return injected.waitForElementStates(node, states, timeout);
		}
	`
	opts := evalOptions{
		forceCallable: true,
		returnByValue: true,
	}
	result, err := h.evalWithScript(apiCtx, opts, fn, states, timeout.Milliseconds())
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
	return false, fmt.Errorf("cannot check states %v of element: %q", states, reflect.TypeOf(result))
}

func (h *ElementHandle) waitForSelector(apiCtx context.Context, selector string, opts *FrameWaitForSelectorOptions) (*ElementHandle, error) {
	parsedSelector, err := NewSelector(selector)
	if err != nil {
		return nil, err
	}
	fn := `
		(node, injected, selector, strict, state, timeout, ...args) => {
			return injected.waitForSelector(selector, node, strict, state, 'raf', timeout, ...args);
		}
	`
	eopts := evalOptions{
		forceCallable: true,
		returnByValue: false,
	}
	result, err := h.evalWithScript(
		apiCtx,
		eopts, fn, parsedSelector,
		opts.Strict, opts.State.String(), opts.Timeout.Milliseconds(),
	)
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

// AsElement returns this element handle.
func (h *ElementHandle) AsElement() api.ElementHandle {
	return h
}

// BoundingBox returns this element's bounding box.
func (h *ElementHandle) BoundingBox() *api.Rect {
	bbox, err := h.boundingBox()
	if err != nil {
		return nil // Don't throw an exception here, just return nil
	}
	return bbox.toApiRect()
}

// Check scrolls element into view and clicks in the center of the element.
func (h *ElementHandle) Check(opts goja.Value) {
	h.SetChecked(true, opts)
}

// Click scrolls element into view and clicks in the center of the element
// TODO: look into making more robust using retries (see: https://github.com/microsoft/playwright/blob/master/src/server/dom.ts#L298)
func (h *ElementHandle) Click(opts goja.Value) {
	actionOpts := NewElementHandleClickOptions(h.defaultTimeout())
	if err := actionOpts.Parse(h.ctx, opts); err != nil {
		k6Throw(h.ctx, "cannot parse element click options: %w", err)
	}
	fn := func(apiCtx context.Context, handle *ElementHandle, p *Position) (interface{}, error) {
		return nil, handle.click(p, actionOpts.ToMouseClickOptions())
	}
	pointerFn := elementHandlePointerActionFn(h, true, fn, &actionOpts.ElementHandleBasePointerOptions)
	_, err := callApiWithTimeout(h.ctx, pointerFn, actionOpts.Timeout)
	if err != nil {
		k6Throw(h.ctx, "cannot click on element: %w", err)
	}
	applySlowMo(h.ctx)
}

func (h *ElementHandle) ContentFrame() api.Frame {
	var (
		node *cdp.Node
		err  error
	)
	action := dom.DescribeNode().WithObjectID(h.remoteObject.ObjectID)
	if node, err = action.Do(cdp.WithExecutor(h.ctx, h.session)); err != nil {
		k6Throw(h.ctx, "cannot get remote node %q: %w", h.remoteObject.ObjectID, err)
	}
	if node == nil || node.FrameID == "" {
		return nil
	}

	return h.frame.manager.getFrameByID(node.FrameID)
}

func (h *ElementHandle) Dblclick(opts goja.Value) {
	actionOpts := NewElementHandleDblclickOptions(h.defaultTimeout())
	if err := actionOpts.Parse(h.ctx, opts); err != nil {
		k6Throw(h.ctx, "cannot parse element double click options: %w", err)
	}
	fn := func(apiCtx context.Context, handle *ElementHandle, p *Position) (interface{}, error) {
		return nil, handle.dblClick(p, actionOpts.ToMouseClickOptions())
	}
	pointerFn := elementHandlePointerActionFn(h, true, fn, &actionOpts.ElementHandleBasePointerOptions)
	_, err := callApiWithTimeout(h.ctx, pointerFn, actionOpts.Timeout)
	if err != nil {
		k6Throw(h.ctx, "cannot double click on element: %w", err)
	}
	applySlowMo(h.ctx)
}

func (h *ElementHandle) DispatchEvent(typ string, eventInit goja.Value) {
	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.dispatchEvent(apiCtx, typ, eventInit)
	}
	opts := NewElementHandleBaseOptions(h.defaultTimeout())
	actFn := elementHandleActionFn(h, []string{}, fn, opts.Force, opts.NoWaitAfter, opts.Timeout)
	_, err := callApiWithTimeout(h.ctx, actFn, opts.Timeout)
	if err != nil {
		k6Throw(h.ctx, "cannot dispatch element event: %w", err)
	}
	applySlowMo(h.ctx)
}

func (h *ElementHandle) Fill(value string, opts goja.Value) {
	actionOpts := NewElementHandleBaseOptions(h.defaultTimeout())
	if err := actionOpts.Parse(h.ctx, opts); err != nil {
		k6Throw(h.ctx, "cannot parse element fill options: %w", err)
	}
	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.fill(apiCtx, value)
	}
	actFn := elementHandleActionFn(h, []string{"visible", "enabled", "editable"},
		fn, actionOpts.Force, actionOpts.NoWaitAfter, actionOpts.Timeout)
	_, err := callApiWithTimeout(h.ctx, actFn, actionOpts.Timeout)
	if err != nil {
		k6Throw(h.ctx, "cannot handle element fill action: %w", err)
	}
	applySlowMo(h.ctx)
}

// Focus scrolls element into view and focuses the element.
func (h *ElementHandle) Focus() {
	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return nil, handle.focus(apiCtx, false)
	}
	opts := NewElementHandleBaseOptions(h.defaultTimeout())
	actFn := elementHandleActionFn(h, []string{}, fn, opts.Force, opts.NoWaitAfter, opts.Timeout)
	_, err := callApiWithTimeout(h.ctx, actFn, opts.Timeout)
	if err != nil {
		k6Throw(h.ctx, "cannot focus on element: %w", err)
	}
	applySlowMo(h.ctx)
}

// GetAttribute retrieves the value of specified element attribute.
func (h *ElementHandle) GetAttribute(name string) goja.Value {
	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.getAttribute(apiCtx, name)
	}
	opts := NewElementHandleBaseOptions(h.defaultTimeout())
	actFn := elementHandleActionFn(h, []string{}, fn, opts.Force, opts.NoWaitAfter, opts.Timeout)
	value, err := callApiWithTimeout(h.ctx, actFn, opts.Timeout)
	if err != nil {
		k6Throw(h.ctx, "cannot get attribute of %q: %q", name, err)
	}
	applySlowMo(h.ctx)
	return value.(goja.Value)
}

// Hover scrolls element into view and hovers over its center point.
func (h *ElementHandle) Hover(opts goja.Value) {
	actionOpts := NewElementHandleHoverOptions(h.defaultTimeout())
	if err := actionOpts.Parse(h.ctx, opts); err != nil {
		k6Throw(h.ctx, "cannot parse element hover options: %w", err)
	}
	fn := func(apiCtx context.Context, handle *ElementHandle, p *Position) (interface{}, error) {
		return nil, handle.hover(apiCtx, p)
	}
	pointerFn := elementHandlePointerActionFn(h, true, fn, &actionOpts.ElementHandleBasePointerOptions)
	_, err := callApiWithTimeout(h.ctx, pointerFn, actionOpts.Timeout)
	if err != nil {
		k6Throw(h.ctx, "cannot hover on element: %w", err)
	}
	applySlowMo(h.ctx)
}

// InnerHTML returns the inner HTML of the element.
func (h *ElementHandle) InnerHTML() string {
	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.innerHTML(apiCtx)
	}
	opts := NewElementHandleBaseOptions(h.defaultTimeout())
	actFn := elementHandleActionFn(h, []string{}, fn, opts.Force, opts.NoWaitAfter, opts.Timeout)
	value, err := callApiWithTimeout(h.ctx, actFn, opts.Timeout)
	if err != nil {
		k6Throw(h.ctx, "cannot get element's inner HTML: %w", err)
	}
	applySlowMo(h.ctx)
	return value.(goja.Value).String()
}

// InnerText returns the inner text of the element.
func (h *ElementHandle) InnerText() string {
	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.innerText(apiCtx)
	}
	opts := NewElementHandleBaseOptions(h.defaultTimeout())
	actFn := elementHandleActionFn(h, []string{}, fn, opts.Force, opts.NoWaitAfter, opts.Timeout)
	value, err := callApiWithTimeout(h.ctx, actFn, opts.Timeout)
	if err != nil {
		k6Throw(h.ctx, "cannot get element's inner text: %w", err)
	}
	applySlowMo(h.ctx)
	return value.(goja.Value).String()
}

func (h *ElementHandle) InputValue(opts goja.Value) string {
	actionOpts := NewElementHandleBaseOptions(h.defaultTimeout())
	if err := actionOpts.Parse(h.ctx, opts); err != nil {
		k6Throw(h.ctx, "cannot parse element input value options: %w", err)
	}
	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.inputValue(apiCtx)
	}
	actFn := elementHandleActionFn(h, []string{}, fn, actionOpts.Force, actionOpts.NoWaitAfter, actionOpts.Timeout)
	value, err := callApiWithTimeout(h.ctx, actFn, actionOpts.Timeout)
	if err != nil {
		k6Throw(h.ctx, "cannot get element's input value: %w", err)
	}
	applySlowMo(h.ctx)
	return value.(goja.Value).String()
}

// IsChecked checks if a checkbox or radio is checked.
func (h *ElementHandle) IsChecked() bool {
	result, err := h.isChecked(h.ctx, 0)
	if err != nil && err != ErrTimedOut { // We don't care anout timeout errors here!
		k6Throw(h.ctx, "cannot handle element is checked: %w", err)
	}
	return result
}

// IsDisabled checks if the element is disabled.
func (h *ElementHandle) IsDisabled() bool {
	result, err := h.isDisabled(h.ctx, 0)
	if err != nil && err != ErrTimedOut { // We don't care anout timeout errors here!
		k6Throw(h.ctx, "cannot handle element is disabled: %w", err)
	}
	return result
}

// IsEditable checks if the element is editable.
func (h *ElementHandle) IsEditable() bool {
	result, err := h.isEditable(h.ctx, 0)
	if err != nil && err != ErrTimedOut { // We don't care anout timeout errors here!
		k6Throw(h.ctx, "cannot handle element is editable: %w", err)
	}
	return result
}

// IsEnabled checks if the element is enabled.
func (h *ElementHandle) IsEnabled() bool {
	result, err := h.isEnabled(h.ctx, 0)
	if err != nil && err != ErrTimedOut { // We don't care anout timeout errors here!
		k6Throw(h.ctx, "cannot handle element is enabled: %w", err)
	}
	return result
}

// IsHidden checks if the element is hidden.
func (h *ElementHandle) IsHidden() bool {
	result, err := h.isHidden(h.ctx, 0)
	if err != nil && err != ErrTimedOut { // We don't care anout timeout errors here!
		k6Throw(h.ctx, "cannot handle element is hidden: %w", err)
	}
	return result
}

// IsVisible checks if the element is visible.
func (h *ElementHandle) IsVisible() bool {
	result, err := h.isVisible(h.ctx, 0)
	if err != nil && err != ErrTimedOut { // We don't care anout timeout errors here!
		k6Throw(h.ctx, "cannot check element is visible: %w", err)
	}
	return result
}

// OwnerFrame returns the frame containing this element.
func (h *ElementHandle) OwnerFrame() api.Frame {
	fn := `
		(node, injected) => {
			return injected.getDocumentElement(node);
		}
	`
	opts := evalOptions{
		forceCallable: true,
		returnByValue: false,
	}
	res, err := h.evalWithScript(h.ctx, opts, fn)
	if err != nil {
		k6Throw(h.ctx, "cannot get document element: %w", err)
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
		k6Throw(h.ctx, "cannot describe owner frame DOM node: %w", err)
	}
	if node == nil || node.FrameID == "" {
		return nil
	}

	return h.frame.manager.getFrameByID(node.FrameID)
}

func (h *ElementHandle) Press(key string, opts goja.Value) {
	parsedOpts := NewElementHandlePressOptions(h.defaultTimeout())
	if err := parsedOpts.Parse(h.ctx, opts); err != nil {
		k6Throw(h.ctx, "cannot parse press options: %v", err)
	}
	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return nil, handle.press(apiCtx, key, NewKeyboardOptions())
	}
	actFn := elementHandleActionFn(h, []string{}, fn, false, parsedOpts.NoWaitAfter, parsedOpts.Timeout)
	_, err := callApiWithTimeout(h.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		k6Throw(h.ctx, "cannot handle element key (%q) press: %v", key, err)
	}
	applySlowMo(h.ctx)
}

// Query runs "element.querySelector" within the page. If no element matches the selector,
// the return value resolves to "null".
func (h *ElementHandle) Query(selector string) api.ElementHandle {
	parsedSelector, err := NewSelector(selector)
	if err != nil {
		k6Throw(h.ctx, "cannot parse selector (%q) in element query: %w", selector, err)
	}
	fn := `
		(node, injected, selector) => {
			return injected.querySelector(selector, node || document, false);
		}
	`
	opts := evalOptions{
		forceCallable: true,
		returnByValue: false,
	}
	result, err := h.evalWithScript(h.ctx, opts, fn, parsedSelector)
	if err != nil {
		k6Throw(h.ctx, "cannot query element for selector (%q): %w", selector, err)
	}
	if result == nil {
		return nil
	}

	var (
		handle  = result.(api.JSHandle)
		element = handle.AsElement()
	)
	applySlowMo(h.ctx)
	if element != nil {
		return element
	}
	handle.Dispose()
	return nil
}

// QueryAll queries element subtree for matching elements. If no element matches the selector,
// the return value resolves to "null".
func (h *ElementHandle) QueryAll(selector string) []api.ElementHandle {
	parsedSelector, err := NewSelector(selector)
	if err != nil {
		k6Throw(h.ctx, "cannot parse selector %q in element query all: %v", selector, err)
	}
	fn := `
		(node, injected, selector) => {
			return injected.querySelectorAll(selector, node || document, false);
		}
	`
	opts := evalOptions{
		forceCallable: true,
		returnByValue: false,
	}
	result, err := h.evalWithScript(h.ctx, opts, fn, parsedSelector)
	if err != nil {
		k6Throw(h.ctx, "cannot evaluate selector %q: %v", selector, err)
	}
	if result == nil {
		return nil
	}

	arrayHandle := result.(api.JSHandle)
	defer arrayHandle.Dispose()
	properties := arrayHandle.GetProperties()
	elements := make([]api.ElementHandle, 0, len(properties))
	for _, property := range properties {
		elementHandle := property.AsElement()
		if elementHandle != nil {
			elements = append(elements, elementHandle)
		} else {
			property.Dispose()
		}
	}
	applySlowMo(h.ctx)
	return elements
}

func (h *ElementHandle) Screenshot(opts goja.Value) goja.ArrayBuffer {
	rt := k6common.GetRuntime(h.ctx)
	parsedOpts := NewElementHandleScreenshotOptions(h.defaultTimeout())
	if err := parsedOpts.Parse(h.ctx, opts); err != nil {
		k6Throw(h.ctx, "cannot parse element screenshot options: %w", err)
	}

	s := newScreenshotter(h.ctx)
	buf, err := s.screenshotElement(h, parsedOpts)
	if err != nil {
		k6Throw(h.ctx, "cannot take screenshot: %w", err)
	}
	return rt.NewArrayBuffer(*buf)
}

func (h *ElementHandle) ScrollIntoViewIfNeeded(opts goja.Value) {
	actionOpts := NewElementHandleBaseOptions(h.defaultTimeout())
	if err := actionOpts.Parse(h.ctx, opts); err != nil {
		k6Throw(h.ctx, "cannot parse element scroll into view options: %w", err)
	}
	err := h.waitAndScrollIntoViewIfNeeded(h.ctx, actionOpts.Force, actionOpts.NoWaitAfter, actionOpts.Timeout)
	if err != nil {
		k6Throw(h.ctx, "cannot handle element scroll into view: %w", err)
	}
	applySlowMo(h.ctx)
}

func (h *ElementHandle) SelectOption(values goja.Value, opts goja.Value) []string {
	rt := k6common.GetRuntime(h.ctx)
	actionOpts := NewElementHandleBaseOptions(h.defaultTimeout())
	if err := actionOpts.Parse(h.ctx, opts); err != nil {
		k6Throw(h.ctx, "cannot parse element selection options: %w", err)
	}
	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.selectOption(apiCtx, values)
	}
	actFn := elementHandleActionFn(h, []string{}, fn, actionOpts.Force, actionOpts.NoWaitAfter, actionOpts.Timeout)
	selectedOptions, err := callApiWithTimeout(h.ctx, actFn, actionOpts.Timeout)
	if err != nil {
		k6Throw(h.ctx, "cannot handle element select option: %w", err)
	}
	var returnVal []string
	if err := rt.ExportTo(selectedOptions.(goja.Value), &returnVal); err != nil {
		k6Throw(h.ctx, "cannot unpack options in element select option: %w", err)
	}
	applySlowMo(h.ctx)
	return returnVal
}

func (h *ElementHandle) SelectText(opts goja.Value) {
	actionOpts := NewElementHandleBaseOptions(h.defaultTimeout())
	if err := actionOpts.Parse(h.ctx, opts); err != nil {
		k6Throw(h.ctx, "cannot parse element select text options: %w", err)
	}
	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return nil, handle.selectText(apiCtx)
	}
	actFn := elementHandleActionFn(h, []string{}, fn, actionOpts.Force, actionOpts.NoWaitAfter, actionOpts.Timeout)
	_, err := callApiWithTimeout(h.ctx, actFn, actionOpts.Timeout)
	if err != nil {
		k6Throw(h.ctx, "cannot select element text: %w", err)
	}
	applySlowMo(h.ctx)
}

// SetChecked checks or unchecks an element.
func (h *ElementHandle) SetChecked(checked bool, opts goja.Value) {
	parsedOpts := NewElementHandleSetCheckedOptions(h.defaultTimeout())
	err := parsedOpts.Parse(h.ctx, opts)
	if err != nil {
		k6Throw(h.ctx, "cannot parse element set checked options: %w", err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle, p *Position) (interface{}, error) {
		return nil, handle.setChecked(apiCtx, checked, p)
	}
	pointerFn := elementHandlePointerActionFn(h, true, fn, &parsedOpts.ElementHandleBasePointerOptions)
	_, err = callApiWithTimeout(h.ctx, pointerFn, parsedOpts.Timeout)
	if err != nil {
		k6Throw(h.ctx, "cannot check element: %w", err)
	}
	applySlowMo(h.ctx)
}

func (h *ElementHandle) SetInputFiles(files goja.Value, opts goja.Value) {
	// TODO: implement
	k6Throw(h.ctx, "ElementHandle.setInputFiles() has not been implemented yet")
}

func (h *ElementHandle) Tap(opts goja.Value) {
	parsedOpts := NewElementHandleTapOptions(h.defaultTimeout())
	err := parsedOpts.Parse(h.ctx, opts)
	if err != nil {
		k6Throw(h.ctx, "cannot parse element tap options: %w", err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle, p *Position) (interface{}, error) {
		return nil, handle.tap(apiCtx, p)
	}
	pointerFn := elementHandlePointerActionFn(h, true, fn, &parsedOpts.ElementHandleBasePointerOptions)
	_, err = callApiWithTimeout(h.ctx, pointerFn, parsedOpts.Timeout)
	if err != nil {
		k6Throw(h.ctx, "cannot tap element: %w", err)
	}
	applySlowMo(h.ctx)
}

func (h *ElementHandle) TextContent() string {
	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.textContent(apiCtx)
	}
	opts := NewElementHandleBaseOptions(h.defaultTimeout())
	actFn := elementHandleActionFn(h, []string{}, fn, opts.Force, opts.NoWaitAfter, opts.Timeout)
	value, err := callApiWithTimeout(h.ctx, actFn, opts.Timeout)
	if err != nil {
		k6Throw(h.ctx, "cannot get text content of element: %w", err)
	}
	applySlowMo(h.ctx)
	return value.(goja.Value).String()
}

// Type scrolls element into view, focuses element and types text.
func (h *ElementHandle) Type(text string, opts goja.Value) {
	parsedOpts := NewElementHandleTypeOptions(h.defaultTimeout())
	if err := parsedOpts.Parse(h.ctx, opts); err != nil {
		k6Throw(h.ctx, "cannot parse element handle type options: %v", err)
	}
	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return nil, handle.typ(apiCtx, text, NewKeyboardOptions())
	}
	actFn := elementHandleActionFn(h, []string{}, fn, false, parsedOpts.NoWaitAfter, parsedOpts.Timeout)
	_, err := callApiWithTimeout(h.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		k6Throw(h.ctx, "cannot type (%q) into element: %w", text, err)
	}
	applySlowMo(h.ctx)
}

// Uncheck scrolls element into view and clicks in the center of the element.
func (h *ElementHandle) Uncheck(opts goja.Value) {
	h.SetChecked(false, opts)
}

func (h *ElementHandle) WaitForElementState(state string, opts goja.Value) {
	parsedOpts := NewElementHandleWaitForElementStateOptions(time.Duration(h.frame.manager.timeoutSettings.timeout()) * time.Second)
	err := parsedOpts.Parse(h.ctx, opts)
	if err != nil {
		k6Throw(h.ctx, "cannot parse element wait for state options: %w", err)
	}
	_, err = h.waitForElementState(h.ctx, []string{state}, parsedOpts.Timeout)
	if err != nil {
		k6Throw(h.ctx, "error while waiting for state: %w", err)
	}
}

func (h *ElementHandle) WaitForSelector(selector string, opts goja.Value) api.ElementHandle {
	parsedOpts := NewFrameWaitForSelectorOptions(h.defaultTimeout())
	if err := parsedOpts.Parse(h.ctx, opts); err != nil {
		k6Throw(h.ctx, "cannot parse element wait for selector options: %w", err)
	}

	handle, err := h.waitForSelector(h.ctx, selector, parsedOpts)
	if err != nil {
		k6Throw(h.ctx, "error while waiting for selector (%q): %w", selector, err)
	}

	return handle
}

// evalWithScript evaluates the given js code in the scope of this ElementHandle and returns the result.
// The js code can call helper functions from injected_script.js.
func (h *ElementHandle) evalWithScript(
	ctx context.Context,
	opts evalOptions, js string, args ...interface{},
) (interface{}, error) {
	script, err := h.execCtx.getInjectedScript(h.ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot get injected script: %w", err)
	}
	args = append([]interface{}{script}, args...)
	return h.eval(ctx, opts, js, args...)
}

// eval evaluates the given js code in the scope of this ElementHandle and returns the result.
func (h *ElementHandle) eval(
	ctx context.Context,
	opts evalOptions, js string, args ...interface{},
) (interface{}, error) {
	// passing `h` makes it evaluate js code in the element handle's scope.
	args = append([]interface{}{h}, args...)
	rt := k6common.GetRuntime(ctx)
	gargs := make([]goja.Value, len(args))
	for i, arg := range args {
		gargs[i] = rt.ToValue(arg)
	}
	result, err := h.execCtx.eval(ctx, opts, rt.ToValue(js), gargs...)
	if err != nil {
		err = fmt.Errorf("element handle cannot evaluate: %w", err)
	}
	return result, err
}
