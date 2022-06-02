package common

import (
	"context"

	"github.com/dop251/goja"

	"github.com/grafana/xk6-browser/log"
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
