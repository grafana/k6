package common

import (
	"context"

	"github.com/grafana/xk6-browser/k6ext"
	"github.com/grafana/xk6-browser/log"

	"github.com/dop251/goja"
)

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

// Click on an element using locator's selector with strict mode on.
func (l *Locator) Click(opts goja.Value) {
	l.log.Debugf("Locator:Click", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	var err error
	defer func() { panicOrSlowMo(l.ctx, err) }()

	copts := NewFrameClickOptions(l.frame.defaultTimeout())
	if err = copts.Parse(l.ctx, opts); err != nil {
		return
	}
	if err = l.click(copts); err != nil {
		return
	}
}

// click is like Click but takes parsed options and neither throws an
// error, or applies slow motion.
func (l *Locator) click(opts *FrameClickOptions) error {
	opts.Strict = true
	return l.frame.click(l.selector, opts)
}

// Dblclick double clicks on an element using locator's selector with strict mode on.
func (l *Locator) Dblclick(opts goja.Value) {
	l.log.Debugf("Locator:Dblclick", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	var err error
	defer func() { panicOrSlowMo(l.ctx, err) }()

	copts := NewFrameDblClickOptions(l.frame.defaultTimeout())
	if err = copts.Parse(l.ctx, opts); err != nil {
		return
	}
	if err = l.dblclick(copts); err != nil {
		return
	}
}

// Dblclick is like Dblclick but takes parsed options and neither throws an
// error, or applies slow motion.
func (l *Locator) dblclick(opts *FrameDblclickOptions) error {
	opts.Strict = true
	return l.frame.dblclick(l.selector, opts)
}

// Check on an element using locator's selector with strict mode on.
func (l *Locator) Check(opts goja.Value) {
	l.log.Debugf("Locator:Check", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	var err error
	defer func() { panicOrSlowMo(l.ctx, err) }()

	copts := NewFrameCheckOptions(l.frame.defaultTimeout())
	if err = copts.Parse(l.ctx, opts); err != nil {
		return
	}
	if err = l.check(copts); err != nil {
		return
	}
}

// check is like Check but takes parsed options and neither throws an
// error, or applies slow motion.
func (l *Locator) check(opts *FrameCheckOptions) error {
	opts.Strict = true
	return l.frame.check(l.selector, opts)
}

// Uncheck on an element using locator's selector with strict mode on.
func (l *Locator) Uncheck(opts goja.Value) {
	l.log.Debugf("Locator:Uncheck", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	var err error
	defer func() { panicOrSlowMo(l.ctx, err) }()

	copts := NewFrameUncheckOptions(l.frame.defaultTimeout())
	if err = copts.Parse(l.ctx, opts); err != nil {
		return
	}
	if err = l.uncheck(copts); err != nil {
		return
	}
}

// uncheck is like Uncheck but takes parsed options and neither throws
// an error, or applies slow motion.
func (l *Locator) uncheck(opts *FrameUncheckOptions) error {
	opts.Strict = true
	return l.frame.uncheck(l.selector, opts)
}

// IsChecked returns true if the element matches the locator's
// selector and is checked. Otherwise, returns false.
func (l *Locator) IsChecked(opts goja.Value) bool {
	l.log.Debugf("Locator:IsChecked", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	copts := NewFrameIsCheckedOptions(l.frame.defaultTimeout())
	if err := copts.Parse(l.ctx, opts); err != nil {
		k6ext.Panic(l.ctx, "%w", err)
	}
	checked, err := l.isChecked(copts)
	if err != nil {
		k6ext.Panic(l.ctx, "%w", err)
	}

	return checked
}

// isChecked is like IsChecked but takes parsed options and does not
// throw an error.
func (l *Locator) isChecked(opts *FrameIsCheckedOptions) (bool, error) {
	opts.Strict = true
	return l.frame.isChecked(l.selector, opts)
}

// IsEditable returns true if the element matches the locator's
// selector and is Editable. Otherwise, returns false.
func (l *Locator) IsEditable(opts goja.Value) bool {
	l.log.Debugf("Locator:IsEditable", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	copts := NewFrameIsEditableOptions(l.frame.defaultTimeout())
	if err := copts.Parse(l.ctx, opts); err != nil {
		k6ext.Panic(l.ctx, "%w", err)
	}
	editable, err := l.isEditable(copts)
	if err != nil {
		k6ext.Panic(l.ctx, "%w", err)
	}

	return editable
}

// isEditable is like IsEditable but takes parsed options and does not
// throw an error.
func (l *Locator) isEditable(opts *FrameIsEditableOptions) (bool, error) {
	opts.Strict = true
	return l.frame.isEditable(l.selector, opts)
}

// IsEnabled returns true if the element matches the locator's
// selector and is Enabled. Otherwise, returns false.
func (l *Locator) IsEnabled(opts goja.Value) bool {
	l.log.Debugf("Locator:IsEnabled", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	copts := NewFrameIsEnabledOptions(l.frame.defaultTimeout())
	if err := copts.Parse(l.ctx, opts); err != nil {
		k6ext.Panic(l.ctx, "%w", err)
	}
	enabled, err := l.isEnabled(copts)
	if err != nil {
		k6ext.Panic(l.ctx, "%w", err)
	}

	return enabled
}

// isEnabled is like IsEnabled but takes parsed options and does not
// throw an error.
func (l *Locator) isEnabled(opts *FrameIsEnabledOptions) (bool, error) {
	opts.Strict = true
	return l.frame.isEnabled(l.selector, opts)
}

// IsDisabled returns true if the element matches the locator's
// selector and is disabled. Otherwise, returns false.
func (l *Locator) IsDisabled(opts goja.Value) bool {
	l.log.Debugf("Locator:IsDisabled", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	copts := NewFrameIsDisabledOptions(l.frame.defaultTimeout())
	if err := copts.Parse(l.ctx, opts); err != nil {
		k6ext.Panic(l.ctx, "%w", err)
	}
	disabled, err := l.isDisabled(copts)
	if err != nil {
		k6ext.Panic(l.ctx, "%w", err)
	}

	return disabled
}

// IsDisabled is like IsDisabled but takes parsed options and does not
// throw an error.
func (l *Locator) isDisabled(opts *FrameIsDisabledOptions) (bool, error) {
	opts.Strict = true
	return l.frame.isDisabled(l.selector, opts)
}

// IsVisible returns true if the element matches the locator's
// selector and is visible. Otherwise, returns false.
func (l *Locator) IsVisible(opts goja.Value) bool {
	l.log.Debugf("Locator:IsVisible", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	copts := NewFrameIsVisibleOptions(l.frame.defaultTimeout())
	if err := copts.Parse(l.ctx, opts); err != nil {
		k6ext.Panic(l.ctx, "%w", err)
	}
	visible, err := l.isVisible(copts)
	if err != nil {
		k6ext.Panic(l.ctx, "%w", err)
	}

	return visible
}

// isVisible is like IsVisible but takes parsed options and does not
// throw an error.
func (l *Locator) isVisible(opts *FrameIsVisibleOptions) (bool, error) {
	opts.Strict = true
	return l.frame.isVisible(l.selector, opts)
}

// IsHidden returns true if the element matches the locator's
// selector and is hidden. Otherwise, returns false.
func (l *Locator) IsHidden(opts goja.Value) bool {
	l.log.Debugf("Locator:IsHidden", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	copts := NewFrameIsHiddenOptions(l.frame.defaultTimeout())
	if err := copts.Parse(l.ctx, opts); err != nil {
		k6ext.Panic(l.ctx, "%w", err)
	}
	hidden, err := l.isHidden(copts)
	if err != nil {
		k6ext.Panic(l.ctx, "%w", err)
	}

	return hidden
}

// isHidden is like IsHidden but takes parsed options and does not
// throw an error.
func (l *Locator) isHidden(opts *FrameIsHiddenOptions) (bool, error) {
	opts.Strict = true
	return l.frame.isHidden(l.selector, opts)
}

// Fill out the element using locator's selector with strict mode on.
func (l *Locator) Fill(value string, opts goja.Value) {
	l.log.Debugf(
		"Locator:Fill", "fid:%s furl:%q sel:%q val:%q opts:%+v",
		l.frame.ID(), l.frame.URL(), l.selector, value, opts,
	)

	var err error
	defer func() { panicOrSlowMo(l.ctx, err) }()

	copts := NewFrameFillOptions(l.frame.defaultTimeout())
	if err = copts.Parse(l.ctx, opts); err != nil {
		return
	}
	if err = l.fill(value, copts); err != nil {
		return
	}
}

func (l *Locator) fill(value string, opts *FrameFillOptions) error {
	opts.Strict = true
	return l.frame.fill(l.selector, value, opts)
}

// Focus on the element using locator's selector with strict mode on.
func (l *Locator) Focus(opts goja.Value) {
	l.log.Debugf("Locator:Focus", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	var err error
	defer func() { panicOrSlowMo(l.ctx, err) }()

	copts := NewFrameBaseOptions(l.frame.defaultTimeout())
	if err = copts.Parse(l.ctx, opts); err != nil {
		return
	}
	if err = l.focus(copts); err != nil {
		return
	}
}

func (l *Locator) focus(opts *FrameBaseOptions) error {
	opts.Strict = true
	return l.frame.focus(l.selector, opts)
}

// GetAttribute of the element using locator's selector with strict mode on.
func (l *Locator) GetAttribute(name string, opts goja.Value) goja.Value {
	l.log.Debugf(
		"Locator:GetAttribute", "fid:%s furl:%q sel:%q name:%q opts:%+v",
		l.frame.ID(), l.frame.URL(), l.selector, name, opts,
	)

	var err error
	defer func() { panicOrSlowMo(l.ctx, err) }()

	copts := NewFrameBaseOptions(l.frame.defaultTimeout())
	if err = copts.Parse(l.ctx, opts); err != nil {
		return nil
	}
	var v goja.Value
	if v, err = l.getAttribute(name, copts); err != nil {
		return nil
	}

	return v
}

func (l *Locator) getAttribute(name string, opts *FrameBaseOptions) (goja.Value, error) {
	opts.Strict = true
	return l.frame.getAttribute(l.selector, name, opts)
}

// InnerHTML returns the element's inner HTML that matches
// the locator's selector with strict mode on.
func (l *Locator) InnerHTML(opts goja.Value) string {
	l.log.Debugf("Locator:InnerHTML", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	var err error
	defer func() { panicOrSlowMo(l.ctx, err) }()

	copts := NewFrameInnerHTMLOptions(l.frame.defaultTimeout())
	if err = copts.Parse(l.ctx, opts); err != nil {
		return ""
	}
	var s string
	if s, err = l.innerHTML(copts); err != nil {
		return ""
	}

	return s
}

func (l *Locator) innerHTML(opts *FrameInnerHTMLOptions) (string, error) {
	opts.Strict = true
	return l.frame.innerHTML(l.selector, opts)
}

// InnerText returns the element's inner text that matches
// the locator's selector with strict mode on.
func (l *Locator) InnerText(opts goja.Value) string {
	l.log.Debugf("Locator:InnerText", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	var err error
	defer func() { panicOrSlowMo(l.ctx, err) }()

	copts := NewFrameInnerTextOptions(l.frame.defaultTimeout())
	if err = copts.Parse(l.ctx, opts); err != nil {
		return ""
	}
	var s string
	if s, err = l.innerText(copts); err != nil {
		return ""
	}

	return s
}

func (l *Locator) innerText(opts *FrameInnerTextOptions) (string, error) {
	opts.Strict = true
	return l.frame.innerText(l.selector, opts)
}

// TextContent returns the element's text content that matches
// the locator's selector with strict mode on.
func (l *Locator) TextContent(opts goja.Value) string {
	l.log.Debugf("Locator:TextContent", "fid:%s furl:%q sel:%q opts:%+v", l.frame.ID(), l.frame.URL(), l.selector, opts)

	var err error
	defer func() { panicOrSlowMo(l.ctx, err) }()

	copts := NewFrameTextContentOptions(l.frame.defaultTimeout())
	if err = copts.Parse(l.ctx, opts); err != nil {
		return ""
	}
	var s string
	if s, err = l.textContent(copts); err != nil {
		return ""
	}

	return s
}

func (l *Locator) textContent(opts *FrameTextContentOptions) (string, error) {
	opts.Strict = true
	return l.frame.textContent(l.selector, opts)
}
