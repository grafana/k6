package common

import (
	"context"
	"fmt"
	"time"

	"github.com/grafana/sobek"

	"go.k6.io/k6/internal/js/modules/k6/browser/log"
)

// Strict mode:
// All operations on locators throw an exception if more
// than one element matches the locator's selector.
//
// See Issue #100 for more details.

// Locator represent a way to find element(s) on the page at any moment.
type Locator struct {
	selector string

	frame *Frame

	ctx context.Context
	log *log.Logger
}

// NewLocator creates and returns a new locator.
func NewLocator(ctx context.Context, selector string, f *Frame, l *log.Logger) *Locator {
	return &Locator{
		selector: selector,
		frame:    f,
		ctx:      ctx,
		log:      l,
	}
}

// Clear will clear the input field.
// This works with the Fill API and fills the input field with an empty string.
func (l *Locator) Clear(opts *FrameFillOptions) error {
	l.log.Debugf(
		"Locator:Clear", "fid:%s furl:%q sel:%q opts:%+v",
		l.frame.ID(), l.frame.URL(), l.selector, opts,
	)

	if err := l.fill("", opts); err != nil {
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

	if err := l.click(opts); err != nil {
		err := fmt.Errorf("clicking on %q: %w", l.selector, err)
		spanRecordError(span, err)
		return err
	}

	applySlowMo(l.ctx)

	return nil
}

// click is like Click but takes parsed options and neither throws an
// error, or applies slow motion.
func (l *Locator) click(opts *FrameClickOptions) error {
	opts.Strict = true
	return l.frame.click(l.selector, opts)
}

// Dblclick double clicks on an element using locator's selector with strict mode on.
func (l *Locator) Dblclick(opts sobek.Value) error {
	l.log.Debugf("Locator:Dblclick", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	copts := NewFrameDblClickOptions(l.frame.defaultTimeout())
	if err := copts.Parse(l.ctx, opts); err != nil {
		return fmt.Errorf("parsing double click options: %w", err)
	}
	if err := l.dblclick(copts); err != nil {
		return fmt.Errorf("double clicking on %q: %w", l.selector, err)
	}

	applySlowMo(l.ctx)

	return nil
}

// Dblclick is like Dblclick but takes parsed options and neither throws an
// error, or applies slow motion.
func (l *Locator) dblclick(opts *FrameDblclickOptions) error {
	opts.Strict = true
	return l.frame.dblclick(l.selector, opts)
}

// SetChecked sets the checked state of the element using locator's selector
// with strict mode on.
func (l *Locator) SetChecked(checked bool, opts sobek.Value) error {
	l.log.Debugf(
		"Locator:SetChecked", "fid:%s furl:%q sel:%q checked:%v opts:%+v",
		l.frame.ID(), l.frame.URL(), l.selector, checked, opts,
	)

	copts := NewFrameCheckOptions(l.frame.defaultTimeout())
	if err := copts.Parse(l.ctx, opts); err != nil {
		return fmt.Errorf("parsing set checked options: %w", err)
	}
	if err := l.setChecked(checked, copts); err != nil {
		return fmt.Errorf("setting %q checked to %v: %w", l.selector, checked, err)
	}

	applySlowMo(l.ctx)

	return nil
}

func (l *Locator) setChecked(checked bool, opts *FrameCheckOptions) error {
	opts.Strict = true
	return l.frame.setChecked(l.selector, checked, opts)
}

// Check on an element using locator's selector with strict mode on.
func (l *Locator) Check(opts sobek.Value) error {
	l.log.Debugf("Locator:Check", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	copts := NewFrameCheckOptions(l.frame.defaultTimeout())
	if err := copts.Parse(l.ctx, opts); err != nil {
		return fmt.Errorf("parsing check options: %w", err)
	}
	if err := l.check(copts); err != nil {
		return fmt.Errorf("checking %q: %w", l.selector, err)
	}

	applySlowMo(l.ctx)

	return nil
}

// check is like Check but takes parsed options and neither throws an
// error, or applies slow motion.
func (l *Locator) check(opts *FrameCheckOptions) error {
	opts.Strict = true
	return l.frame.check(l.selector, opts)
}

// Uncheck on an element using locator's selector with strict mode on.
func (l *Locator) Uncheck(opts sobek.Value) error {
	l.log.Debugf("Locator:Uncheck", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	copts := NewFrameUncheckOptions(l.frame.defaultTimeout())
	if err := copts.Parse(l.ctx, opts); err != nil {
		return fmt.Errorf("parsing uncheck options: %w", err)
	}
	if err := l.uncheck(copts); err != nil {
		return fmt.Errorf("unchecking %q: %w", l.selector, err)
	}

	applySlowMo(l.ctx)

	return nil
}

// uncheck is like Uncheck but takes parsed options and neither throws
// an error, or applies slow motion.
func (l *Locator) uncheck(opts *FrameUncheckOptions) error {
	opts.Strict = true
	return l.frame.uncheck(l.selector, opts)
}

// IsChecked returns true if the element matches the locator's
// selector and is checked. Otherwise, returns false.
func (l *Locator) IsChecked(opts sobek.Value) (bool, error) {
	l.log.Debugf("Locator:IsChecked", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	copts := NewFrameIsCheckedOptions(l.frame.defaultTimeout())
	if err := copts.Parse(l.ctx, opts); err != nil {
		return false, fmt.Errorf("parsing is checked options: %w", err)
	}
	checked, err := l.isChecked(copts)
	if err != nil {
		return false, fmt.Errorf("checking is %q checked: %w", l.selector, err)
	}

	return checked, nil
}

// isChecked is like IsChecked but takes parsed options and does not
// throw an error.
func (l *Locator) isChecked(opts *FrameIsCheckedOptions) (bool, error) {
	opts.Strict = true
	return l.frame.isChecked(l.selector, opts)
}

// IsEditable returns true if the element matches the locator's
// selector and is Editable. Otherwise, returns false.
func (l *Locator) IsEditable(opts sobek.Value) (bool, error) {
	l.log.Debugf("Locator:IsEditable", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	copts := NewFrameIsEditableOptions(l.frame.defaultTimeout())
	if err := copts.Parse(l.ctx, opts); err != nil {
		return false, fmt.Errorf("parsing is editable options: %w", err)
	}
	editable, err := l.isEditable(copts)
	if err != nil {
		return false, fmt.Errorf("checking is %q editable: %w", l.selector, err)
	}

	return editable, nil
}

// isEditable is like IsEditable but takes parsed options and does not
// throw an error.
func (l *Locator) isEditable(opts *FrameIsEditableOptions) (bool, error) {
	opts.Strict = true
	return l.frame.isEditable(l.selector, opts)
}

// IsEnabled returns true if the element matches the locator's
// selector and is Enabled. Otherwise, returns false.
func (l *Locator) IsEnabled(opts sobek.Value) (bool, error) {
	l.log.Debugf("Locator:IsEnabled", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	copts := NewFrameIsEnabledOptions(l.frame.defaultTimeout())
	if err := copts.Parse(l.ctx, opts); err != nil {
		return false, fmt.Errorf("parsing is enabled options: %w", err)
	}
	enabled, err := l.isEnabled(copts)
	if err != nil {
		return false, fmt.Errorf("checking is %q enabled: %w", l.selector, err)
	}

	return enabled, nil
}

// isEnabled is like IsEnabled but takes parsed options and does not
// throw an error.
func (l *Locator) isEnabled(opts *FrameIsEnabledOptions) (bool, error) {
	opts.Strict = true
	return l.frame.isEnabled(l.selector, opts)
}

// IsDisabled returns true if the element matches the locator's
// selector and is disabled. Otherwise, returns false.
func (l *Locator) IsDisabled(opts sobek.Value) (bool, error) {
	l.log.Debugf("Locator:IsDisabled", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	copts := NewFrameIsDisabledOptions(l.frame.defaultTimeout())
	if err := copts.Parse(l.ctx, opts); err != nil {
		return false, fmt.Errorf("parsing is disabled options: %w", err)
	}
	disabled, err := l.isDisabled(copts)
	if err != nil {
		return false, fmt.Errorf("checking is %q disabled: %w", l.selector, err)
	}

	return disabled, nil
}

// IsDisabled is like IsDisabled but takes parsed options and does not
// throw an error.
func (l *Locator) isDisabled(opts *FrameIsDisabledOptions) (bool, error) {
	opts.Strict = true
	return l.frame.isDisabled(l.selector, opts)
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
func (l *Locator) Fill(value string, opts sobek.Value) error {
	l.log.Debugf(
		"Locator:Fill", "fid:%s furl:%q sel:%q val:%q opts:%+v",
		l.frame.ID(), l.frame.URL(), l.selector, value, opts,
	)

	copts := NewFrameFillOptions(l.frame.defaultTimeout())
	if err := copts.Parse(l.ctx, opts); err != nil {
		return fmt.Errorf("parsing fill options: %w", err)
	}
	if err := l.fill(value, copts); err != nil {
		return fmt.Errorf("filling %q with %q: %w", l.selector, value, err)
	}

	applySlowMo(l.ctx)

	return nil
}

func (l *Locator) fill(value string, opts *FrameFillOptions) error {
	opts.Strict = true
	return l.frame.fill(l.selector, value, opts)
}

// Focus on the element using locator's selector with strict mode on.
func (l *Locator) Focus(opts sobek.Value) error {
	l.log.Debugf("Locator:Focus", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	copts := NewFrameBaseOptions(l.frame.defaultTimeout())
	if err := copts.Parse(l.ctx, opts); err != nil {
		return fmt.Errorf("parsing focus options: %w", err)
	}
	if err := l.focus(copts); err != nil {
		return fmt.Errorf("focusing on %q: %w", l.selector, err)
	}

	applySlowMo(l.ctx)

	return nil
}

func (l *Locator) focus(opts *FrameBaseOptions) error {
	opts.Strict = true
	return l.frame.focus(l.selector, opts)
}

// GetAttribute of the element using locator's selector with strict mode on.
// The second return value is true if the attribute exists, and false otherwise.
func (l *Locator) GetAttribute(name string, opts sobek.Value) (string, bool, error) {
	l.log.Debugf(
		"Locator:GetAttribute", "fid:%s furl:%q sel:%q name:%q opts:%+v",
		l.frame.ID(), l.frame.URL(), l.selector, name, opts,
	)

	copts := NewFrameBaseOptions(l.frame.defaultTimeout())
	if err := copts.Parse(l.ctx, opts); err != nil {
		return "", false, fmt.Errorf("parsing get attribute options: %w", err)
	}
	s, ok, err := l.getAttribute(name, copts)
	if err != nil {
		return "", false, fmt.Errorf("getting attribute %q of %q: %w", name, l.selector, err)
	}

	return s, ok, nil
}

func (l *Locator) getAttribute(name string, opts *FrameBaseOptions) (string, bool, error) {
	opts.Strict = true
	return l.frame.getAttribute(l.selector, name, opts)
}

// InnerHTML returns the element's inner HTML that matches
// the locator's selector with strict mode on.
func (l *Locator) InnerHTML(opts sobek.Value) (string, error) {
	l.log.Debugf("Locator:InnerHTML", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	copts := NewFrameInnerHTMLOptions(l.frame.defaultTimeout())
	if err := copts.Parse(l.ctx, opts); err != nil {
		return "", fmt.Errorf("parsing inner HTML options: %w", err)
	}
	s, err := l.innerHTML(copts)
	if err != nil {
		return "", fmt.Errorf("getting inner HTML of %q: %w", l.selector, err)
	}

	return s, nil
}

func (l *Locator) innerHTML(opts *FrameInnerHTMLOptions) (string, error) {
	opts.Strict = true
	return l.frame.innerHTML(l.selector, opts)
}

// InnerText returns the element's inner text that matches
// the locator's selector with strict mode on.
func (l *Locator) InnerText(opts sobek.Value) (string, error) {
	l.log.Debugf("Locator:InnerText", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	copts := NewFrameInnerTextOptions(l.frame.defaultTimeout())
	if err := copts.Parse(l.ctx, opts); err != nil {
		return "", fmt.Errorf("parsing inner text options: %w", err)
	}
	s, err := l.innerText(copts)
	if err != nil {
		return "", fmt.Errorf("getting inner text of %q: %w", l.selector, err)
	}

	return s, nil
}

func (l *Locator) innerText(opts *FrameInnerTextOptions) (string, error) {
	opts.Strict = true
	return l.frame.innerText(l.selector, opts)
}

// TextContent returns the element's text content that matches
// the locator's selector with strict mode on. The second return
// value is true if the returned text content is not null or empty,
// and false otherwise.
func (l *Locator) TextContent(opts sobek.Value) (string, bool, error) {
	l.log.Debugf("Locator:TextContent", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	copts := NewFrameTextContentOptions(l.frame.defaultTimeout())
	if err := copts.Parse(l.ctx, opts); err != nil {
		return "", false, fmt.Errorf("parsing text context options: %w", err)
	}
	s, ok, err := l.textContent(copts)
	if err != nil {
		return "", false, fmt.Errorf("getting text content of %q: %w", l.selector, err)
	}

	return s, ok, nil
}

func (l *Locator) textContent(opts *FrameTextContentOptions) (string, bool, error) {
	opts.Strict = true
	return l.frame.textContent(l.selector, opts)
}

// InputValue returns the element's input value that matches
// the locator's selector with strict mode on.
func (l *Locator) InputValue(opts sobek.Value) (string, error) {
	l.log.Debugf("Locator:InputValue", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	copts := NewFrameInputValueOptions(l.frame.defaultTimeout())
	if err := copts.Parse(l.ctx, opts); err != nil {
		return "", fmt.Errorf("parsing input value options: %w", err)
	}
	v, err := l.inputValue(copts)
	if err != nil {
		return "", fmt.Errorf("getting input value of %q: %w", l.selector, err)
	}

	return v, nil
}

func (l *Locator) inputValue(opts *FrameInputValueOptions) (string, error) {
	opts.Strict = true
	return l.frame.inputValue(l.selector, opts)
}

// SelectOption filters option values of the first element that matches
// the locator's selector (with strict mode on), selects the options,
// and returns the filtered options.
func (l *Locator) SelectOption(values sobek.Value, opts sobek.Value) ([]string, error) {
	l.log.Debugf("Locator:SelectOption", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	copts := NewFrameSelectOptionOptions(l.frame.defaultTimeout())
	if err := copts.Parse(l.ctx, opts); err != nil {
		return nil, fmt.Errorf("parsing select option options: %w", err)
	}
	convValues, err := ConvertSelectOptionValues(l.frame.vu.Runtime(), values)
	if err != nil {
		return nil, fmt.Errorf("parsing select option values: %w", err)
	}
	v, err := l.selectOption(convValues, copts)
	if err != nil {
		return nil, fmt.Errorf("selecting option on %q: %w", l.selector, err)
	}

	applySlowMo(l.ctx)

	return v, nil
}

func (l *Locator) selectOption(values []any, opts *FrameSelectOptionOptions) ([]string, error) {
	opts.Strict = true
	return l.frame.selectOption(l.selector, values, opts)
}

// Press the given key on the element found that matches the locator's
// selector with strict mode on.
func (l *Locator) Press(key string, opts sobek.Value) error {
	l.log.Debugf(
		"Locator:Press", "fid:%s furl:%q sel:%q key:%q opts:%+v",
		l.frame.ID(), l.frame.URL(), l.selector, key, opts,
	)

	copts := NewFramePressOptions(l.frame.defaultTimeout())
	if err := copts.Parse(l.ctx, opts); err != nil {
		return fmt.Errorf("parsing press options: %w", err)
	}
	if err := l.press(key, copts); err != nil {
		return fmt.Errorf("pressing %q on %q: %w", key, l.selector, err)
	}

	applySlowMo(l.ctx)

	return nil
}

func (l *Locator) press(key string, opts *FramePressOptions) error {
	opts.Strict = true
	return l.frame.press(l.selector, key, opts)
}

// Type text on the element found that matches the locator's
// selector with strict mode on.
func (l *Locator) Type(text string, opts sobek.Value) error {
	l.log.Debugf(
		"Locator:Type", "fid:%s furl:%q sel:%q text:%q opts:%+v",
		l.frame.ID(), l.frame.URL(), l.selector, text, opts,
	)
	_, span := TraceAPICall(l.ctx, l.frame.page.targetID.String(), "locator.type")
	defer span.End()

	copts := NewFrameTypeOptions(l.frame.defaultTimeout())
	if err := copts.Parse(l.ctx, opts); err != nil {
		err := fmt.Errorf("parsing type options: %w", err)
		spanRecordError(span, err)
		return err
	}
	if err := l.typ(text, copts); err != nil {
		err := fmt.Errorf("typing %q in %q: %w", text, l.selector, err)
		spanRecordError(span, err)
		return err
	}

	applySlowMo(l.ctx)

	return nil
}

func (l *Locator) typ(text string, opts *FrameTypeOptions) error {
	opts.Strict = true
	return l.frame.typ(l.selector, text, opts)
}

// Hover moves the pointer over the element that matches the locator's
// selector with strict mode on.
func (l *Locator) Hover(opts sobek.Value) error {
	l.log.Debugf("Locator:Hover", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	copts := NewFrameHoverOptions(l.frame.defaultTimeout())
	if err := copts.Parse(l.ctx, opts); err != nil {
		return fmt.Errorf("parsing hover options: %w", err)
	}
	if err := l.hover(copts); err != nil {
		return fmt.Errorf("hovering on %q: %w", l.selector, err)
	}

	applySlowMo(l.ctx)

	return nil
}

func (l *Locator) hover(opts *FrameHoverOptions) error {
	opts.Strict = true
	return l.frame.hover(l.selector, opts)
}

// Tap the element found that matches the locator's selector with strict mode on.
func (l *Locator) Tap(opts *FrameTapOptions) error {
	l.log.Debugf("Locator:Tap", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	opts.Strict = true
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

	if err := l.dispatchEvent(typ, eventInit, opts); err != nil {
		return fmt.Errorf("dispatching locator event %q to %q: %w", typ, l.selector, err)
	}

	applySlowMo(l.ctx)

	return nil
}

func (l *Locator) dispatchEvent(typ string, eventInit any, opts *FrameDispatchEventOptions) error {
	opts.Strict = true
	return l.frame.dispatchEvent(l.selector, typ, eventInit, opts)
}

// WaitFor waits for the element matching the locator's selector with strict mode on.
func (l *Locator) WaitFor(opts sobek.Value) error {
	l.log.Debugf("Locator:WaitFor", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	popts := NewFrameWaitForSelectorOptions(l.frame.defaultTimeout())
	if err := popts.Parse(l.ctx, opts); err != nil {
		return fmt.Errorf("parsing wait for options: %w", err)
	}
	if err := l.waitFor(popts); err != nil {
		return fmt.Errorf("waiting for %q: %w", l.selector, err)
	}

	return nil
}

func (l *Locator) waitFor(opts *FrameWaitForSelectorOptions) error {
	opts.Strict = true
	_, err := l.frame.waitFor(l.selector, opts, 20)
	return err
}

// DefaultTimeout returns the default timeout for the locator.
// This is an internal API and should not be used by users.
func (l *Locator) DefaultTimeout() time.Duration {
	return l.frame.defaultTimeout()
}
