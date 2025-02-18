package common

import (
	"context"
	"fmt"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/input"
)

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

// Click will trigger a series of MouseMove, MouseDown and MouseUp events in the browser.
func (m *Mouse) Click(x float64, y float64, opts *MouseClickOptions) error {
	if err := m.click(x, y, opts); err != nil {
		return fmt.Errorf("clicking on x:%f y:%f: %w", x, y, err)
	}
	return nil
}

func (m *Mouse) click(x float64, y float64, opts *MouseClickOptions) error {
	mouseDownUpOpts := opts.ToMouseDownUpOptions()
	if err := m.move(x, y, NewMouseMoveOptions()); err != nil {
		return err
	}
	for i := 0; i < int(mouseDownUpOpts.ClickCount); i++ {
		if err := m.down(mouseDownUpOpts); err != nil {
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
		if err := m.up(mouseDownUpOpts); err != nil {
			return err
		}
	}

	return nil
}

// DblClick will trigger Click twice in quick succession.
func (m *Mouse) DblClick(x float64, y float64, opts *MouseDblClickOptions) error {
	if err := m.click(x, y, opts.ToMouseClickOptions()); err != nil {
		return fmt.Errorf("double clicking on x:%f y:%f: %w", x, y, err)
	}
	return nil
}

// Down will trigger a MouseDown event in the browser.
func (m *Mouse) Down(opts *MouseDownUpOptions) error {
	if err := m.down(opts); err != nil {
		return fmt.Errorf("pressing the mouse button on x:%f y:%f: %w", m.x, m.y, err)
	}
	return nil
}

func (m *Mouse) down(opts *MouseDownUpOptions) error {
	m.button = input.MouseButton(opts.Button)
	action := input.DispatchMouseEvent(input.MousePressed, m.x, m.y).
		WithButton(input.MouseButton(opts.Button)).
		WithModifiers(input.Modifier(m.keyboard.modifiers)).
		WithClickCount(opts.ClickCount)
	if err := action.Do(cdp.WithExecutor(m.ctx, m.session)); err != nil {
		return fmt.Errorf("mouse down: %w", err)
	}
	return nil
}

// Up will trigger a MouseUp event in the browser.
func (m *Mouse) Up(opts *MouseDownUpOptions) error {
	if err := m.up(opts); err != nil {
		return fmt.Errorf("releasing the mouse button on x:%f y:%f: %w", m.x, m.y, err)
	}
	return nil
}

func (m *Mouse) up(opts *MouseDownUpOptions) error {
	m.button = input.None
	action := input.DispatchMouseEvent(input.MouseReleased, m.x, m.y).
		WithButton(input.MouseButton(opts.Button)).
		WithModifiers(input.Modifier(m.keyboard.modifiers)).
		WithClickCount(opts.ClickCount)
	if err := action.Do(cdp.WithExecutor(m.ctx, m.session)); err != nil {
		return fmt.Errorf("mouse up: %w", err)
	}

	return nil
}

// Move will trigger a MouseMoved event in the browser.
func (m *Mouse) Move(x float64, y float64, opts *MouseMoveOptions) error {
	if err := m.move(x, y, opts); err != nil {
		return fmt.Errorf("moving the mouse pointer to x:%f y:%f: %w", x, y, err)
	}
	return nil
}

func (m *Mouse) move(x float64, y float64, opts *MouseMoveOptions) error {
	fromX := m.x
	fromY := m.y
	m.x = x
	m.y = y
	for i := int64(1); i <= opts.Steps; i++ {
		x := fromX + (m.x-fromX)*float64(i/opts.Steps)
		y := fromY + (m.y-fromY)*float64(i/opts.Steps)
		action := input.DispatchMouseEvent(input.MouseMoved, x, y).
			WithButton(m.button).
			WithModifiers(input.Modifier(m.keyboard.modifiers))
		if err := action.Do(cdp.WithExecutor(m.ctx, m.session)); err != nil {
			return fmt.Errorf("mouse move: %w", err)
		}
	}

	return nil
}
