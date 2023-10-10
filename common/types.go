package common

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/grafana/xk6-browser/k6ext"

	"github.com/dop251/goja"
)

// ImageFormat represents an image file format.
type ImageFormat string

// Valid image format options.
const (
	ImageFormatJPEG ImageFormat = "jpeg"
	ImageFormatPNG  ImageFormat = "png"
)

func (f ImageFormat) String() string {
	return imageFormatToString[f]
}

var imageFormatToString = map[ImageFormat]string{
	ImageFormatJPEG: "jpeg",
	ImageFormatPNG:  "png",
}

var imageFormatToID = map[string]ImageFormat{
	"jpeg": ImageFormatJPEG,
	"png":  ImageFormatPNG,
}

// MarshalJSON marshals the enum as a quoted JSON string.
func (f ImageFormat) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(imageFormatToString[f])
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

// UnmarshalJSON unmarshals a quoted JSON string to the enum value.
func (f *ImageFormat) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}
	// Note that if the string cannot be found then it will be set to the zero value.
	*f = imageFormatToID[j]
	return nil
}

// FrameLifecycleEvent is emitted when a frame lifecycle event occurs.
type FrameLifecycleEvent struct {
	// URL is the URL of the frame that emitted the event.
	URL string

	// Event is the lifecycle event that occurred.
	Event LifecycleEvent
}

type LifecycleEvent int

const (
	LifecycleEventLoad LifecycleEvent = iota
	LifecycleEventDOMContentLoad
	LifecycleEventNetworkIdle
)

func (l LifecycleEvent) String() string {
	return lifecycleEventToString[l]
}

var lifecycleEventToString = map[LifecycleEvent]string{
	LifecycleEventLoad:           "load",
	LifecycleEventDOMContentLoad: "domcontentloaded",
	LifecycleEventNetworkIdle:    "networkidle",
}

var lifecycleEventToID = map[string]LifecycleEvent{
	"load":             LifecycleEventLoad,
	"domcontentloaded": LifecycleEventDOMContentLoad,
	"networkidle":      LifecycleEventNetworkIdle,
}

// MarshalJSON marshals the enum as a quoted JSON string.
func (l LifecycleEvent) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(lifecycleEventToString[l])
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

// UnmarshalJSON unmarshals a quoted JSON string to the enum value.
func (l *LifecycleEvent) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}
	// Note that if the string cannot be found then it will be set to the zero value.
	*l = lifecycleEventToID[j]
	return nil
}

// MarshalText returns the string representation of the enum value.
// It returns an error if the enum value is invalid.
func (l *LifecycleEvent) MarshalText() ([]byte, error) {
	if l == nil {
		return []byte(""), nil
	}
	var (
		ok bool
		s  string
	)
	if s, ok = lifecycleEventToString[*l]; !ok {
		return nil, fmt.Errorf("invalid lifecycle event: %v", int(*l))
	}

	return []byte(s), nil
}

// UnmarshalText unmarshals a text representation to the enum value.
// It returns an error if given a wrong value.
func (l *LifecycleEvent) UnmarshalText(text []byte) error {
	var (
		ok  bool
		val = string(text)
	)

	if *l, ok = lifecycleEventToID[val]; !ok {
		valid := make([]string, 0, len(lifecycleEventToID))
		for k := range lifecycleEventToID {
			valid = append(valid, k)
		}
		sort.Slice(valid, func(i, j int) bool {
			return lifecycleEventToID[valid[j]] > lifecycleEventToID[valid[i]]
		})
		return fmt.Errorf(
			"invalid lifecycle event: %q; must be one of: %s",
			val, strings.Join(valid, ", "))
	}

	return nil
}

type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

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

type SelectOption struct {
	Value *string `json:"value"`
	Label *string `json:"label"`
	Index *int64  `json:"index"`
}

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

func (s *Size) Parse(ctx context.Context, viewport goja.Value) error {
	rt := k6ext.Runtime(ctx)
	if viewport != nil && !goja.IsUndefined(viewport) && !goja.IsNull(viewport) {
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

// Parse viewport details from a given goja viewport value.
func (v *Viewport) Parse(ctx context.Context, viewport goja.Value) error {
	rt := k6ext.Runtime(ctx)
	if viewport != nil && !goja.IsUndefined(viewport) && !goja.IsNull(viewport) {
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
