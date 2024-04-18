package common

import (
	"context"
	"fmt"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/input"
)

// Touchscreen represents a touchscreen.
type Touchscreen struct {
	BaseEventEmitter

	ctx      context.Context
	session  session
	keyboard *Keyboard
}

// NewTouchscreen returns a new TouchScreen.
func NewTouchscreen(ctx context.Context, s session, k *Keyboard) *Touchscreen {
	return &Touchscreen{
		ctx:      ctx,
		session:  s,
		keyboard: k,
	}
}

func (t *Touchscreen) tap(x float64, y float64) error {
	action := input.DispatchTouchEvent(input.TouchStart, []*input.TouchPoint{{X: x, Y: y}}).
		WithModifiers(input.Modifier(t.keyboard.modifiers))
	if err := action.Do(cdp.WithExecutor(t.ctx, t.session)); err != nil {
		return fmt.Errorf("touch start: %w", err)
	}

	action = input.DispatchTouchEvent(input.TouchEnd, []*input.TouchPoint{}).
		WithModifiers(input.Modifier(t.keyboard.modifiers))
	if err := action.Do(cdp.WithExecutor(t.ctx, t.session)); err != nil {
		return fmt.Errorf("touch end: %w", err)
	}

	return nil
}

// Tap dispatches a tap start and tap end event.
func (t *Touchscreen) Tap(x float64, y float64) error {
	if err := t.tap(x, y); err != nil {
		return fmt.Errorf("tapping: %w", err)
	}
	return nil
}
