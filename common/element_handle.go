package common

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"time"

	"github.com/grafana/xk6-browser/api"
	"github.com/grafana/xk6-browser/common/js"
	"github.com/grafana/xk6-browser/k6ext"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	cdppage "github.com/chromedp/cdproto/page"
	"github.com/dop251/goja"
)

const resultDone = "done"

// Ensure ElementHandle implements the api.ElementHandle and api.JSHandle interfaces.
var _ api.ElementHandle = &ElementHandle{}
var _ api.JSHandle = &ElementHandle{}

type (
	elementHandleActionFunc        func(context.Context, *ElementHandle) (interface{}, error)
	elementHandlePointerActionFunc func(context.Context, *ElementHandle, *Position) (interface{}, error)
	retryablePointerActionFunc     func(context.Context, *ScrollIntoViewOptions) (interface{}, error)

	// evalFunc is a common interface for both evalWithScript and eval.
	// It helps abstracting these methods to aid with testing.
	evalFunc func(ctx context.Context, opts evalOptions, js string, args ...interface{}) (interface{}, error)
)

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
		return nil, fmt.Errorf("getting bounding box model of DOM node: %w", err)
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
		var (
			el          = h.frame.FrameElement()
			element, ok = el.(*ElementHandle)
		)
		if !ok {
			return false, fmt.Errorf("unexpected type %T", el)
		}
		box, err := element.boundingBox()
		if err != nil {
			return false, err
		}
		if box == nil {
			return false, errors.New("getting bounding box of element")
		}
		// Translate from viewport coordinates to frame coordinates.
		point.X -= box.X
		point.Y -= box.Y
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

	// Either we're done or an error happened (returned as "error:..." from JS)
	const done = "done"
	v, ok := result.(goja.Value)
	if !ok {
		return false, fmt.Errorf("unexpected type %T", result)
	}
	if v.ExportType().Kind() != reflect.String {
		// We got a { hitTargetDescription: ... } result
		// Meaning: Another element is preventing pointer events.
		//
		// It's safe to count an object return as an interception.
		// We just don't interpret what is intercepting with the target element
		// because we don't need any more functionality from this JS function
		// right now.
		return false, errorFromDOMError("error:intercept")
	} else if v.String() != done {
		return false, errorFromDOMError(v.String())
	}

	return true, nil
}

func (h *ElementHandle) checkElementState(_ context.Context, state string) (*bool, error) {
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
	v, ok := result.(goja.Value)
	if !ok {
		return nil, fmt.Errorf("unexpected type %T", result)
	}
	//nolint:exhaustive
	switch v.ExportType().Kind() {
	case reflect.String: // An error happened (returned as "error:..." from JS)
		return nil, errorFromDOMError(v.String())
	case reflect.Bool:
		returnVal := new(bool)
		*returnVal = v.ToBoolean()
		return returnVal, nil
	}

	return nil, fmt.Errorf(
		"checking state %q of element: %q", state, reflect.TypeOf(result))
}

func (h *ElementHandle) click(p *Position, opts *MouseClickOptions) error {
	return h.frame.page.Mouse.click(p.X, p.Y, opts)
}

func (h *ElementHandle) clickablePoint() (*Position, error) {
	var (
		quads []dom.Quad
		err   error
	)
	getContentQuads := dom.GetContentQuads().WithObjectID(h.remoteObject.ObjectID)
	if quads, err = getContentQuads.Do(cdp.WithExecutor(h.ctx, h.session)); err != nil {
		return nil, fmt.Errorf("getting node content quads %T: %w", getContentQuads, err)
	}
	if len(quads) == 0 {
		return nil, fmt.Errorf("node is either not visible or not an HTMLElement: %w", err)
	}

	// Filter out quads that have too small area to click into.
	var layoutViewport *cdppage.LayoutViewport
	getLayoutMetrics := cdppage.GetLayoutMetrics()
	if _, _, _, layoutViewport, _, _, err = getLayoutMetrics.Do(cdp.WithExecutor(h.ctx, h.session)); err != nil {
		return nil, fmt.Errorf("getting page layout metrics %T: %w", getLayoutMetrics, err)
	}

	return filterQuads(layoutViewport.ClientWidth, layoutViewport.ClientHeight, quads)
}

func filterQuads(viewportWidth, viewportHeight int64, quads []dom.Quad) (*Position, error) {
	var filteredQuads []dom.Quad
	for _, q := range quads {
		// Keep the points within the viewport and positive.
		nq := q
		for i := 0; i < len(q); i += 2 {
			nq[i] = math.Min(math.Max(q[i], 0), float64(viewportWidth))
			nq[i+1] = math.Min(math.Max(q[i+1], 0), float64(viewportHeight))
		}
		// Compute sum of all directed areas of adjacent triangles
		// https://en.wikipedia.org/wiki/Polygon#Area
		var area float64
		for i := 0; i < len(q); i += 2 {
			p2 := (i + 2) % (len(q) / 2)
			area += (q[i]*q[p2+1] - q[p2]*q[i+1]) / 2
		}
		// We don't want to click on something less than a pixel.
		if math.Abs(area) > 0.99 {
			filteredQuads = append(filteredQuads, q)
		}
	}
	if len(filteredQuads) == 0 {
		return nil, errors.New("node is either not visible or not an HTMLElement")
	}

	// Return the middle point of the first quad.
	var (
		first = filteredQuads[0]
		l     = len(first)
		n     = (float64(l) / 2)
		p     Position
	)
	for i := 0; i < l; i += 2 {
		p.X += first[i] / n
		p.Y += first[i+1] / n
	}

	p = compensateHalfIntegerRoundingError(p)

	return &p, nil
}

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
func compensateHalfIntegerRoundingError(p Position) Position {
	if rx := p.X - math.Floor(p.X); rx > 0.49 && rx < 0.51 {
		p.X -= 0.02
	}
	if ry := p.Y - math.Floor(p.Y); ry > 0.49 && ry < 0.51 {
		p.Y -= 0.02
	}
	return p
}

func (h *ElementHandle) dblClick(p *Position, opts *MouseClickOptions) error {
	return h.frame.page.Mouse.click(p.X, p.Y, opts)
}

func (h *ElementHandle) defaultTimeout() time.Duration {
	return time.Duration(h.frame.manager.timeoutSettings.timeout()) * time.Second
}

func (h *ElementHandle) dispatchEvent(_ context.Context, typ string, eventInit goja.Value) (interface{}, error) {
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

func (h *ElementHandle) fill(_ context.Context, value string) error {
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
		return err
	}
	v, ok := result.(goja.Value)
	if !ok {
		return fmt.Errorf("unexpected type %T", result)
	}
	if s := v.String(); s != resultDone {
		// Either we're done or an error happened (returned as "error:..." from JS)
		return errorFromDOMError(s)
	}

	return nil
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

	rt := h.execCtx.vu.Runtime()
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
			rt   = h.execCtx.vu.Runtime()
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

func (h *ElementHandle) tap(apiCtx context.Context, p *Position) error {
	return h.frame.page.Touchscreen.tap(p.X, p.Y)
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
	actFn := h.newAction([]string{"visible", "stable"}, fn, force, noWaitAfter, timeout)
	_, err := callApiWithTimeout(h.ctx, actFn, timeout)
	if err != nil {
		return err
	}
	return nil
}

func (h *ElementHandle) waitForElementState(
	apiCtx context.Context, states []string, timeout time.Duration,
) (bool, error) {
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
		return false, errorFromDOMError(err.Error())
	}
	v, ok := result.(goja.Value)
	if !ok {
		return false, fmt.Errorf("unexpected type %T", result)
	}
	//nolint:exhaustive
	switch v.ExportType().Kind() {
	case reflect.String: // Either we're done or an error happened (returned as "error:..." from JS)
		if v.String() == "done" {
			return true, nil
		}
		return false, errorFromDOMError(v.String())
	case reflect.Bool:
		return v.ToBoolean(), nil
	}

	return false, fmt.Errorf(
		"checking states %v of element: %q", states, reflect.TypeOf(result))
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
		return nil // Don't panic here, just return nil
	}
	return bbox.toApiRect()
}

// Click scrolls element into view and clicks in the center of the element
// TODO: look into making more robust using retries (see: https://github.com/microsoft/playwright/blob/master/src/server/dom.ts#L298)
func (h *ElementHandle) Click(opts goja.Value) {
	actionOpts := NewElementHandleClickOptions(h.defaultTimeout())
	if err := actionOpts.Parse(h.ctx, opts); err != nil {
		k6ext.Panic(h.ctx, "parsing element click options: %v", err)
	}
	fn := func(apiCtx context.Context, handle *ElementHandle, p *Position) (interface{}, error) {
		return nil, handle.click(p, actionOpts.ToMouseClickOptions())
	}
	pointerFn := h.newPointerAction(fn, &actionOpts.ElementHandleBasePointerOptions)
	_, err := callApiWithTimeout(h.ctx, pointerFn, actionOpts.Timeout)
	if err != nil {
		k6ext.Panic(h.ctx, "clicking on element: %v", err)
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
		k6ext.Panic(h.ctx, "getting remote node %q: %w", h.remoteObject.ObjectID, err)
	}
	if node == nil || node.FrameID == "" {
		return nil
	}

	return h.frame.manager.getFrameByID(node.FrameID)
}

func (h *ElementHandle) Dblclick(opts goja.Value) {
	actionOpts := NewElementHandleDblclickOptions(h.defaultTimeout())
	if err := actionOpts.Parse(h.ctx, opts); err != nil {
		k6ext.Panic(h.ctx, "parsing element double click options: %w", err)
	}
	fn := func(apiCtx context.Context, handle *ElementHandle, p *Position) (interface{}, error) {
		return nil, handle.dblClick(p, actionOpts.ToMouseClickOptions())
	}
	pointerFn := h.newPointerAction(fn, &actionOpts.ElementHandleBasePointerOptions)
	_, err := callApiWithTimeout(h.ctx, pointerFn, actionOpts.Timeout)
	if err != nil {
		k6ext.Panic(h.ctx, "double clicking on element: %w", err)
	}
	applySlowMo(h.ctx)
}

func (h *ElementHandle) DispatchEvent(typ string, eventInit goja.Value) {
	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.dispatchEvent(apiCtx, typ, eventInit)
	}
	opts := NewElementHandleBaseOptions(h.defaultTimeout())
	actFn := h.newAction([]string{}, fn, opts.Force, opts.NoWaitAfter, opts.Timeout)
	_, err := callApiWithTimeout(h.ctx, actFn, opts.Timeout)
	if err != nil {
		k6ext.Panic(h.ctx, "dispatching element event: %w", err)
	}
	applySlowMo(h.ctx)
}

func (h *ElementHandle) Fill(value string, opts goja.Value) {
	actionOpts := NewElementHandleBaseOptions(h.defaultTimeout())
	if err := actionOpts.Parse(h.ctx, opts); err != nil {
		k6ext.Panic(h.ctx, "parsing element fill options: %w", err)
	}
	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return nil, handle.fill(apiCtx, value)
	}
	actFn := h.newAction([]string{"visible", "enabled", "editable"},
		fn, actionOpts.Force, actionOpts.NoWaitAfter, actionOpts.Timeout)
	_, err := callApiWithTimeout(h.ctx, actFn, actionOpts.Timeout)
	if err != nil {
		k6ext.Panic(h.ctx, "handling element fill action: %w", err)
	}
	applySlowMo(h.ctx)
}

// Focus scrolls element into view and focuses the element.
func (h *ElementHandle) Focus() {
	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return nil, handle.focus(apiCtx, false)
	}
	opts := NewElementHandleBaseOptions(h.defaultTimeout())
	actFn := h.newAction([]string{}, fn, opts.Force, opts.NoWaitAfter, opts.Timeout)
	_, err := callApiWithTimeout(h.ctx, actFn, opts.Timeout)
	if err != nil {
		k6ext.Panic(h.ctx, "focusing on element: %w", err)
	}
	applySlowMo(h.ctx)
}

// GetAttribute retrieves the value of specified element attribute.
func (h *ElementHandle) GetAttribute(name string) goja.Value {
	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.getAttribute(apiCtx, name)
	}
	opts := NewElementHandleBaseOptions(h.defaultTimeout())
	actFn := h.newAction([]string{}, fn, opts.Force, opts.NoWaitAfter, opts.Timeout)
	v, err := callApiWithTimeout(h.ctx, actFn, opts.Timeout)
	if err != nil {
		k6ext.Panic(h.ctx, "getting attribute of %q: %q", name, err)
	}
	applySlowMo(h.ctx)

	return asGojaValue(h.ctx, v)
}

// Hover scrolls element into view and hovers over its center point.
func (h *ElementHandle) Hover(opts goja.Value) {
	actionOpts := NewElementHandleHoverOptions(h.defaultTimeout())
	if err := actionOpts.Parse(h.ctx, opts); err != nil {
		k6ext.Panic(h.ctx, "parsing element hover options: %w", err)
	}
	fn := func(apiCtx context.Context, handle *ElementHandle, p *Position) (interface{}, error) {
		return nil, handle.hover(apiCtx, p)
	}
	pointerFn := h.newPointerAction(fn, &actionOpts.ElementHandleBasePointerOptions)
	_, err := callApiWithTimeout(h.ctx, pointerFn, actionOpts.Timeout)
	if err != nil {
		k6ext.Panic(h.ctx, "hovering on element: %w", err)
	}
	applySlowMo(h.ctx)
}

// InnerHTML returns the inner HTML of the element.
func (h *ElementHandle) InnerHTML() string {
	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.innerHTML(apiCtx)
	}
	opts := NewElementHandleBaseOptions(h.defaultTimeout())
	actFn := h.newAction([]string{}, fn, opts.Force, opts.NoWaitAfter, opts.Timeout)
	v, err := callApiWithTimeout(h.ctx, actFn, opts.Timeout)
	if err != nil {
		k6ext.Panic(h.ctx, "getting element's inner HTML: %w", err)
	}
	applySlowMo(h.ctx)

	return gojaValueToString(h.ctx, v)
}

// InnerText returns the inner text of the element.
func (h *ElementHandle) InnerText() string {
	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.innerText(apiCtx)
	}
	opts := NewElementHandleBaseOptions(h.defaultTimeout())
	actFn := h.newAction([]string{}, fn, opts.Force, opts.NoWaitAfter, opts.Timeout)
	v, err := callApiWithTimeout(h.ctx, actFn, opts.Timeout)
	if err != nil {
		k6ext.Panic(h.ctx, "getting element's inner text: %w", err)
	}
	applySlowMo(h.ctx)

	return gojaValueToString(h.ctx, v)
}

func (h *ElementHandle) InputValue(opts goja.Value) string {
	actionOpts := NewElementHandleBaseOptions(h.defaultTimeout())
	if err := actionOpts.Parse(h.ctx, opts); err != nil {
		k6ext.Panic(h.ctx, "parsing element input value options: %w", err)
	}
	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.inputValue(apiCtx)
	}
	actFn := h.newAction([]string{}, fn, actionOpts.Force, actionOpts.NoWaitAfter, actionOpts.Timeout)
	v, err := callApiWithTimeout(h.ctx, actFn, actionOpts.Timeout)
	if err != nil {
		k6ext.Panic(h.ctx, "getting element's input value: %w", err)
	}
	applySlowMo(h.ctx)

	return gojaValueToString(h.ctx, v)
}

// IsChecked checks if a checkbox or radio is checked.
func (h *ElementHandle) IsChecked() bool {
	result, err := h.isChecked(h.ctx, 0)
	if err != nil && err != ErrTimedOut { // We don't care anout timeout errors here!
		k6ext.Panic(h.ctx, "element isChecked: %w", err)
	}
	return result
}

// IsDisabled checks if the element is disabled.
func (h *ElementHandle) IsDisabled() bool {
	result, err := h.isDisabled(h.ctx, 0)
	if err != nil && err != ErrTimedOut { // We don't care anout timeout errors here!
		k6ext.Panic(h.ctx, "element isDisabled: %w", err)
	}
	return result
}

// IsEditable checks if the element is editable.
func (h *ElementHandle) IsEditable() bool {
	result, err := h.isEditable(h.ctx, 0)
	if err != nil && err != ErrTimedOut { // We don't care anout timeout errors here!
		k6ext.Panic(h.ctx, "element isEditable: %w", err)
	}
	return result
}

// IsEnabled checks if the element is enabled.
func (h *ElementHandle) IsEnabled() bool {
	result, err := h.isEnabled(h.ctx, 0)
	if err != nil && err != ErrTimedOut { // We don't care anout timeout errors here!
		k6ext.Panic(h.ctx, "element isEnabled: %w", err)
	}
	return result
}

// IsHidden checks if the element is hidden.
func (h *ElementHandle) IsHidden() bool {
	result, err := h.isHidden(h.ctx, 0)
	if err != nil && err != ErrTimedOut { // We don't care anout timeout errors here!
		k6ext.Panic(h.ctx, "element isHidden: %w", err)
	}
	return result
}

// IsVisible checks if the element is visible.
func (h *ElementHandle) IsVisible() bool {
	result, err := h.isVisible(h.ctx, 0)
	if err != nil && err != ErrTimedOut { // We don't care anout timeout errors here!
		k6ext.Panic(h.ctx, "element isVisible: %w", err)
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
		k6ext.Panic(h.ctx, "getting document element: %w", err)
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
		k6ext.Panic(h.ctx, "describing owner frame DOM node: %w", err)
	}
	if node == nil || node.FrameID == "" {
		return nil
	}

	return h.frame.manager.getFrameByID(node.FrameID)
}

func (h *ElementHandle) Press(key string, opts goja.Value) {
	parsedOpts := NewElementHandlePressOptions(h.defaultTimeout())
	if err := parsedOpts.Parse(h.ctx, opts); err != nil {
		k6ext.Panic(h.ctx, "parsing press %q options: %v", key, err)
	}
	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return nil, handle.press(apiCtx, key, NewKeyboardOptions())
	}
	actFn := h.newAction([]string{}, fn, false, parsedOpts.NoWaitAfter, parsedOpts.Timeout)
	_, err := callApiWithTimeout(h.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		k6ext.Panic(h.ctx, "pressing %q: %v", key, err)
	}
	applySlowMo(h.ctx)
}

// Query runs "element.querySelector" within the page. If no element matches the selector,
// the return value resolves to "null".
func (h *ElementHandle) Query(selector string) api.ElementHandle {
	parsedSelector, err := NewSelector(selector)
	if err != nil {
		k6ext.Panic(h.ctx, "parsing selector %q: %w", selector, err)
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
		k6ext.Panic(h.ctx, "querying selector %q: %w", selector, err)
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

// QueryAll queries element subtree for matching elements.
// If no element matches the selector, the return value resolves to "null".
func (h *ElementHandle) QueryAll(selector string) []api.ElementHandle {
	defer applySlowMo(h.ctx)

	handles, err := h.queryAll(selector, h.evalWithScript)
	if err != nil {
		k6ext.Panic(h.ctx, "QueryAll: %w", err)
	}

	return handles
}

func (h *ElementHandle) queryAll(selector string, eval evalFunc) ([]api.ElementHandle, error) {
	parsedSelector, err := NewSelector(selector)
	if err != nil {
		return nil, fmt.Errorf("parsing selector %q: %w", selector, err)
	}
	result, err := eval(
		h.ctx,
		evalOptions{forceCallable: true, returnByValue: false},
		js.QueryAll,
		parsedSelector,
	)
	if err != nil {
		return nil, fmt.Errorf("querying selector %q: %w", selector, err)
	}
	if result == nil {
		// it is ok to return a nil slice because it means we didn't find any elements.
		return nil, nil
	}

	handles, ok := result.(api.JSHandle)
	if !ok {
		return nil, fmt.Errorf("getting element handle for selector %q: %w", selector, ErrJSHandleInvalid)
	}
	defer handles.Dispose()
	var (
		props = handles.GetProperties()
		els   = make([]api.ElementHandle, 0, len(props))
	)
	for _, prop := range props {
		if el := prop.AsElement(); el != nil {
			els = append(els, el)
		} else {
			prop.Dispose()
		}
	}

	return els, nil
}

// SetChecked checks or unchecks an element.
func (h *ElementHandle) SetChecked(checked bool, opts goja.Value) {
	parsedOpts := NewElementHandleSetCheckedOptions(h.defaultTimeout())
	err := parsedOpts.Parse(h.ctx, opts)
	if err != nil {
		k6ext.Panic(h.ctx, "parsing setChecked options: %w", err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle, p *Position) (interface{}, error) {
		return nil, handle.setChecked(apiCtx, checked, p)
	}
	pointerFn := h.newPointerAction(fn, &parsedOpts.ElementHandleBasePointerOptions)
	_, err = callApiWithTimeout(h.ctx, pointerFn, parsedOpts.Timeout)
	if err != nil {
		k6ext.Panic(h.ctx, "checking element: %w", err)
	}
	applySlowMo(h.ctx)
}

// Uncheck scrolls element into view, and if it's an input element of type
// checkbox that is already checked, clicks on it to mark it as unchecked.
func (h *ElementHandle) Uncheck(opts goja.Value) {
	h.SetChecked(false, opts)
}

// Check scrolls element into view, and if it's an input element of type
// checkbox that is unchecked, clicks on it to mark it as checked.
func (h *ElementHandle) Check(opts goja.Value) {
	h.SetChecked(true, opts)
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

func (h *ElementHandle) Screenshot(opts goja.Value) goja.ArrayBuffer {
	rt := h.execCtx.vu.Runtime()
	parsedOpts := NewElementHandleScreenshotOptions(h.defaultTimeout())
	if err := parsedOpts.Parse(h.ctx, opts); err != nil {
		k6ext.Panic(h.ctx, "parsing screenshot options: %w", err)
	}

	s := newScreenshotter(h.ctx)
	buf, err := s.screenshotElement(h, parsedOpts)
	if err != nil {
		k6ext.Panic(h.ctx, "taking screenshot: %w", err)
	}
	return rt.NewArrayBuffer(*buf)
}

func (h *ElementHandle) ScrollIntoViewIfNeeded(opts goja.Value) {
	actionOpts := NewElementHandleBaseOptions(h.defaultTimeout())
	if err := actionOpts.Parse(h.ctx, opts); err != nil {
		k6ext.Panic(h.ctx, "parsing scrollIntoViewIfNeeded options: %w", err)
	}
	err := h.waitAndScrollIntoViewIfNeeded(h.ctx, actionOpts.Force, actionOpts.NoWaitAfter, actionOpts.Timeout)
	if err != nil {
		k6ext.Panic(h.ctx, "scrolling element into view: %w", err)
	}
	applySlowMo(h.ctx)
}

func (h *ElementHandle) SelectOption(values goja.Value, opts goja.Value) []string {
	rt := h.execCtx.vu.Runtime()
	actionOpts := NewElementHandleBaseOptions(h.defaultTimeout())
	if err := actionOpts.Parse(h.ctx, opts); err != nil {
		k6ext.Panic(h.ctx, "parsing selectOption options: %w", err)
	}
	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.selectOption(apiCtx, values)
	}
	actFn := h.newAction([]string{}, fn, actionOpts.Force, actionOpts.NoWaitAfter, actionOpts.Timeout)
	selectedOptions, err := callApiWithTimeout(h.ctx, actFn, actionOpts.Timeout)
	if err != nil {
		k6ext.Panic(h.ctx, "selecting options: %w", err)
	}
	var returnVal []string
	if err := rt.ExportTo(asGojaValue(h.ctx, selectedOptions), &returnVal); err != nil {
		k6ext.Panic(h.ctx, "unpacking selected options: %w", err)
	}

	applySlowMo(h.ctx)

	return returnVal
}

func (h *ElementHandle) SelectText(opts goja.Value) {
	actionOpts := NewElementHandleBaseOptions(h.defaultTimeout())
	if err := actionOpts.Parse(h.ctx, opts); err != nil {
		k6ext.Panic(h.ctx, "parsing selectText options: %w", err)
	}
	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return nil, handle.selectText(apiCtx)
	}
	actFn := h.newAction([]string{}, fn, actionOpts.Force, actionOpts.NoWaitAfter, actionOpts.Timeout)
	_, err := callApiWithTimeout(h.ctx, actFn, actionOpts.Timeout)
	if err != nil {
		k6ext.Panic(h.ctx, "selecting text: %w", err)
	}
	applySlowMo(h.ctx)
}

func (h *ElementHandle) SetInputFiles(files goja.Value, opts goja.Value) {
	// TODO: implement
	k6ext.Panic(h.ctx, "ElementHandle.setInputFiles() has not been implemented yet")
}

func (h *ElementHandle) Tap(opts goja.Value) {
	parsedOpts := NewElementHandleTapOptions(h.defaultTimeout())
	err := parsedOpts.Parse(h.ctx, opts)
	if err != nil {
		k6ext.Panic(h.ctx, "parsing tap options: %w", err)
	}

	fn := func(apiCtx context.Context, handle *ElementHandle, p *Position) (interface{}, error) {
		return nil, handle.tap(apiCtx, p)
	}
	pointerFn := h.newPointerAction(fn, &parsedOpts.ElementHandleBasePointerOptions)
	_, err = callApiWithTimeout(h.ctx, pointerFn, parsedOpts.Timeout)
	if err != nil {
		k6ext.Panic(h.ctx, "tapping element: %w", err)
	}
	applySlowMo(h.ctx)
}

func (h *ElementHandle) TextContent() string {
	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return handle.textContent(apiCtx)
	}
	opts := NewElementHandleBaseOptions(h.defaultTimeout())
	actFn := h.newAction([]string{}, fn, opts.Force, opts.NoWaitAfter, opts.Timeout)
	v, err := callApiWithTimeout(h.ctx, actFn, opts.Timeout)
	if err != nil {
		k6ext.Panic(h.ctx, "getting text content of element: %w", err)
	}
	applySlowMo(h.ctx)

	return gojaValueToString(h.ctx, v)
}

// Type scrolls element into view, focuses element and types text.
func (h *ElementHandle) Type(text string, opts goja.Value) {
	parsedOpts := NewElementHandleTypeOptions(h.defaultTimeout())
	if err := parsedOpts.Parse(h.ctx, opts); err != nil {
		k6ext.Panic(h.ctx, "parsing type options: %v", err)
	}
	fn := func(apiCtx context.Context, handle *ElementHandle) (interface{}, error) {
		return nil, handle.typ(apiCtx, text, NewKeyboardOptions())
	}
	actFn := h.newAction([]string{}, fn, false, parsedOpts.NoWaitAfter, parsedOpts.Timeout)
	_, err := callApiWithTimeout(h.ctx, actFn, parsedOpts.Timeout)
	if err != nil {
		k6ext.Panic(h.ctx, "typing text %q: %w", text, err)
	}
	applySlowMo(h.ctx)
}

func (h *ElementHandle) WaitForElementState(state string, opts goja.Value) {
	parsedOpts := NewElementHandleWaitForElementStateOptions(h.defaultTimeout())
	err := parsedOpts.Parse(h.ctx, opts)
	if err != nil {
		k6ext.Panic(h.ctx, "parsing waitForElementState options: %w", err)
	}
	_, err = h.waitForElementState(h.ctx, []string{state}, parsedOpts.Timeout)
	if err != nil {
		k6ext.Panic(h.ctx, "waiting for element state %q: %w", state, err)
	}
}

func (h *ElementHandle) WaitForSelector(selector string, opts goja.Value) api.ElementHandle {
	parsedOpts := NewFrameWaitForSelectorOptions(h.defaultTimeout())
	if err := parsedOpts.Parse(h.ctx, opts); err != nil {
		k6ext.Panic(h.ctx, "parsing waitForSelector %q options: %w", selector, err)
	}

	handle, err := h.waitForSelector(h.ctx, selector, parsedOpts)
	if err != nil {
		k6ext.Panic(h.ctx, "waiting for selector %q: %w", selector, err)
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
		return nil, fmt.Errorf("getting injected script: %w", err)
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
	result, err := h.execCtx.eval(ctx, opts, js, args...)
	if err != nil {
		return nil, err
	}
	return result, err
}

func (h *ElementHandle) newAction(
	states []string, fn elementHandleActionFunc, force, noWaitAfter bool, timeout time.Duration,
) func(apiCtx context.Context, resultCh chan interface{}, errCh chan error) {
	// All or a subset of the following actionability checks are made before performing the actual action:
	// 1. Attached to DOM
	// 2. Visible
	// 3. Stable
	// 4. Enabled
	actionFn := func(apiCtx context.Context) (interface{}, error) {
		// Check if we should run actionability checks
		if !force {
			if _, err := h.waitForElementState(apiCtx, states, timeout); err != nil {
				return nil, err
			}
		}

		b := NewBarrier()
		h.frame.manager.addBarrier(b)
		defer h.frame.manager.removeBarrier(b)

		res, err := fn(apiCtx, h)
		if err != nil {
			return nil, err
		}
		// Do we need to wait for navigation to happen
		if !noWaitAfter {
			if err := b.Wait(apiCtx); err != nil {
				return nil, err
			}
		}

		return res, nil
	}

	return func(apiCtx context.Context, resultCh chan interface{}, errCh chan error) {
		if res, err := actionFn(apiCtx); err != nil {
			errCh <- err
		} else {
			resultCh <- res
		}
	}
}

//nolint:funlen,gocognit,cyclop
func (h *ElementHandle) newPointerAction(
	fn elementHandlePointerActionFunc, opts *ElementHandleBasePointerOptions,
) func(apiCtx context.Context, resultCh chan interface{}, errCh chan error) {
	// All or a subset of the following actionability checks are made before performing the actual action:
	// 1. Attached to DOM
	// 2. Visible
	// 3. Stable
	// 4. Enabled
	// 5. Receives events
	pointerFn := func(apiCtx context.Context, sopts *ScrollIntoViewOptions) (res interface{}, err error) {
		// Check if we should run actionability checks
		if !opts.Force {
			states := []string{"visible", "stable", "enabled"}
			if _, err = h.waitForElementState(apiCtx, states, opts.Timeout); err != nil {
				return nil, fmt.Errorf("waiting for element state: %w", err)
			}
		}

		// Decide position where a mouse down should happen if needed by action
		p := opts.Position

		// Change scrolling action depending on the scrolling options
		if sopts == nil {
			var rect *dom.Rect
			if p != nil {
				rect = &dom.Rect{X: p.X, Y: p.Y}
			}
			err = h.scrollRectIntoViewIfNeeded(apiCtx, rect)
		} else {
			_, err = h.eval(
				apiCtx,
				evalOptions{forceCallable: true, returnByValue: false},
				js.ScrollIntoView,
				sopts,
			)
		}
		if err != nil {
			return nil, fmt.Errorf("scrolling element into view: %w", err)
		}

		// Get the clickable point
		if p != nil {
			p, err = h.offsetPosition(apiCtx, opts.Position)
		} else {
			p, err = h.clickablePoint()
		}
		if err != nil {
			return nil, fmt.Errorf("getting element position: %w", err)
		}
		// Do a final actionability check to see if element can receive events
		// at mouse position in question
		if !opts.Force {
			if ok, err := h.checkHitTargetAt(apiCtx, *p); !ok {
				return nil, fmt.Errorf("checking hit target: %w", err)
			}
		}
		// Are we only "trialing" the action but not actually performing
		// it (ie. running the actionability checks).
		if opts.Trial {
			return nil, nil //nolint:nilnil
		}

		b := NewBarrier()
		h.frame.manager.addBarrier(b)
		defer h.frame.manager.removeBarrier(b)
		if res, err = fn(apiCtx, h, p); err != nil {
			return nil, fmt.Errorf("evaluating pointer action: %w", err)
		}
		// Do we need to wait for navigation to happen
		if !opts.NoWaitAfter {
			if err = b.Wait(apiCtx); err != nil {
				return nil, fmt.Errorf("waiting for navigation: %w", err)
			}
		}

		return res, nil
	}

	return func(apiCtx context.Context, resultCh chan interface{}, errCh chan error) {
		if res, err := retryPointerAction(apiCtx, pointerFn, opts); err != nil {
			errCh <- err
		} else {
			resultCh <- res
		}
	}
}

func retryPointerAction(
	apiCtx context.Context, fn retryablePointerActionFunc, opts *ElementHandleBasePointerOptions,
) (res interface{}, err error) {
	// try the default scrolling
	if res, err = fn(apiCtx, nil); opts.Force || err == nil {
		return res, err
	}
	// try with different scrolling options
	for _, p := range []ScrollPosition{
		ScrollPositionStart,
		ScrollPositionCenter,
		ScrollPositionEnd,
		ScrollPositionNearest,
	} {
		s := ScrollIntoViewOptions{Block: p, Inline: p}
		if res, err = fn(apiCtx, &s); err == nil {
			break
		}
	}

	return res, err
}

func errorFromDOMError(derr string) error {
	// return the same sentinel error value for the timed out err
	if strings.Contains(derr, "timed out") {
		return ErrTimedOut
	}
	if s := "error:expectednode:"; strings.HasPrefix(derr, s) {
		return fmt.Errorf("expected node but got %s", strings.TrimPrefix(derr, s))
	}
	errs := map[string]string{
		"error:notconnected":           "element is not attached to the DOM",
		"error:notelement":             "node is not an element",
		"error:nothtmlelement":         "not an HTMLElement",
		"error:notfillableelement":     "element is not an <input>, <textarea> or [contenteditable] element",
		"error:notfillableinputtype":   "input of this type cannot be filled",
		"error:notfillablenumberinput": "cannot type text into input[type=number]",
		"error:notvaliddate":           "malformed value",
		"error:notinput":               "node is not an HTMLInputElement",
		"error:hasnovalue":             "node is not an HTMLInputElement or HTMLTextAreaElement or HTMLSelectElement",
		"error:notselect":              "element is not a <select> element",
		"error:notcheckbox":            "not a checkbox or radio button",
		"error:notmultiplefileinput":   "non-multiple file input can only accept single file",
		"error:strictmodeviolation":    "strict mode violation, multiple elements returned for selector query",
		"error:notqueryablenode":       "node is not queryable",
		"error:nthnocapture":           "can't query n-th element in a chained selector with capture",
		"error:intercept":              "another element is intercepting with pointer action",
	}
	if err, ok := errs[derr]; ok {
		return errors.New(err)
	}

	return errors.New(derr)
}
