package common

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"go.opentelemetry.io/otel/attribute"

	"go.k6.io/k6/internal/js/modules/k6/browser/common/js"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext"
)

// Common error types for element visibility.
var (
	// ErrElementNotVisible is returned when an element is not visible for an operation.
	ErrElementNotVisible = errors.New("element is not visible")
	// ErrElementNotAttachedToDOM is returned when an element is not attached to the DOM.
	ErrElementNotAttachedToDOM = errors.New("element is not attached to the DOM")
)

const (
	resultDone       = "done"
	resultNeedsInput = "needsinput"
)

type (
	elementHandleActionFunc        func(context.Context, *ElementHandle) (any, error)
	elementHandlePointerActionFunc func(context.Context, *ElementHandle, *Position) (any, error)
	retryablePointerActionFunc     func(context.Context, *ScrollIntoViewOptions) (any, error)

	// evalFunc is a common interface for both evalWithScript and eval.
	// It helps abstracting these methods to aid with testing.
	evalFunc func(ctx context.Context, opts evalOptions, js string, args ...any) (any, error)
)

// ElementHandle represents a HTML element JS object inside an execution context.
type ElementHandle struct {
	BaseJSHandle

	frame *Frame
}

// String returns a string repesentation of ElementHandle.
// It exists mostly for debugging where we don't want fmt.Sprintf to just
// go through a complex object and try to stringify it.
func (h *ElementHandle) String() string {
	return "ElementHandle{" +
		" BaseJSHandle:  " + h.BaseJSHandle.String() +
		" frame: " + h.frame.String() +
		"}"
}

func (h *ElementHandle) boundingBox() (*Rect, error) {
	var box *dom.BoxModel
	var err error
	action := dom.GetBoxModel().WithObjectID(h.remoteObject.ObjectID)
	if box, err = action.Do(cdp.WithExecutor(h.ctx, h.session)); err != nil {
		return nil, fmt.Errorf("getting bounding box model of DOM node: %w", err)
	}

	if box == nil || box.Border == nil {
		return nil, ErrElementNotAttachedToDOM
	}

	quad := box.Border
	x := math.Min(quad[0], math.Min(quad[2], math.Min(quad[4], quad[6])))
	y := math.Min(quad[1], math.Min(quad[3], math.Min(quad[5], quad[7])))
	width := math.Max(quad[0], math.Max(quad[2], math.Max(quad[4], quad[6]))) - x
	height := math.Max(quad[1], math.Max(quad[3], math.Max(quad[5], quad[7]))) - y

	position, err := h.frame.position()
	if err != nil {
		return nil, fmt.Errorf("getting position of parent frame: %w", err)
	}

	return &Rect{X: x + position.X, Y: y + position.Y, Width: width, Height: height}, nil
}

// translatePointToPage translates the point to the page's coordinates if the
// point is relative to the parent frame.
func (h *ElementHandle) translatePointToPage(apiCtx context.Context, point Position) (Position, error) {
	h.logger.Debugf("ElementHandle:translatePointToPage", "point before translation: %v", point)

	frame, err := h.ownerFrame(apiCtx)
	if err != nil {
		return Position{}, fmt.Errorf("checking hit target at %v: %w", point, err)
	}

	if frame == nil || frame.parentFrame == nil {
		h.logger.Debugf("ElementHandle:translatePointToPage", "no parent frame")
		return point, nil
	}

	el, err := frame.FrameElement()
	if err != nil {
		return Position{}, err
	}

	box, err := el.BoundingBox()
	if err != nil {
		return Position{}, fmt.Errorf("getting bounding box of parent frame: %w", err)
	}

	if box.contains(point) {
		h.logger.Debugf("ElementHandle:translatePointToPage", "point is already in the page")
		return point, nil
	}

	// Translate from frame coordinates to page coordinates.
	point.X += box.X
	point.Y += box.Y

	h.logger.Debugf("ElementHandle:translatePointToPage", "point after translation: %v", point)

	return point, nil
}

// checkHitTargetAt checks if the element is hit by the pointer at the given point.
//
// It will recurse through frames and iframes to check if the point hits a target
// when an error:intercept occurs.
//
// It will eventually return error:intercept if the point doesn't hit any target. At
// this point the caller should retry the action after scrolling.
func (h *ElementHandle) checkHitTargetAt(apiCtx context.Context, point Position) (bool, error) {
	h.logger.Debugf("ElementHandle:checkHitTargetAt", "checking hit target at %v", point)

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
	const done = resultDone
	if v, ok := result.(string); !ok {
		frame, err := h.ownerFrame(apiCtx)
		if err != nil {
			return false, fmt.Errorf("checking hit target at %v: %w", point, err)
		}

		if frame == nil {
			// We got a { hitTargetDescription: ... } result
			// Meaning: Another element is preventing pointer events.
			//
			// It's safe to count an object return as an interception.
			// We just don't interpret what is intercepting with the target element
			// because we don't need any more functionality from this JS function
			// right now.
			return false, errorFromDOMError("error:intercept")
		}

		el, err := frame.FrameElement()
		if err != nil {
			return false, err
		}

		return el.checkHitTargetAt(apiCtx, point)
	} else if v != done {
		return false, errorFromDOMError(v)
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
	switch v := result.(type) {
	case string: // An error happened (returned as "error:..." from JS)
		return nil, errorFromDOMError(v)
	case bool:
		return &v, nil
	}

	return nil, fmt.Errorf(
		"checking state %q of element %q", state, reflect.TypeOf(result))
}

func (h *ElementHandle) click(p *Position, opts *MouseClickOptions) error {
	return h.frame.page.Mouse.click(p.X, p.Y, opts)
}

// This will get the clickable point of the element relative to the page even
// when the element is in an iframe. In some cases this isn't the case and we
// need to translate the point to the page.
func (h *ElementHandle) clickablePoint() (*Position, error) {
	r, err := h.BoundingBox()
	if err != nil {
		return nil, fmt.Errorf("finding clickable point: %w", err)
	}
	return &Position{X: r.X + r.Width/2, Y: r.Y + r.Height/2}, nil
}

func (h *ElementHandle) dblclick(p *Position, opts *MouseClickOptions) error {
	return h.frame.page.Mouse.click(p.X, p.Y, opts)
}

// DefaultTimeout returns the default timeout for this element handle.
func (h *ElementHandle) DefaultTimeout() time.Duration {
	return h.frame.manager.timeoutSettings.timeout()
}

func (h *ElementHandle) dispatchEvent(_ context.Context, typ string, eventInit any) (any, error) {
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
	s, ok := result.(string)
	if !ok {
		return fmt.Errorf("unexpected type %T", result)
	}

	if s == resultNeedsInput {
		if err := h.frame.page.Keyboard.InsertText(value); err != nil {
			return fmt.Errorf("fill: %w", err)
		}
	} else if s != resultDone {
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
	s, ok := result.(string)
	if !ok {
		return fmt.Errorf("unexpected type %T", result)
	}
	if s != resultDone {
		// Either we're done or an error happened (returned as "error:..." from JS)
		return errorFromDOMError(s)
	}

	return nil
}

func (h *ElementHandle) getAttribute(apiCtx context.Context, name string) (any, error) {
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

func (h *ElementHandle) hover(_ context.Context, p *Position) error {
	return h.frame.page.Mouse.move(p.X, p.Y, NewMouseMoveOptions())
}

func (h *ElementHandle) innerHTML(apiCtx context.Context) (any, error) {
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

func (h *ElementHandle) innerText(apiCtx context.Context) (any, error) {
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

func (h *ElementHandle) inputValue(apiCtx context.Context) (any, error) {
	//nolint:lll
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

func (h *ElementHandle) isHidden(apiCtx context.Context) (bool, error) {
	return h.waitForElementState(apiCtx, []string{"hidden"}, 0)
}

func (h *ElementHandle) isVisible(apiCtx context.Context) (bool, error) {
	return h.waitForElementState(apiCtx, []string{"visible"}, 0)
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

	var border struct{ Top, Left float64 }
	if err := convert(result, &border); err != nil {
		return nil, fmt.Errorf("converting result (%v of type %t) to border: %w", result, result, err)
	}

	box, err := h.BoundingBox()
	if err != nil {
		return nil, fmt.Errorf("finding offset position: %w", err)
	}

	if box == nil || (border.Left == 0 && border.Top == 0) {
		return nil, ErrElementNotVisible
	}

	// Make point relative to the padding box to align with offsetX/offsetY.
	return &Position{
		X: box.X + border.Left + offset.X,
		Y: box.Y + border.Top + offset.Y,
	}, nil
}

func (h *ElementHandle) ownerFrame(apiCtx context.Context) (*Frame, error) {
	frameID, err := h.frame.page.getOwnerFrame(apiCtx, h)
	if err != nil {
		return nil, err
	}
	if frameID == "" {
		return nil, nil //nolint:nilnil
	}
	frame, ok := h.frame.page.frameManager.getFrameByID(frameID)
	if ok {
		return frame, nil
	}
	for _, page := range h.frame.page.browserCtx.browser.pages {
		frame, ok = page.frameManager.getFrameByID(frameID)
		if ok {
			return frame, nil
		}
	}

	return nil, nil //nolint:nilnil
}

func (h *ElementHandle) scrollRectIntoViewIfNeeded(apiCtx context.Context, rect *dom.Rect) error {
	action := dom.ScrollIntoViewIfNeeded().WithObjectID(h.remoteObject.ObjectID).WithRect(rect)
	err := action.Do(cdp.WithExecutor(apiCtx, h.session))
	if err != nil {
		if strings.Contains(err.Error(), "Node does not have a layout object") {
			return ErrElementNotVisible
		}
		if strings.Contains(err.Error(), "Node is detached from document") {
			return ErrElementNotAttachedToDOM
		}
		return err
	}
	return nil
}

func (h *ElementHandle) press(apiCtx context.Context, key string, opts KeyboardOptions) error {
	err := h.focus(apiCtx, true)
	if err != nil {
		return err
	}
	err = h.frame.page.Keyboard.comboPress(key, opts)
	if err != nil {
		return err
	}
	return nil
}

func (h *ElementHandle) selectOption(apiCtx context.Context, values []any) (any, error) {
	fn := `
		(node, injected, values) => {
			return injected.selectOptions(node, values);
		}
	`
	opts := evalOptions{
		forceCallable: true,
		returnByValue: false,
	}
	result, err := h.evalWithScript(apiCtx, opts, fn, values)
	if err != nil {
		return nil, err
	}
	if result, ok := result.(string); ok {
		// An error happened (returned as "error:..." from JS)
		if result != resultDone {
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
	if result, ok := result.(string); ok {
		if result != resultDone {
			return errorFromDOMError(result)
		}
	}
	return nil
}

func (h *ElementHandle) textContent(apiCtx context.Context) (any, error) {
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

func (h *ElementHandle) typ(apiCtx context.Context, text string, opts KeyboardOptions) error {
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

func (h *ElementHandle) waitAndScrollIntoViewIfNeeded(
	_ context.Context, force, noWaitAfter bool, timeout time.Duration,
) error {
	fn := func(apiCtx context.Context, _ *ElementHandle) (any, error) {
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
	_, err := call(h.ctx, actFn, timeout)

	return err
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
		return false, errorFromDOMError(err)
	}
	switch v := result.(type) {
	case string: // Either we're done or an error happened (returned as "error:..." from JS)
		if v == resultDone {
			return true, nil
		}
		return false, errorFromDOMError(v)
	case bool:
		return v, nil
	}

	return false, fmt.Errorf(
		"waiting for states %v of element %q", states, reflect.TypeOf(result))
}

// stepIntoFrame steps into an iframe/frame. Due to CORS, we need to perform this
// step outside of the browser (chromium). It returns the frame that it has stepped
// into and the selector to use within that frame.
func (h *ElementHandle) stepIntoFrame(
	apiCtx context.Context, parsedSelector *Selector, frameNavIndex int, opts *FrameWaitForSelectorOptions,
) (*Frame, string, error) {
	// Split selector at frame navigation boundary
	beforeFrame, afterFrame := h.splitSelectorAtFrame(parsedSelector, frameNavIndex)

	// Find the iframe element using the "before frame" selector
	iframeSelector := h.reconstructSelector(beforeFrame)

	iframeHandle, err := h.waitForSelector(apiCtx, iframeSelector, opts)
	if err != nil {
		return nil, "", fmt.Errorf("finding iframe with selector %q: %w", iframeSelector, err)
	}

	// This is a valid response from waitForSelector. It means that the element
	// was either hidden or detached.
	if iframeHandle == nil {
		return nil, "", ErrElementNotVisible
	}

	frame, err := iframeHandle.ContentFrame()
	if err != nil {
		return nil, "", fmt.Errorf("getting iframe frame: %w", err)
	}

	// Wait for selector in the iframe using the "after frame" selector
	afterFrameSelector := h.reconstructSelector(afterFrame)

	return frame, afterFrameSelector, nil
}

func (h *ElementHandle) waitForSelector(
	apiCtx context.Context, selector string, opts *FrameWaitForSelectorOptions,
) (*ElementHandle, error) {
	parsedSelector, err := NewSelector(selector)
	if err != nil {
		return nil, err
	}

	// Check for frame navigation in the selector
	frameNavIndex := h.findFrameNavigationIndex(parsedSelector)
	if frameNavIndex != -1 {
		// Strict is true because we assume the user is interested in the element
		// in the frame and not the frame it is in.
		opts := &FrameWaitForSelectorOptions{
			State:   DOMElementStateAttached,
			Timeout: opts.Timeout,
			Strict:  true,
		}

		frame, afterFrameSelector, err := h.stepIntoFrame(apiCtx, parsedSelector, frameNavIndex, opts)
		if err != nil {
			return nil, err
		}

		return frame.waitForSelector(afterFrameSelector, opts)
	}

	// No frame navigation - proceed with normal waitForSelector logic
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
		// This is a valid response, which means that the element was either
		// hidden or detached.
		return nil, nil //nolint:nilnil
	}
}

func (h *ElementHandle) count(apiCtx context.Context, selector string) (int, error) {
	parsedSelector, err := NewSelector(selector)
	if err != nil {
		return 0, err
	}

	// Check for frame navigation in the selector
	frameNavIndex := h.findFrameNavigationIndex(parsedSelector)
	if frameNavIndex != -1 {
		// Strict is true because we assume the user is interested in the element
		// in the frame and not the frame it is in.
		opts := &FrameWaitForSelectorOptions{
			State:   DOMElementStateAttached,
			Timeout: h.frame.defaultTimeout(),
			Strict:  true,
		}

		frame, afterFrameSelector, err := h.stepIntoFrame(apiCtx, parsedSelector, frameNavIndex, opts)
		if err != nil {
			return 0, err
		}

		return frame.count(afterFrameSelector)
	}

	// No frame navigation - proceed with normal count logic
	fn := `
			(node, injected, selector) => {
				return injected.count(selector, node);
			}
		`
	eopts := evalOptions{
		forceCallable: true,
		returnByValue: true,
	}
	result, err := h.evalWithScript(
		apiCtx,
		eopts, fn, parsedSelector,
	)
	if err != nil {
		return 0, err
	}
	switch r := result.(type) {
	case float64:
		if r < float64(math.MinInt32) || r > float64(math.MaxInt32) {
			return 0, fmt.Errorf("value %v out of range for int32 type", r)
		}
		return int(r), nil
	default:
		return 0, fmt.Errorf("unexpected value %v of type %T", r, r)
	}
}

// findFrameNavigationIndex finds the index of the first internal:control=enter-frame directive
func (h *ElementHandle) findFrameNavigationIndex(selector *Selector) int {
	for i, part := range selector.Parts {
		if part.Name == "internal:control" && part.Body == "enter-frame" {
			return i
		}
	}
	return -1
}

// splitSelectorAtFrame splits a selector at the frame navigation boundary
func (h *ElementHandle) splitSelectorAtFrame(selector *Selector, frameIndex int) (*Selector, *Selector) {
	beforeFrame := &Selector{
		Selector: selector.Selector, // Keep original for reference
		Parts:    selector.Parts[:frameIndex],
		Capture:  selector.Capture,
	}

	afterFrame := &Selector{
		Selector: selector.Selector, // Keep original for reference
		Parts:    selector.Parts[frameIndex+1:],
		Capture:  selector.Capture,
	}

	return beforeFrame, afterFrame
}

// reconstructSelector rebuilds a selector string from selector parts
func (h *ElementHandle) reconstructSelector(selector *Selector) string {
	if len(selector.Parts) == 0 {
		return ""
	}

	parts := make([]string, len(selector.Parts))
	for i, part := range selector.Parts {
		if part.Name == "css" {
			parts[i] = part.Body
		} else {
			parts[i] = part.Name + "=" + part.Body
		}
	}

	return strings.Join(parts, " >> ")
}

// AsElement returns this element handle.
func (h *ElementHandle) AsElement() *ElementHandle {
	return h
}

// BoundingBox returns this element's bounding box.
func (h *ElementHandle) BoundingBox() (*Rect, error) {
	bbox, err := h.boundingBox()
	if err != nil && strings.Contains(err.Error(), "Could not compute box model") {
		return nil, fmt.Errorf("%w: %w", ErrElementNotVisible, err)
	}
	if err != nil {
		return nil, fmt.Errorf("getting bounding box: %w", err)
	}
	return bbox, nil
}

// Click scrolls element into view and clicks in the center of the element
// TODO: look into making more robust using retries
// (see: https://github.com/microsoft/playwright/blob/master/src/server/dom.ts#L298)
func (h *ElementHandle) Click(opts *ElementHandleClickOptions) error {
	click := h.newPointerAction(
		func(apiCtx context.Context, handle *ElementHandle, p *Position) (any, error) {
			return nil, handle.click(p, opts.ToMouseClickOptions())
		},
		&opts.ElementHandleBasePointerOptions,
	)
	if _, err := call(h.ctx, click, opts.Timeout); err != nil {
		return fmt.Errorf("clicking on element: %w", err)
	}
	applySlowMo(h.ctx)

	return nil
}

// ContentFrame returns the frame that contains this element.
func (h *ElementHandle) ContentFrame() (*Frame, error) {
	var (
		node *cdp.Node
		err  error
	)
	action := dom.DescribeNode().WithObjectID(h.remoteObject.ObjectID)
	if node, err = action.Do(cdp.WithExecutor(h.ctx, h.session)); err != nil {
		return nil, fmt.Errorf("getting remote node %q: %w", h.remoteObject.ObjectID, err)
	}
	if node == nil || node.FrameID == "" {
		return nil, fmt.Errorf("element is not an iframe")
	}

	frame, ok := h.frame.manager.getFrameByID(node.FrameID)
	if !ok {
		return nil, fmt.Errorf("frame not found for id %s", node.FrameID)
	}

	return frame, nil
}

// Dblclick scrolls element into view and double clicks on the element.
func (h *ElementHandle) Dblclick(opts *ElementHandleDblclickOptions) error {
	dblclick := func(_ context.Context, handle *ElementHandle, p *Position) (any, error) {
		return nil, handle.dblclick(p, opts.ToMouseClickOptions())
	}
	dblclickAction := h.newPointerAction(dblclick, &opts.ElementHandleBasePointerOptions)
	if _, err := call(h.ctx, dblclickAction, opts.Timeout); err != nil {
		return fmt.Errorf("double clicking on element: %w", err)
	}

	applySlowMo(h.ctx)

	return nil
}

// DispatchEvent dispatches a DOM event to the element.
func (h *ElementHandle) DispatchEvent(typ string, eventInit any) error {
	dispatchEvent := func(apiCtx context.Context, handle *ElementHandle) (any, error) {
		return handle.dispatchEvent(apiCtx, typ, eventInit)
	}
	opts := NewElementHandleBaseOptions(h.DefaultTimeout())
	dispatchEventAction := h.newAction(
		[]string{}, dispatchEvent, opts.Force, opts.NoWaitAfter, opts.Timeout,
	)
	if _, err := call(h.ctx, dispatchEventAction, opts.Timeout); err != nil {
		return fmt.Errorf("dispatching element event %q: %w", typ, err)
	}

	applySlowMo(h.ctx)

	return nil
}

// Fill types the given value into the element.
func (h *ElementHandle) Fill(value string, opts *ElementHandleBaseOptions) error {
	fill := func(apiCtx context.Context, handle *ElementHandle) (any, error) {
		return nil, handle.fill(apiCtx, value)
	}
	fillAction := h.newAction(
		[]string{"visible", "enabled", "editable"},
		fill, opts.Force, opts.NoWaitAfter, opts.Timeout,
	)
	if _, err := call(h.ctx, fillAction, opts.Timeout); err != nil {
		return fmt.Errorf("filling element: %w", err)
	}

	applySlowMo(h.ctx)

	return nil
}

// Focus scrolls element into view and focuses the element.
func (h *ElementHandle) Focus() error {
	focus := func(apiCtx context.Context, handle *ElementHandle) (any, error) {
		return nil, handle.focus(apiCtx, false)
	}
	opts := NewElementHandleBaseOptions(h.DefaultTimeout())
	focusAction := h.newAction(
		[]string{}, focus, opts.Force, opts.NoWaitAfter, opts.Timeout,
	)
	if _, err := call(h.ctx, focusAction, opts.Timeout); err != nil {
		return fmt.Errorf("focusing on element: %w", err)
	}

	applySlowMo(h.ctx)

	return nil
}

// GetAttribute retrieves the value of specified element attribute.
// The second return value is true if the attribute exists, and false otherwise.
func (h *ElementHandle) GetAttribute(name string) (string, bool, error) {
	getAttribute := func(apiCtx context.Context, handle *ElementHandle) (any, error) {
		return handle.getAttribute(apiCtx, name)
	}
	opts := NewElementHandleBaseOptions(h.DefaultTimeout())
	getAttributeAction := h.newAction(
		[]string{}, getAttribute, opts.Force, opts.NoWaitAfter, opts.Timeout,
	)

	v, err := call(h.ctx, getAttributeAction, opts.Timeout)
	if err != nil {
		return "", false, fmt.Errorf("getting attribute %q of element: %w", name, err)
	}
	if v == nil {
		return "", false, nil
	}
	s, ok := v.(string)
	if !ok {
		return "", false, fmt.Errorf(
			"getting attribute %q of element: unexpected type %T (expecting string)",
			name, v,
		)
	}

	return s, true, nil
}

// Hover scrolls element into view and hovers over its center point.
func (h *ElementHandle) Hover(opts *ElementHandleHoverOptions) error {
	hover := func(apiCtx context.Context, handle *ElementHandle, p *Position) (any, error) {
		return nil, handle.hover(apiCtx, p)
	}
	hoverAction := h.newPointerAction(hover, &opts.ElementHandleBasePointerOptions)
	if _, err := call(h.ctx, hoverAction, opts.Timeout); err != nil {
		return fmt.Errorf("hovering on element: %w", err)
	}

	applySlowMo(h.ctx)

	return nil
}

// InnerHTML returns the inner HTML of the element.
func (h *ElementHandle) InnerHTML() (string, error) {
	innerHTML := func(apiCtx context.Context, handle *ElementHandle) (any, error) {
		return handle.innerHTML(apiCtx)
	}
	opts := NewElementHandleBaseOptions(h.DefaultTimeout())
	innerHTMLAction := h.newAction(
		[]string{}, innerHTML, opts.Force, opts.NoWaitAfter, opts.Timeout,
	)
	v, err := call(h.ctx, innerHTMLAction, opts.Timeout)
	if err != nil {
		return "", fmt.Errorf("getting element's inner HTML: %w", err)
	}

	applySlowMo(h.ctx)

	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("unexpected type %T (expecting string)", v)
	}

	return s, nil
}

// InnerText returns the inner text of the element.
func (h *ElementHandle) InnerText() (string, error) {
	innerText := func(apiCtx context.Context, handle *ElementHandle) (any, error) {
		return handle.innerText(apiCtx)
	}
	opts := NewElementHandleBaseOptions(h.DefaultTimeout())
	innerTextAction := h.newAction(
		[]string{}, innerText, opts.Force, opts.NoWaitAfter, opts.Timeout.Abs(),
	)
	v, err := call(h.ctx, innerTextAction, opts.Timeout)
	if err != nil {
		return "", fmt.Errorf("getting element's inner text: %w", err)
	}

	applySlowMo(h.ctx)

	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("unexpected type %T (expecting string)", v)
	}

	return s, nil
}

// InputValue returns the value of the input element.
func (h *ElementHandle) InputValue(opts *ElementHandleBaseOptions) (string, error) {
	inputValue := func(apiCtx context.Context, handle *ElementHandle) (any, error) {
		return handle.inputValue(apiCtx)
	}
	inputValueAction := h.newAction([]string{}, inputValue, opts.Force, opts.NoWaitAfter, opts.Timeout)
	v, err := call(h.ctx, inputValueAction, opts.Timeout)
	if err != nil {
		return "", fmt.Errorf("getting element's input value: %w", err)
	}

	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("unexpected type %T (expecting string)", v)
	}

	return s, nil
}

// IsChecked checks if a checkbox or radio is checked.
func (h *ElementHandle) IsChecked() (bool, error) {
	ok, err := h.isChecked(h.ctx, 0)
	// We don't care about timeout errors here!
	if err != nil && !errors.Is(err, ErrTimedOut) {
		return false, fmt.Errorf("checking element is checked: %w", err)
	}

	return ok, nil
}

// IsDisabled checks if the element is disabled.
func (h *ElementHandle) IsDisabled() (bool, error) {
	ok, err := h.isDisabled(h.ctx, 0)
	// We don't care anout timeout errors here!
	if err != nil && !errors.Is(err, ErrTimedOut) {
		return false, fmt.Errorf("checking element is disabled: %w", err)
	}

	return ok, nil
}

// IsEditable checks if the element is editable.
func (h *ElementHandle) IsEditable() (bool, error) {
	ok, err := h.isEditable(h.ctx, 0)
	// We don't care anout timeout errors here!
	if err != nil && !errors.Is(err, ErrTimedOut) {
		return false, fmt.Errorf("checking element is editable: %w", err)
	}

	return ok, nil
}

// IsEnabled checks if the element is enabled.
func (h *ElementHandle) IsEnabled() (bool, error) {
	ok, err := h.isEnabled(h.ctx, 0)
	// We don't care anout timeout errors here!
	if err != nil && !errors.Is(err, ErrTimedOut) {
		return false, fmt.Errorf("checking element is enabled: %w", err)
	}

	return ok, nil
}

// IsHidden checks if the element is hidden.
func (h *ElementHandle) IsHidden() (bool, error) {
	ok, err := h.isHidden(h.ctx)
	// We don't care anout timeout errors here!
	if err != nil && !errors.Is(err, ErrTimedOut) {
		return false, fmt.Errorf("checking element is hidden: %w", err)
	}

	return ok, nil
}

// IsVisible checks if the element is visible.
func (h *ElementHandle) IsVisible() (bool, error) {
	ok, err := h.isVisible(h.ctx)
	// We don't care anout timeout errors here!
	if err != nil && !errors.Is(err, ErrTimedOut) {
		return false, fmt.Errorf("checking element is visible: %w", err)
	}

	return ok, nil
}

// OwnerFrame returns the frame containing this element.
func (h *ElementHandle) OwnerFrame() (_ *Frame, rerr error) {
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
		return nil, fmt.Errorf("getting document element: %w", err)
	}
	if res == nil {
		return nil, errors.New("getting document element: nil document")
	}

	documentHandle, ok := res.(*ElementHandle)
	if !ok {
		return nil, fmt.Errorf("unexpected result type while getting document element: %T", res)
	}
	defer func() {
		if err := documentHandle.Dispose(); err != nil {
			err = fmt.Errorf("disposing document element: %w", err)
			rerr = errors.Join(err, rerr)
		}
	}()

	if documentHandle.remoteObject.ObjectID == "" {
		return nil, err
	}

	var node *cdp.Node
	action := dom.DescribeNode().WithObjectID(documentHandle.remoteObject.ObjectID)
	if node, err = action.Do(cdp.WithExecutor(h.ctx, h.session)); err != nil {
		return nil, fmt.Errorf("getting node in frame: %w", err)
	}
	if node == nil || node.FrameID == "" {
		return nil, fmt.Errorf("no frame found for node: %w", err)
	}

	frame, ok := h.frame.manager.getFrameByID(node.FrameID)
	if !ok {
		return nil, fmt.Errorf("no frame found for id %s", node.FrameID)
	}

	return frame, nil
}

// Press scrolls element into view and presses the given keys.
func (h *ElementHandle) Press(key string, opts *ElementHandlePressOptions) error {
	press := func(apiCtx context.Context, handle *ElementHandle) (any, error) {
		return nil, handle.press(apiCtx, key, KeyboardOptions{})
	}
	pressAction := h.newAction(
		[]string{}, press, false, opts.NoWaitAfter, opts.Timeout,
	)
	if _, err := call(h.ctx, pressAction, opts.Timeout); err != nil {
		return fmt.Errorf("pressing %q on element: %w", key, err)
	}

	applySlowMo(h.ctx)

	return nil
}

// Query runs "element.querySelector" within the page. If no element matches the selector,
// the return value resolves to "null".
func (h *ElementHandle) Query(selector string, strict bool) (_ *ElementHandle, rerr error) {
	parsedSelector, err := NewSelector(selector)
	if err != nil {
		return nil, fmt.Errorf("parsing selector %q: %w", selector, err)
	}

	// Check for frame navigation in the selector
	frameNavIndex := h.findFrameNavigationIndex(parsedSelector)
	if frameNavIndex != -1 {
		// Strict is true because we assume the user is interested in the element
		// in the frame and not the frame it is in.
		opts := &FrameWaitForSelectorOptions{
			State:   DOMElementStateAttached,
			Timeout: h.frame.defaultTimeout(),
			Strict:  true,
		}

		frame, afterFrameSelector, err := h.stepIntoFrame(h.ctx, parsedSelector, frameNavIndex, opts)
		if err != nil {
			return nil, err
		}

		return frame.Query(afterFrameSelector, strict)
	}

	// No frame navigation - proceed with normal Query logic
	querySelector := `
		(node, injected, selector, strict) => {
			return injected.querySelector(selector, strict, node || document);
		}
	`
	opts := evalOptions{
		forceCallable: true,
		returnByValue: false,
	}
	result, err := h.evalWithScript(h.ctx, opts, querySelector, parsedSelector, strict)
	if err != nil {
		return nil, fmt.Errorf("querying selector %q: %w", selector, err)
	}
	if result == nil {
		return nil, nil //nolint:nilnil
	}
	handle, ok := result.(JSHandleAPI)
	if !ok {
		return nil, fmt.Errorf("querying selector %q, wrong type %T", selector, result)
	}
	element := handle.AsElement()
	if element == nil {
		defer func() {
			if err := handle.Dispose(); err != nil {
				err = fmt.Errorf("disposing element handle: %w", err)
				rerr = errors.Join(err, rerr)
			}
		}()
		return nil, fmt.Errorf("querying selector %q", selector)
	}

	return element, nil
}

// QueryAll queries element subtree for matching elements.
// If no element matches the selector, the return value resolves to "null".
func (h *ElementHandle) QueryAll(selector string) ([]*ElementHandle, error) {
	handles, err := h.queryAll(selector, h.evalWithScript)
	if err != nil {
		return nil, fmt.Errorf("querying all selector %q: %w", selector, err)
	}

	return handles, nil
}

func (h *ElementHandle) queryAll(selector string, eval evalFunc) (_ []*ElementHandle, rerr error) {
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
		return nil, fmt.Errorf("querying all selectors %q: %w", selector, err)
	}
	if result == nil {
		// it is ok to return a nil slice because it means we didn't find any elements.
		return nil, nil
	}

	handles, ok := result.(JSHandleAPI)
	if !ok {
		return nil, fmt.Errorf("getting element handle for selector %q: %w", selector, ErrJSHandleInvalid)
	}
	defer func() {
		if err := handles.Dispose(); err != nil {
			err = fmt.Errorf("disposing element handles: %w", err)
			rerr = errors.Join(err, rerr)
		}
	}()

	props, err := handles.GetProperties()
	if err != nil {
		// GetProperties has a rich error already, so we don't need to wrap it.
		return nil, err //nolint:wrapcheck
	}

	type indexedElem struct {
		index int
		elem  *ElementHandle
	}

	indexedElems := make([]indexedElem, 0, len(props))
	for key, prop := range props {
		if el := prop.AsElement(); el != nil {
			// DOM elements are stored with numeric indices ("0", "1", "2", ...)
			// per JavaScript's array-like object specification.
			idx, err := strconv.Atoi(key)
			if err != nil {
				return nil, fmt.Errorf("parsing element index %q for selector %q: %w", key, selector, err)
			}
			indexedElems = append(indexedElems, indexedElem{index: idx, elem: el})
		} else if err := prop.Dispose(); err != nil {
			return nil, fmt.Errorf("disposing property while querying all selectors %q: %w", selector, err)
		}
	}

	slices.SortFunc(indexedElems, func(a, b indexedElem) int {
		return cmp.Compare(a.index, b.index)
	})

	els := make([]*ElementHandle, 0, len(indexedElems))
	for _, ie := range indexedElems {
		els = append(els, ie.elem)
	}

	return els, nil
}

// SetChecked checks or unchecks an element.
func (h *ElementHandle) SetChecked(checked bool, opts *ElementHandleSetCheckedOptions) error {
	setChecked := func(apiCtx context.Context, handle *ElementHandle, p *Position) (any, error) {
		return nil, handle.setChecked(apiCtx, checked, p)
	}
	setCheckedAction := h.newPointerAction(setChecked, &opts.ElementHandleBasePointerOptions)
	if _, err := call(h.ctx, setCheckedAction, opts.Timeout); err != nil {
		return fmt.Errorf("checking element: %w", err)
	}

	applySlowMo(h.ctx)

	return nil
}

// Uncheck scrolls element into view, and if it's an input element of type
// checkbox that is already checked, clicks on it to mark it as unchecked.
func (h *ElementHandle) Uncheck(opts *ElementHandleSetCheckedOptions) error {
	return h.SetChecked(false, opts)
}

// Check scrolls element into view, and if it's an input element of type
// checkbox that is unchecked, clicks on it to mark it as checked.
func (h *ElementHandle) Check(opts *ElementHandleSetCheckedOptions) error {
	return h.SetChecked(true, opts)
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

// Screenshot will instruct Chrome to save a screenshot of the current element and save it to specified file.
func (h *ElementHandle) Screenshot(
	opts *ElementHandleScreenshotOptions,
	sp ScreenshotPersister,
) ([]byte, error) {
	spanCtx, span := TraceAPICall(
		h.ctx,
		h.frame.page.targetID.String(),
		"elementHandle.screenshot",
	)
	defer span.End()

	span.SetAttributes(attribute.String("screenshot.path", opts.Path))

	s := newScreenshotter(spanCtx, sp, h.logger)
	buf, err := s.screenshotElement(h, opts)
	if err != nil {
		return nil, spanRecordErrorf(span, "taking screenshot of elementHandle: %w", err)
	}

	return buf, err
}

// ScrollIntoViewIfNeeded scrolls element into view if needed.
func (h *ElementHandle) ScrollIntoViewIfNeeded(opts *ElementHandleBaseOptions) error {
	err := h.waitAndScrollIntoViewIfNeeded(h.ctx, opts.Force, opts.NoWaitAfter, opts.Timeout)
	if err != nil {
		return fmt.Errorf("scrolling element into view: %w", err)
	}

	applySlowMo(h.ctx)

	return nil
}

// SelectOption selects the options matching the given values.
func (h *ElementHandle) SelectOption(values []any, opts *ElementHandleBaseOptions) ([]string, error) {
	selectOption := func(apiCtx context.Context, handle *ElementHandle) (any, error) {
		return handle.selectOption(apiCtx, values)
	}
	selectOptionAction := h.newAction(
		[]string{}, selectOption, opts.Force, opts.NoWaitAfter, opts.Timeout,
	)
	selectedOptions, err := call(h.ctx, selectOptionAction, opts.Timeout)
	if err != nil {
		return nil, fmt.Errorf("selecting options: %w", err)
	}
	var returnVal []string
	if err := convert(selectedOptions, &returnVal); err != nil {
		return nil, fmt.Errorf("unpacking selected options: %w", err)
	}

	applySlowMo(h.ctx)

	return returnVal, nil
}

// SelectText selects the text of the element.
func (h *ElementHandle) SelectText(opts *ElementHandleBaseOptions) error {
	selectText := func(apiCtx context.Context, handle *ElementHandle) (any, error) {
		return nil, handle.selectText(apiCtx)
	}
	selectTextAction := h.newAction(
		[]string{}, selectText, opts.Force, opts.NoWaitAfter, opts.Timeout,
	)
	if _, err := call(h.ctx, selectTextAction, opts.Timeout); err != nil {
		return fmt.Errorf("selecting text: %w", err)
	}

	applySlowMo(h.ctx)

	return nil
}

// SetInputFiles sets the given files into the input file element.
func (h *ElementHandle) SetInputFiles(files *Files, opts *ElementHandleSetInputFilesOptions) error {
	setInputFiles := func(apiCtx context.Context, handle *ElementHandle) (any, error) {
		return nil, handle.setInputFiles(apiCtx, files)
	}
	setInputFilesAction := h.newAction([]string{}, setInputFiles, opts.Force, opts.NoWaitAfter, opts.Timeout)
	if _, err := call(h.ctx, setInputFilesAction, opts.Timeout); err != nil {
		return fmt.Errorf("setting input files: %w", err)
	}

	return nil
}

func (h *ElementHandle) setInputFiles(apiCtx context.Context, files *Files) error {
	// allow clearing the input by passing an empty array
	var payload []*File
	if files != nil {
		payload = files.Payload
	}
	fn := `
		(node, injected, payload) => {
			return injected.setInputFiles(node, payload);
		}
	`
	evalOpts := evalOptions{
		forceCallable: true,
		returnByValue: true,
	}
	result, err := h.evalWithScript(apiCtx, evalOpts, fn, payload)
	if err != nil {
		return err
	}
	v, ok := result.(string)
	if !ok {
		return fmt.Errorf("unexpected type %T", result)
	}
	if v != "done" {
		return errorFromDOMError(v)
	}

	return nil
}

// Tap scrolls element into view and taps in the center of the element.
func (h *ElementHandle) Tap(opts *ElementHandleTapOptions) error {
	tap := func(apiCtx context.Context, handle *ElementHandle, p *Position) (any, error) {
		return nil, handle.tap(apiCtx, p)
	}
	tapAction := h.newPointerAction(tap, &opts.ElementHandleBasePointerOptions)

	if _, err := call(h.ctx, tapAction, opts.Timeout); err != nil {
		return fmt.Errorf("tapping element: %w", err)
	}

	applySlowMo(h.ctx)

	return nil
}

func (h *ElementHandle) tap(_ context.Context, p *Position) error {
	return h.frame.page.Touchscreen.tap(p.X, p.Y)
}

// TextContent returns the text content of the element.
// The second return value is true if the text content exists, and false otherwise.
func (h *ElementHandle) TextContent() (string, bool, error) {
	textContent := func(apiCtx context.Context, handle *ElementHandle) (any, error) {
		return handle.textContent(apiCtx)
	}
	opts := NewElementHandleBaseOptions(h.DefaultTimeout())
	textContentAction := h.newAction(
		[]string{}, textContent, opts.Force, opts.NoWaitAfter, opts.Timeout,
	)
	v, err := call(h.ctx, textContentAction, opts.Timeout)
	if err != nil {
		return "", false, fmt.Errorf("getting text content of element: %w", err)
	}
	if v == nil {
		return "", false, nil
	}
	s, ok := v.(string)
	if !ok {
		return "", false, fmt.Errorf(
			"getting text content of element: unexpected type %T (expecting string)",
			v,
		)
	}

	return s, true, nil
}

// Timeout will return the default timeout or the one set by the user.
// It's an internal method not to be exposed as a JS API.
func (h *ElementHandle) Timeout() time.Duration {
	return h.DefaultTimeout()
}

// Type scrolls element into view, focuses element and types text.
func (h *ElementHandle) Type(text string, opts *ElementHandleTypeOptions) error {
	typ := func(apiCtx context.Context, handle *ElementHandle) (any, error) {
		return nil, handle.typ(apiCtx, text, KeyboardOptions{})
	}
	typeAction := h.newAction(
		[]string{}, typ, false, opts.NoWaitAfter, opts.Timeout,
	)
	if _, err := call(h.ctx, typeAction, opts.Timeout); err != nil {
		return fmt.Errorf("typing text %q: %w", text, err)
	}

	applySlowMo(h.ctx)

	return nil
}

// WaitForElementState waits for the element to reach the given state.
func (h *ElementHandle) WaitForElementState(state string, opts *ElementHandleWaitForElementStateOptions) error {
	_, err := h.waitForElementState(h.ctx, []string{state}, opts.Timeout)
	if err != nil {
		return fmt.Errorf("waiting for element state %q: %w", state, err)
	}

	return nil
}

// WaitForSelector waits for the selector to appear in the DOM.
func (h *ElementHandle) WaitForSelector(selector string, opts *FrameWaitForSelectorOptions) (*ElementHandle, error) {
	handle, err := h.waitForSelector(h.ctx, selector, opts)
	if err != nil {
		return nil, fmt.Errorf("waiting for selector %q: %w", selector, err)
	}

	return handle, nil
}

// evalWithScript evaluates the given js code in the scope of this ElementHandle and returns the result.
// The js code can call helper functions from injected_script.js.
func (h *ElementHandle) evalWithScript(
	ctx context.Context,
	opts evalOptions, js string, args ...any,
) (any, error) {
	script, err := h.execCtx.getInjectedScript(h.ctx)
	if err != nil {
		return nil, fmt.Errorf("getting injected script: %w", err)
	}
	return h.eval(ctx, opts, js, append([]any{script}, args...)...)
}

// eval evaluates the given js code in the scope of this ElementHandle and returns the result.
func (h *ElementHandle) eval(
	ctx context.Context,
	opts evalOptions, js string, args ...any,
) (any, error) {
	// passing `h` makes it evaluate js code in the element handle's scope.
	return h.execCtx.eval(ctx, opts, js, append([]any{h}, args...)...)
}

func (h *ElementHandle) newAction(
	states []string, fn elementHandleActionFunc, force, noWaitAfter bool, timeout time.Duration,
) func(apiCtx context.Context, resultCh chan any, errCh chan error) {
	// All or a subset of the following actionability checks are made before performing the actual action:
	// 1. Attached to DOM
	// 2. Visible
	// 3. Stable
	// 4. Enabled
	actionFn := func(apiCtx context.Context) (any, error) {
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

	return func(apiCtx context.Context, resultCh chan any, errCh chan error) {
		if res, err := actionFn(apiCtx); err != nil {
			select {
			case <-apiCtx.Done():
			case errCh <- err:
			}
		} else {
			select {
			case <-apiCtx.Done():
			case resultCh <- res:
			}
		}
	}
}

//nolint:gocognit,funlen
func (h *ElementHandle) newPointerAction(
	fn elementHandlePointerActionFunc, opts *ElementHandleBasePointerOptions,
) func(apiCtx context.Context, resultCh chan any, errCh chan error) {
	// All or a subset of the following actionability checks are made before performing the actual action:
	// 1. Attached to DOM
	// 2. Visible
	// 3. Stable
	// 4. Enabled
	// 5. Receives events
	pointerFn := func(apiCtx context.Context, sopts *ScrollIntoViewOptions) (res any, err error) {
		// We need to scroll the element into view first, otherwise we can
		// end up in a situation where the element is not in the correct state
		// (visible and stable, but could be enabled).
		err = h.scrollRectIntoViewIfNeeded(apiCtx, nil)
		if err != nil {
			return nil, fmt.Errorf("scrolling element into view: %w", err)
		}

		// Check if we should run actionability checks
		if !opts.Force {
			// As mentioned above, if we didn't scroll first, we could end
			// up stuck waiting indefinitely for the element to be in the
			// correct state.
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
			if err != nil {
				return nil, fmt.Errorf("getting element position: %w", err)
			}
		} else {
			p, err = h.clickablePoint()
			if err != nil {
				return nil, fmt.Errorf("getting clickable point: %w", err)
			}
		}

		// Further translation of the point might be necessary if the point
		// is still relative to the parent frame it is in and not the page.
		*p, err = h.translatePointToPage(apiCtx, *p)
		if err != nil {
			return nil, fmt.Errorf("translating point to page: %w", err)
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

	return func(apiCtx context.Context, resultCh chan any, errCh chan error) {
		if res, err := retryPointerAction(apiCtx, pointerFn, opts); err != nil {
			select {
			case <-apiCtx.Done():
			case errCh <- err:
			}
		} else {
			select {
			case <-apiCtx.Done():
			case resultCh <- res:
			}
		}
	}
}

func retryPointerAction(
	apiCtx context.Context, fn retryablePointerActionFunc, opts *ElementHandleBasePointerOptions,
) (res any, err error) {
	for {
		res, err = fn(apiCtx, nil)
		if opts.Force || err == nil {
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
				return res, nil
			}
		}

		// Only locator based APIs should retry.
		if !opts.retry {
			return res, err
		}

		if !errors.Is(err, ErrElementNotVisible) &&
			!errors.Is(err, ErrElementNotAttachedToDOM) &&
			!strings.Contains(err.Error(), "frame has been detached") {
			return res, err
		}

		// Wait with timeout or context cancellation
		select {
		case <-apiCtx.Done():
			return nil, ContextErr(apiCtx)
		case <-time.After(20 * time.Millisecond):
			// Continue retrying after delay
		}
	}
}

func errorFromDOMError(v any) error {
	var (
		err  error
		serr string
	)
	switch e := v.(type) {
	case string:
		serr = e
	case error:
		if e == nil {
			return errors.New("DOM error is nil")
		}
		err, serr = e, e.Error()
	default:
		return fmt.Errorf("unexpected DOM error type %T", v)
	}
	var uerr *k6ext.UserFriendlyError
	if errors.As(err, &uerr) {
		return err
	}
	if strings.Contains(serr, "timed out") {
		return &k6ext.UserFriendlyError{
			Err: ErrTimedOut,
		}
	}
	if s := "error:expectednode:"; strings.HasPrefix(serr, s) {
		return fmt.Errorf("expected node but got %s", strings.TrimPrefix(serr, s))
	}

	if serr == "error:notconnected" {
		return ErrElementNotAttachedToDOM
	}

	errs := map[string]string{
		"error:notelement":             "node is not an element",
		"error:nothtmlelement":         "not an HTMLElement",
		"error:notfillableelement":     "element is not an <input>, <textarea> or [contenteditable] element",
		"error:notfillableinputtype":   "input of this type cannot be filled",
		"error:notfillablenumberinput": "cannot type text into input[type=number]",
		"error:notvaliddate":           "malformed value",
		"error:notinput":               "node is not an HTMLInputElement",
		"error:notfile":                "node is not an input[type=file] element",
		"error:hasnovalue":             "node is not an HTMLInputElement or HTMLTextAreaElement or HTMLSelectElement",
		"error:notselect":              "element is not a <select> element",
		"error:notcheckbox":            "not a checkbox or radio button",
		"error:notmultiplefileinput":   "non-multiple file input can only accept single file",
		"error:strictmodeviolation":    "strict mode violation, multiple elements returned for selector query",
		"error:notqueryablenode":       "node is not queryable",
		"error:nthnocapture":           "can't query n-th element in a chained selector with capture",
		"error:intercept":              "another element is intercepting with pointer action",
	}
	if err, ok := errs[serr]; ok {
		return errors.New(err)
	}

	return errors.New(serr)
}
