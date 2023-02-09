package common

import (
	"context"
	"time"

	"github.com/grafana/xk6-browser/api"
	"github.com/grafana/xk6-browser/k6ext"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/input"
	"github.com/dop251/goja"
)

// Ensure Mouse implements the api.Mouse interface.
var _ api.Mouse = &Mouse{}

// Mouse represents a mouse input device.
type Mouse struct {
	ctx             context.Context
	session         session
	frame           *Frame
	timeoutSettings *TimeoutSettings
	keyboard        *Keyboard
	x               float64
	y               float64
	button          input.MouseButton
}

// NewMouse creates a new mouse.
func NewMouse(ctx context.Context, s session, f *Frame, ts *TimeoutSettings, k *Keyboard) *Mouse {
	return &Mouse{
		ctx:             ctx,
		session:         s,
		frame:           f,
		timeoutSettings: ts,
		keyboard:        k,
		button:          input.None,
	}
}

func (m *Mouse) click(x float64, y float64, opts *MouseClickOptions) error {
	mouseDownUpOpts := opts.ToMouseDownUpOptions()
	if err := m.move(x, y, NewMouseMoveOptions()); err != nil {
		return err
	}
	if err := m.down(x, y, mouseDownUpOpts); err != nil {
		return err
	}
	if opts.Delay != 0 {
		t := time.NewTimer(time.Duration(opts.Delay) * time.Millisecond)
		select {
		case <-m.ctx.Done():
			t.Stop()
		case <-t.C:
		}
	}
	if err := m.up(x, y, mouseDownUpOpts); err != nil {
		return err
	}
	return nil
}

func (m *Mouse) dblClick(x float64, y float64, opts *MouseDblClickOptions) error {
	mouseDownUpOpts := opts.ToMouseDownUpOptions()
	if err := m.move(x, y, NewMouseMoveOptions()); err != nil {
		return err
	}
	for i := 0; i < 2; i++ {
		if err := m.down(x, y, mouseDownUpOpts); err != nil {
			return err
		}
		if opts.Delay != 0 {
			t := time.NewTimer(time.Duration(opts.Delay) * time.Millisecond)
			select {
			case <-m.ctx.Done():
				t.Stop()
			case <-t.C:
			}
		}
		if err := m.up(x, y, mouseDownUpOpts); err != nil {
			return err
		}
	}
	return nil
}

func (m *Mouse) down(x float64, y float64, opts *MouseDownUpOptions) error {
	m.button = input.MouseButton(opts.Button)
	action := input.DispatchMouseEvent(input.MousePressed, m.x, m.y).
		WithButton(input.MouseButton(opts.Button)).
		WithModifiers(input.Modifier(m.keyboard.modifiers)).
		WithClickCount(opts.ClickCount)
	if err := action.Do(cdp.WithExecutor(m.ctx, m.session)); err != nil {
		return err
	}
	return nil
}

func (m *Mouse) move(x float64, y float64, opts *MouseMoveOptions) error {
	var fromX float64 = m.x
	var fromY float64 = m.y
	m.x = x
	m.y = y
	for i := int64(1); i <= opts.Steps; i++ {
		x := fromX + (m.x-fromX)*float64(i/opts.Steps)
		y := fromY + (m.y-fromY)*float64(i/opts.Steps)
		action := input.DispatchMouseEvent(input.MouseMoved, x, y).
			WithButton(m.button).
			WithModifiers(input.Modifier(m.keyboard.modifiers))
		if err := action.Do(cdp.WithExecutor(m.ctx, m.session)); err != nil {
			return err
		}
	}
	return nil
}

func (m *Mouse) up(x float64, y float64, opts *MouseDownUpOptions) error {
	var button input.MouseButton = input.Left
	var clickCount int64 = 1
	m.button = input.None
	action := input.DispatchMouseEvent(input.MouseReleased, m.x, m.y).
		WithButton(button).
		WithModifiers(input.Modifier(m.keyboard.modifiers)).
		WithClickCount(clickCount)
	if err := action.Do(cdp.WithExecutor(m.ctx, m.session)); err != nil {
		return err
	}
	return nil
}

// Click will trigger a series of MouseMove, MouseDown and MouseUp events in the browser.
func (m *Mouse) Click(x float64, y float64, opts goja.Value) {
	mouseOpts := NewMouseClickOptions()
	if err := mouseOpts.Parse(m.ctx, opts); err != nil {
		k6ext.Panic(m.ctx, "parsing mouse click options: %w", err)
	}
	if err := m.click(x, y, mouseOpts); err != nil {
		k6ext.Panic(m.ctx, "clicking on x:%f y:%f: %w", x, y, err)
	}
}

func (m *Mouse) DblClick(x float64, y float64, opts goja.Value) {
	mouseOpts := NewMouseDblClickOptions()
	if err := mouseOpts.Parse(m.ctx, opts); err != nil {
		k6ext.Panic(m.ctx, "parsing double click options: %w", err)
	}
	if err := m.dblClick(x, y, mouseOpts); err != nil {
		k6ext.Panic(m.ctx, "double clicking on x:%f y:%f: %w", x, y, err)
	}
}

// Down will trigger a MouseDown event in the browser.
func (m *Mouse) Down(x float64, y float64, opts goja.Value) {
	mouseOpts := NewMouseDownUpOptions()
	if err := mouseOpts.Parse(m.ctx, opts); err != nil {
		k6ext.Panic(m.ctx, "parsing mouse down options: %w", err)
	}
	if err := m.down(x, y, mouseOpts); err != nil {
		k6ext.Panic(m.ctx, "pressing the mouse button on x:%f y:%f: %w", x, y, err)
	}
}

// Move will trigger a MouseMoved event in the browser.
func (m *Mouse) Move(x float64, y float64, opts goja.Value) {
	mouseOpts := NewMouseDownUpOptions()
	if err := mouseOpts.Parse(m.ctx, opts); err != nil {
		k6ext.Panic(m.ctx, "parsing mouse move options: %w", err)
	}
	if err := m.down(x, y, mouseOpts); err != nil {
		k6ext.Panic(m.ctx, "moving the mouse pointer to x:%f y:%f: %w", x, y, err)
	}
}

// Up will trigger a MouseUp event in the browser.
func (m *Mouse) Up(x float64, y float64, opts goja.Value) {
	mouseOpts := NewMouseDownUpOptions()
	if err := mouseOpts.Parse(m.ctx, opts); err != nil {
		k6ext.Panic(m.ctx, "parsing mouse up options: %w", err)
	}
	if err := m.up(x, y, mouseOpts); err != nil {
		k6ext.Panic(m.ctx, "releasing the mouse button on x:%f y:%f: %w", x, y, err)
	}
}

// Wheel will trigger a MouseWheel event in the browser
/*func (m *Mouse) Wheel(opts goja.Value) {
	var deltaX float64 = 0.0
	var deltaY float64 = 0.0

	if opts != nil && !goja.IsUndefined(opts) && !goja.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "deltaX":
				deltaX = opts.Get(k).ToFloat()
			case "deltaY":
				deltaY = opts.Get(k).ToFloat()
			}
		}
	}

	action := input.DispatchMouseEvent(input.MouseWheel, m.x, m.y).
		WithModifiers(input.Modifier(m.keyboard.modifiers)).
		WithDeltaX(deltaX).
		WithDeltaY(deltaY)
	if err := action.Do(cdp.WithExecutor(m.ctx, m.session)); err != nil {
		k6Throw(m.ctx, "mouse down: %w", err)
	}
}*/
