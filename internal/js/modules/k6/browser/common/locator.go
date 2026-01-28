package common

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"go.k6.io/k6/internal/js/modules/k6/browser/log"
)

// Strict mode:
// All operations on locators throw an exception if more
// than one element matches the locator's selector.
//
// See Issue #100 for more details.

// Locator represents a way to find element(s) on the page at any moment.
type Locator struct {
	selector string
	opts     *LocatorOptions

	frame *Frame

	ctx context.Context
	log *log.Logger
}

// LocatorOptions allows modifying the [Locator] behavior.
type LocatorOptions struct {
	// Matches only elements that contain the specified text.
	// String or RegExp. Optional.
	HasText string
	// Matches only elements that do not contain the specified text.
	// String or RegExp. Optional.
	HasNotText string
}

// NewLocator creates and returns a new locator.
func NewLocator(ctx context.Context, opts *LocatorOptions, selector string, f *Frame, l *log.Logger) *Locator {
	if opts == nil {
		opts = new(LocatorOptions)
	}
	if opts.HasText != "" {
		selector += " >> internal:has-text=" + opts.HasText
	}
	if opts.HasNotText != "" {
		selector += " >> internal:has-not-text=" + opts.HasNotText
	}
	return &Locator{
		selector: selector,
		opts:     opts,
		frame:    f,
		ctx:      ctx,
		log:      l,
	}
}

// BoundingBox will return the bounding box of the element.
func (l *Locator) BoundingBox(opts *FrameBaseOptions) (*Rect, error) {
	opts.Strict = true
	return l.frame.boundingBox(l.selector, opts)
}

// Clear will clear the input field.
// This works with the Fill API and fills the input field with an empty string.
func (l *Locator) Clear(opts *FrameFillOptions) error {
	l.log.Debugf(
		"Locator:Clear", "fid:%s furl:%q sel:%q opts:%+v",
		l.frame.ID(), l.frame.URL(), l.selector, opts,
	)

	opts.Strict = true
	if err := l.frame.fill(l.selector, "", opts); err != nil {
		return fmt.Errorf("clearing %q: %w", l.selector, err)
	}

	return nil
}

// Timeout will return the default timeout or the one set by the user.
func (l *Locator) Timeout() time.Duration {
	return l.frame.defaultTimeout()
}

// Click on an element using locator's selector with strict mode on.
func (l *Locator) Click(opts *FrameClickOptions) error {
	l.log.Debugf("Locator:Click", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)
	_, span := TraceAPICall(l.ctx, l.frame.page.targetID.String(), "locator.click")
	defer span.End()

	opts.Strict = true
	opts.retry = true
	if err := l.frame.click(l.selector, opts); err != nil {
		return spanRecordErrorf(span, "clicking on %q: %w", l.selector, err)
	}

	applySlowMo(l.ctx)

	return nil
}

func (l *Locator) All() ([]*Locator, error) {
	l.log.Debugf("Locator:All", "fid:%s furl:%q sel:%q", l.frame.ID(), l.frame.URL(), l.selector)

	count, err := l.Count()
	if err != nil {
		return nil, err
	}

	locators := make([]*Locator, count)
	for i := 0; i < count; i++ {
		locators[i] = l.Nth(i)
	}

	return locators, nil
}

// ContentFrame creates and returns a new FrameLocator, which is useful when
// needing to interact with elements in an iframe and the current locator already
// points to the iframe.
func (l *Locator) ContentFrame() *FrameLocator {
	return NewFrameLocator(l.ctx, l.selector, l.frame, l.log)
}

// Count APIs do not wait for the element to be present. It also does not set
// strict to true, allowing it to return the total number of elements matching
// the selector.
func (l *Locator) Count() (int, error) {
	return l.frame.count(l.selector)
}

// Dblclick double clicks on an element using locator's selector with strict mode on.
func (l *Locator) Dblclick(opts *FrameDblclickOptions) error {
	l.log.Debugf("Locator:Dblclick", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	opts.Strict = true
	opts.retry = true
	if err := l.frame.dblclick(l.selector, opts); err != nil {
		return fmt.Errorf("double clicking on %q: %w", l.selector, err)
	}

	applySlowMo(l.ctx)

	return nil
}

func (l *Locator) Evaluate(pageFunc string, args ...any) (any, error) {
	return l.frame.evaluateWithSelector(l.selector, pageFunc, args...)
}

func (l *Locator) EvaluateHandle(pageFunc string, args ...any) (JSHandleAPI, error) {
	return l.frame.evaluateHandleWithSelector(l.selector, pageFunc, args...)
}

// SetChecked sets the checked state of the element using locator's selector
// with strict mode on.
func (l *Locator) SetChecked(checked bool, opts *FrameCheckOptions) error {
	l.log.Debugf(
		"Locator:SetChecked", "fid:%s furl:%q sel:%q checked:%v opts:%+v",
		l.frame.ID(), l.frame.URL(), l.selector, checked, opts,
	)

	opts.Strict = true
	opts.retry = true
	if err := l.frame.setChecked(l.selector, checked, opts); err != nil {
		return fmt.Errorf("setting %q checked to %v: %w", l.selector, checked, err)
	}

	applySlowMo(l.ctx)

	return nil
}

// Check on an element using locator's selector with strict mode on.
func (l *Locator) Check(opts *FrameCheckOptions) error {
	l.log.Debugf("Locator:Check", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	opts.Strict = true
	opts.retry = true
	if err := l.frame.check(l.selector, opts); err != nil {
		return fmt.Errorf("checking %q: %w", l.selector, err)
	}

	applySlowMo(l.ctx)

	return nil
}

// Uncheck on an element using locator's selector with strict mode on.
func (l *Locator) Uncheck(opts *FrameUncheckOptions) error {
	l.log.Debugf("Locator:Uncheck", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	opts.Strict = true
	opts.retry = true
	if err := l.frame.uncheck(l.selector, opts); err != nil {
		return fmt.Errorf("unchecking %q: %w", l.selector, err)
	}

	applySlowMo(l.ctx)

	return nil
}

// IsChecked returns true if the element matches the locator's
// selector and is checked. Otherwise, returns false.
func (l *Locator) IsChecked(opts *FrameIsCheckedOptions) (bool, error) {
	l.log.Debugf("Locator:IsChecked", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	opts.Strict = true
	checked, err := l.frame.isChecked(l.selector, opts)
	if err != nil {
		return false, fmt.Errorf("checking is %q checked: %w", l.selector, err)
	}

	return checked, nil
}

// IsEditable returns true if the element matches the locator's
// selector and is Editable. Otherwise, returns false.
func (l *Locator) IsEditable(opts *FrameIsEditableOptions) (bool, error) {
	l.log.Debugf("Locator:IsEditable", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	opts.Strict = true
	editable, err := l.frame.isEditable(l.selector, opts)
	if err != nil {
		return false, fmt.Errorf("checking is %q editable: %w", l.selector, err)
	}

	return editable, nil
}

// IsEnabled returns true if the element matches the locator's
// selector and is Enabled. Otherwise, returns false.
func (l *Locator) IsEnabled(opts *FrameIsEnabledOptions) (bool, error) {
	l.log.Debugf("Locator:IsEnabled", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	opts.Strict = true
	enabled, err := l.frame.isEnabled(l.selector, opts)
	if err != nil {
		return false, fmt.Errorf("checking is %q enabled: %w", l.selector, err)
	}

	return enabled, nil
}

// IsDisabled returns true if the element matches the locator's
// selector and is disabled. Otherwise, returns false.
func (l *Locator) IsDisabled(opts *FrameIsDisabledOptions) (bool, error) {
	l.log.Debugf("Locator:IsDisabled", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	opts.Strict = true
	disabled, err := l.frame.isDisabled(l.selector, opts)
	if err != nil {
		return false, fmt.Errorf("checking is %q disabled: %w", l.selector, err)
	}

	return disabled, nil
}

// IsVisible returns true if the element matches the locator's
// selector and is visible. Otherwise, returns false.
func (l *Locator) IsVisible() (bool, error) {
	l.log.Debugf("Locator:IsVisible", "fid:%s furl:%q sel:%q", l.frame.ID(), l.frame.URL(), l.selector)

	visible, err := l.frame.isVisible(l.selector, &FrameIsVisibleOptions{Strict: true})
	if err != nil {
		return false, fmt.Errorf("checking is %q visible: %w", l.selector, err)
	}

	return visible, nil
}

// IsHidden returns true if the element matches the locator's
// selector and is hidden. Otherwise, returns false.
func (l *Locator) IsHidden() (bool, error) {
	l.log.Debugf("Locator:IsHidden", "fid:%s furl:%q sel:%q", l.frame.ID(), l.frame.URL(), l.selector)

	hidden, err := l.frame.isHidden(l.selector, &FrameIsHiddenOptions{Strict: true})
	if err != nil {
		return false, fmt.Errorf("checking is %q hidden: %w", l.selector, err)
	}

	return hidden, nil
}

// Fill out the element using locator's selector with strict mode on.
func (l *Locator) Fill(value string, opts *FrameFillOptions) error {
	l.log.Debugf(
		"Locator:Fill", "fid:%s furl:%q sel:%q val:%q opts:%+v",
		l.frame.ID(), l.frame.URL(), l.selector, value, opts,
	)

	opts.Strict = true
	if err := l.frame.fill(l.selector, value, opts); err != nil {
		return fmt.Errorf("filling %q with %q: %w", l.selector, value, err)
	}

	applySlowMo(l.ctx)

	return nil
}

// LocatorFilterOptions allows filtering a [Locator] by various criteria.
// It's similar to [LocatorOptions] but used for filtering existing locators.
type LocatorFilterOptions struct {
	*LocatorOptions
}

// Filter returns a new [Locator] after applying the options to the current one.
func (l *Locator) Filter(opts *LocatorFilterOptions) *Locator {
	return NewLocator(l.ctx, opts.LocatorOptions, l.selector, l.frame, l.log)
}

// First will return the first child of the element matching the locator's
// selector.
func (l *Locator) First() *Locator {
	return NewLocator(l.ctx, nil, l.selector+" >> nth=0", l.frame, l.log)
}

// Focus on the element using locator's selector with strict mode on.
func (l *Locator) Focus(opts *FrameBaseOptions) error {
	l.log.Debugf("Locator:Focus", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	opts.Strict = true
	if err := l.frame.focus(l.selector, opts); err != nil {
		return fmt.Errorf("focusing on %q: %w", l.selector, err)
	}

	applySlowMo(l.ctx)

	return nil
}

// GetAttribute of the element using locator's selector with strict mode on.
// The second return value is true if the attribute exists, and false otherwise.
func (l *Locator) GetAttribute(name string, opts *FrameBaseOptions) (string, bool, error) {
	l.log.Debugf(
		"Locator:GetAttribute", "fid:%s furl:%q sel:%q name:%q opts:%+v",
		l.frame.ID(), l.frame.URL(), l.selector, name, opts,
	)

	opts.Strict = true
	s, ok, err := l.frame.getAttribute(l.selector, name, opts)
	if err != nil {
		return "", false, fmt.Errorf("getting attribute %q of %q: %w", name, l.selector, err)
	}

	return s, ok, nil
}

// GetByAltText creates and returns a new relative locator that allows locating elements by their alt text.
func (l *Locator) GetByAltText(alt string, opts *GetByBaseOptions) *Locator {
	l.log.Debugf(
		"Locator:GetByAltText", "fid:%s furl:%q selector:%s alt:%q opts:%+v",
		l.frame.ID(), l.frame.URL(), l.selector, alt, opts,
	)

	return l.Locator(l.frame.buildAttributeSelector("alt", alt, opts), nil)
}

// GetByLabel creates and returns a new relative locator that allows locating input elements by the text
// of the associated `<label>` or `aria-labelledby` element, or by the `aria-label` attribute.
func (l *Locator) GetByLabel(label string, opts *GetByBaseOptions) *Locator {
	l.log.Debugf(
		"Locator:GetByLabel", "fid:%s furl:%q selector:%s label:%q opts:%+v",
		l.frame.ID(), l.frame.URL(), l.selector, label, opts,
	)

	return l.Locator(l.frame.buildLabelSelector(label, opts), nil)
}

// GetByPlaceholder creates and returns a new relative locator for this based on the placeholder attribute.
func (l *Locator) GetByPlaceholder(placeholder string, opts *GetByBaseOptions) *Locator {
	l.log.Debugf(
		"Locator:GetByPlaceholder", "fid:%s furl:%q selector:%s placeholder:%q opts:%+v",
		l.frame.ID(), l.frame.URL(), l.selector, placeholder, opts,
	)

	return l.Locator(l.frame.buildAttributeSelector("placeholder", placeholder, opts), nil)
}

// GetByRole creates and returns a new relative locator using the ARIA role and any additional options.
func (l *Locator) GetByRole(role string, opts *GetByRoleOptions) *Locator {
	l.log.Debugf(
		"Locator:GetByRole", "fid:%s furl:%q selector:%s role:%q opts:%+v",
		l.frame.ID(), l.frame.URL(), l.selector, role, opts,
	)

	return l.Locator(l.frame.buildRoleSelector(role, opts), nil)
}

// GetByTestID creates and returns a new relative locator based on the data-testid attribute.
func (l *Locator) GetByTestID(testID string) *Locator {
	l.log.Debugf(
		"Locator:GetByTestID", "fid:%s furl:%q selector:%s testID:%q",
		l.frame.ID(), l.frame.URL(), l.selector, testID,
	)

	return l.Locator(l.frame.buildTestIDSelector(testID), nil)
}

// GetByText creates and returns a new relative locator based on text content.
func (l *Locator) GetByText(text string, opts *GetByBaseOptions) *Locator {
	l.log.Debugf(
		"Locator:GetByText", "fid:%s furl:%q selector:%s text:%q opts:%+v",
		l.frame.ID(), l.frame.URL(), l.selector, text, opts,
	)

	return l.Locator(l.frame.buildTextSelector(text, opts), nil)
}

// GetByTitle creates and returns a new relative locator based on the title attribute.
func (l *Locator) GetByTitle(title string, opts *GetByBaseOptions) *Locator {
	l.log.Debugf(
		"Locator:GetByTitle", "fid:%s furl:%q selector:%s title:%q opts:%+v",
		l.frame.ID(), l.frame.URL(), l.selector, title, opts,
	)

	return l.Locator(l.frame.buildAttributeSelector("title", title, opts), nil)
}

// Locator creates and returns a new locator chained/relative to the current locator.
func (l *Locator) Locator(selector string, opts *LocatorOptions) *Locator {
	return NewLocator(l.ctx, opts, l.selector+" >> "+selector, l.frame, l.log)
}

// FrameLocator creates a frame locator for an iframe matching the given selector
// within the current locator's scope.
func (l *Locator) FrameLocator(selector string) *FrameLocator {
	l.log.Debugf("Locator:FrameLocator", "selector:%q childSelector:%q", l.selector, selector)

	return l.Locator(selector, nil).ContentFrame()
}

// InnerHTML returns the element's inner HTML that matches
// the locator's selector with strict mode on.
func (l *Locator) InnerHTML(opts *FrameInnerHTMLOptions) (string, error) {
	l.log.Debugf("Locator:InnerHTML", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	opts.Strict = true
	s, err := l.frame.innerHTML(l.selector, opts)
	if err != nil {
		return "", fmt.Errorf("getting inner HTML of %q: %w", l.selector, err)
	}

	return s, nil
}

// InnerText returns the element's inner text that matches
// the locator's selector with strict mode on.
func (l *Locator) InnerText(opts *FrameInnerTextOptions) (string, error) {
	l.log.Debugf("Locator:InnerText", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	opts.Strict = true
	s, err := l.frame.innerText(l.selector, opts)
	if err != nil {
		return "", fmt.Errorf("getting inner text of %q: %w", l.selector, err)
	}

	return s, nil
}

// Last will return the last child of the element matching the locator's
// selector.
func (l *Locator) Last() *Locator {
	return NewLocator(l.ctx, nil, l.selector+" >> nth=-1", l.frame, l.log)
}

// Nth will return the nth child of the element matching the locator's
// selector.
func (l *Locator) Nth(nth int) *Locator {
	return NewLocator(l.ctx, nil, l.selector+" >> nth="+strconv.Itoa(nth), l.frame, l.log)
}

// TextContent returns the element's text content that matches
// the locator's selector with strict mode on. The second return
// value is true if the returned text content is not null or empty,
// and false otherwise.
func (l *Locator) TextContent(opts *FrameTextContentOptions) (string, bool, error) {
	l.log.Debugf("Locator:TextContent", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	opts.Strict = true
	s, ok, err := l.frame.textContent(l.selector, opts)
	if err != nil {
		return "", false, fmt.Errorf("getting text content of %q: %w", l.selector, err)
	}

	return s, ok, nil
}

// InputValue returns the element's input value that matches
// the locator's selector with strict mode on.
func (l *Locator) InputValue(opts *FrameInputValueOptions) (string, error) {
	l.log.Debugf("Locator:InputValue", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	opts.Strict = true
	v, err := l.frame.inputValue(l.selector, opts)
	if err != nil {
		return "", fmt.Errorf("getting input value of %q: %w", l.selector, err)
	}

	return v, nil
}

// SelectOption filters option values of the first element that matches
// the locator's selector (with strict mode on), selects the options,
// and returns the filtered options.
func (l *Locator) SelectOption(values []any, opts *FrameSelectOptionOptions) ([]string, error) {
	l.log.Debugf("Locator:SelectOption", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	opts.Strict = true
	v, err := l.frame.selectOption(l.selector, values, opts)
	if err != nil {
		return nil, fmt.Errorf("selecting option on %q: %w", l.selector, err)
	}

	applySlowMo(l.ctx)

	return v, nil
}

// Press the given key on the element found that matches the locator's
// selector with strict mode on.
func (l *Locator) Press(key string, opts *FramePressOptions) error {
	l.log.Debugf(
		"Locator:Press", "fid:%s furl:%q sel:%q key:%q opts:%+v",
		l.frame.ID(), l.frame.URL(), l.selector, key, opts,
	)

	opts.Strict = true
	if err := l.frame.press(l.selector, key, opts); err != nil {
		return fmt.Errorf("pressing %q on %q: %w", key, l.selector, err)
	}

	applySlowMo(l.ctx)

	return nil
}

// PressSequentially focuses on the element and sequentially sends a keydown,
// keypress, and keyup events for each character in the provided string.
// For handling special keys, use the [Locator.Press] method.
func (l *Locator) PressSequentially(text string, opts *FrameTypeOptions) error {
	l.log.Debugf(
		"Locator:PressSequentially", "fid:%s furl:%q sel:%q text:%q opts:%+v",
		l.frame.ID(), l.frame.URL(), l.selector, text, opts,
	)
	_, span := TraceAPICall(l.ctx, l.frame.page.targetID.String(), "locator.pressSequentially")
	defer span.End()

	opts.Strict = true
	if err := l.frame.typ(l.selector, text, opts); err != nil {
		return spanRecordErrorf(span, "pressing sequentially %q on %q: %w", text, l.selector, err)
	}

	applySlowMo(l.ctx)

	return nil
}

// Type text on the element found that matches the locator's
// selector with strict mode on.
func (l *Locator) Type(text string, opts *FrameTypeOptions) error {
	l.log.Debugf(
		"Locator:Type", "fid:%s furl:%q sel:%q text:%q opts:%+v",
		l.frame.ID(), l.frame.URL(), l.selector, text, opts,
	)
	_, span := TraceAPICall(l.ctx, l.frame.page.targetID.String(), "locator.type")
	defer span.End()

	opts.Strict = true
	if err := l.frame.typ(l.selector, text, opts); err != nil {
		return spanRecordErrorf(span, "typing %q in %q: %w", text, l.selector, err)
	}

	applySlowMo(l.ctx)

	return nil
}

// Hover moves the pointer over the element that matches the locator's
// selector with strict mode on.
func (l *Locator) Hover(opts *FrameHoverOptions) error {
	l.log.Debugf("Locator:Hover", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	opts.Strict = true
	opts.retry = true
	if err := l.frame.hover(l.selector, opts); err != nil {
		return fmt.Errorf("hovering on %q: %w", l.selector, err)
	}

	applySlowMo(l.ctx)

	return nil
}

// Tap the element found that matches the locator's selector with strict mode on.
func (l *Locator) Tap(opts *FrameTapOptions) error {
	l.log.Debugf("Locator:Tap", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	opts.Strict = true
	opts.retry = true
	if err := l.frame.tap(l.selector, opts); err != nil {
		return fmt.Errorf("tapping on %q: %w", l.selector, err)
	}

	applySlowMo(l.ctx)

	return nil
}

// DispatchEvent dispatches an event for the element matching the
// locator's selector with strict mode on.
func (l *Locator) DispatchEvent(typ string, eventInit any, opts *FrameDispatchEventOptions) error {
	l.log.Debugf(
		"Locator:DispatchEvent", "fid:%s furl:%q sel:%q typ:%q eventInit:%+v opts:%+v",
		l.frame.ID(), l.frame.URL(), l.selector, typ, eventInit, opts,
	)

	opts.Strict = true
	if err := l.frame.dispatchEvent(l.selector, typ, eventInit, opts); err != nil {
		return fmt.Errorf("dispatching locator event %q to %q: %w", typ, l.selector, err)
	}

	applySlowMo(l.ctx)

	return nil
}

// WaitFor waits for the element matching the locator's selector with strict mode on.
func (l *Locator) WaitFor(opts *FrameWaitForSelectorOptions) error {
	l.log.Debugf("Locator:WaitFor", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	opts.Strict = true
	_, err := l.frame.waitFor(l.selector, opts, 20)
	if err != nil {
		return fmt.Errorf("waiting for %q: %w", l.selector, err)
	}

	return nil
}

// DefaultTimeout returns the default timeout for the locator.
// This is an internal API and should not be used by users.
func (l *Locator) DefaultTimeout() time.Duration {
	return l.frame.defaultTimeout()
}

// FrameLocator represent a way to find element(s) in an iframe.
type FrameLocator struct {
	selector string

	frame *Frame

	ctx context.Context
	log *log.Logger
}

// NewFrameLocator creates and returns a new frame locator.
func NewFrameLocator(ctx context.Context, selector string, f *Frame, l *log.Logger) *FrameLocator {
	return &FrameLocator{
		selector: selector,
		frame:    f,
		ctx:      ctx,
		log:      l,
	}
}

// GetByAltText creates and returns a new locator for this frame locator
// based on the alt attribute text.
func (fl *FrameLocator) GetByAltText(alt string, opts *GetByBaseOptions) *Locator {
	fl.log.Debugf("FrameLocator:GetByAltText", "selector: %q alt: %q opts:%+v", fl.selector, alt, opts)

	return fl.Locator(fl.frame.buildAttributeSelector("alt", alt, opts), nil)
}

// GetByLabel creates and returns a new locator for this frame locator based on the label text.
func (fl *FrameLocator) GetByLabel(label string, opts *GetByBaseOptions) *Locator {
	fl.log.Debugf("FrameLocator:GetByLabel", "selector: %q label: %q opts:%+v", fl.selector, label, opts)

	return fl.Locator(fl.frame.buildLabelSelector(label, opts), nil)
}

// GetByPlaceholder creates and returns a new locator for this frame locator based on the placeholder attribute.
func (fl *FrameLocator) GetByPlaceholder(placeholder string, opts *GetByBaseOptions) *Locator {
	fl.log.Debugf("FrameLocator:GetByPlaceholder", "selector: %q placeholder: %q opts:%+v", fl.selector, placeholder, opts)

	return fl.Locator(fl.frame.buildAttributeSelector("placeholder", placeholder, opts), nil)
}

// GetByRole creates and returns a new locator for this frame locator based on their ARIA role.
func (fl *FrameLocator) GetByRole(role string, opts *GetByRoleOptions) *Locator {
	fl.log.Debugf("FrameLocator:GetByRole", "selector: %q role: %q opts:%+v", fl.selector, role, opts)

	return fl.Locator(fl.frame.buildRoleSelector(role, opts), nil)
}

// GetByTestID creates and returns a new locator for this frame locator based on the data-testid attribute.
func (fl *FrameLocator) GetByTestID(testID string) *Locator {
	fl.log.Debugf("FrameLocator:GetByTestID", "selector: %q testID: %q", fl.selector, testID)

	return fl.Locator(fl.frame.buildTestIDSelector(testID), nil)
}

// GetByText creates and returns a new locator for this frame locator based on text content.
func (fl *FrameLocator) GetByText(text string, opts *GetByBaseOptions) *Locator {
	fl.log.Debugf("FrameLocator:GetByText", "selector: %q text: %q opts:%+v", fl.selector, text, opts)

	return fl.Locator(fl.frame.buildTextSelector(text, opts), nil)
}

// GetByTitle creates and returns a new locator for this frame locator based on the title attribute.
func (fl *FrameLocator) GetByTitle(title string, opts *GetByBaseOptions) *Locator {
	fl.log.Debugf("FrameLocator:GetByTitle", "selector: %q title: %q opts:%+v", fl.selector, title, opts)

	return fl.Locator(fl.frame.buildAttributeSelector("title", title, opts), nil)
}

// Locator creates and returns a new locator chained/relative to the current FrameLocator.
func (fl *FrameLocator) Locator(selector string, opts *LocatorOptions) *Locator {
	// Add frame navigation marker to indicate we need to enter the frame's contentDocument
	frameNavSelector := fl.selector + " >> internal:control=enter-frame >> " + selector
	return NewLocator(fl.ctx, opts, frameNavSelector, fl.frame, fl.log)
}

// FrameLocator creates a nested frame locator for an iframe matching the given
func (fl *FrameLocator) FrameLocator(selector string) *FrameLocator {
	fl.log.Debugf("FrameLocator:FrameLocator", "selector:%q childSelector:%q", fl.selector, selector)

	return fl.Locator(selector, nil).ContentFrame()
}
