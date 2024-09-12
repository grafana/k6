package common

import (
	"context"
	"fmt"
	"math"

	"github.com/grafana/sobek"

	"github.com/grafana/xk6-browser/k6ext"
)

// Position represents a position.
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Rect represents a rectangle.
type Rect struct {
	X      float64 `js:"x"`
	Y      float64 `js:"y"`
	Width  float64 `js:"width"`
	Height float64 `js:"height"`
}

func (r *Rect) enclosingIntRect() *Rect {
	x := math.Floor(r.X + 1e-3)
	y := math.Floor(r.Y + 1e-3)
	x2 := math.Ceil(r.X + r.Width - 1e-3)
	y2 := math.Ceil(r.Y + r.Height - 1e-3)
	return &Rect{X: x, Y: y, Width: x2 - x, Height: y2 - y}
}

// SelectOption represents a select option.
type SelectOption struct {
	Value *string `json:"value"`
	Label *string `json:"label"`
	Index *int64  `json:"index"`
}

// Size represents a size.
type Size struct {
	Width  float64 `js:"width"`
	Height float64 `js:"height"`
}

func (s Size) enclosingIntSize() *Size {
	return &Size{
		Width:  math.Floor(s.Width + 1e-3),
		Height: math.Floor(s.Height + 1e-3),
	}
}

// Parse size details from a given sobek viewport value.
func (s *Size) Parse(ctx context.Context, viewport sobek.Value) error {
	rt := k6ext.Runtime(ctx)
	if viewport != nil && !sobek.IsUndefined(viewport) && !sobek.IsNull(viewport) {
		viewport := viewport.ToObject(rt)
		for _, k := range viewport.Keys() {
			switch k {
			case "width":
				s.Width = viewport.Get(k).ToFloat()
			case "height":
				s.Height = viewport.Get(k).ToFloat()
			}
		}
	}

	return nil
}

func (s Size) String() string {
	return fmt.Sprintf("%fx%f", s.Width, s.Height)
}

// Viewport represents a page viewport.
type Viewport struct {
	Width  int64 `js:"width"`
	Height int64 `js:"height"`
}

// Parse viewport details from a given sobek viewport value.
func (v *Viewport) Parse(ctx context.Context, viewport sobek.Value) error {
	rt := k6ext.Runtime(ctx)
	if viewport != nil && !sobek.IsUndefined(viewport) && !sobek.IsNull(viewport) {
		viewport := viewport.ToObject(rt)
		for _, k := range viewport.Keys() {
			switch k {
			case "width":
				v.Width = viewport.Get(k).ToInteger()
			case "height":
				v.Height = viewport.Get(k).ToInteger()
			}
		}
	}

	return nil
}

func (v Viewport) String() string {
	return fmt.Sprintf("%dx%d", v.Width, v.Height)
}

// calculateInset depending on a given operating system and,
// add the calculated inset width and height to Viewport.
// It won't update the Viewport if headless is true.
func (v *Viewport) calculateInset(headless bool, os string) {
	if headless {
		return
	}
	// TODO: popup windows have their own insets.
	var inset Viewport
	switch os {
	default:
		inset = Viewport{Width: 24, Height: 88}
	case "windows":
		inset = Viewport{Width: 16, Height: 88}
	case "linux":
		inset = Viewport{Width: 8, Height: 85}
	case "darwin":
		// Playwright is using w:2 h:80 here but I checked it
		// on my Mac and w:0 h:79 works best.
		inset = Viewport{Width: 0, Height: 79}
	}
	v.Width += inset.Width
	v.Height += inset.Height
}
