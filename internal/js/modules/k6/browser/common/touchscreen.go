package common

import (
	"context"
	"fmt"

	"github.com/chromedp/cdproto/input"
)

// Touchscreen represents a touchscreen.
type Touchscreen struct {
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

// Tap dispatches a tap start and tap end event.
func (t *Touchscreen) Tap(x float64, y float64) error {
	if err := t.tap(x, y); err != nil {
		return fmt.Errorf("tapping: %w", err)
	}
	return nil
}

func (t *Touchscreen) tap(x float64, y float64) error {
	touchStart := newCommand(input.DispatchTouchEvent, input.DispatchTouchEventParams{
		Type:        input.DispatchTouchEventTypeTouchStart,
		TouchPoints: []*input.TouchPoint{{X: x, Y: y}},
		Modifiers:   t.keyboard.modifiers,
	})

	if err := touchStart.Do(t.ctx, t.session); err != nil {
		return fmt.Errorf("touch start: %w", err)
	}

	touchEnd := newCommand(input.DispatchTouchEvent, input.DispatchTouchEventParams{
		Type:        input.DispatchTouchEventTypeTouchEnd,
		TouchPoints: []*input.TouchPoint{},
		Modifiers:   t.keyboard.modifiers,
	})

	if err := touchEnd.Do(t.ctx, t.session); err != nil {
		return fmt.Errorf("touch end: %w", err)
	}

	return nil
}
