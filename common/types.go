/*
 *
 * xk6-browser - a browser automation extension for k6
 * Copyright (C) 2021 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package common

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"

	"github.com/dop251/goja"
	k6common "go.k6.io/k6/js/common"

	"github.com/grafana/xk6-browser/api"
)

// ColorScheme represents a browser color scheme.
type ColorScheme string

// Valid color schemes.
const (
	ColorSchemeLight        ColorScheme = "light"
	ColorSchemeDark         ColorScheme = "dark"
	ColorSchemeNoPreference ColorScheme = "no-preference"
)

func (c ColorScheme) String() string {
	return colorSchemeToString[c]
}

var colorSchemeToString = map[ColorScheme]string{
	ColorSchemeLight:        "light",
	ColorSchemeDark:         "dark",
	ColorSchemeNoPreference: "no-preference",
}

var colorSchemeToID = map[string]ColorScheme{
	"light":         ColorSchemeLight,
	"dark":          ColorSchemeDark,
	"no-preference": ColorSchemeNoPreference,
}

// MarshalJSON marshals the enum as a quoted JSON string.
func (c ColorScheme) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(colorSchemeToString[c])
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

// UnmarshalJSON unmarshals a quoted JSON string to the enum value.
func (c *ColorScheme) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}
	// Note that if the string cannot be found then it will be set to the zero value.
	*c = colorSchemeToID[j]
	return nil
}

// Credentials holds HTTP authentication credentials.
type Credentials struct {
	Username string `js:"username"`
	Password string `js:"password"`
}

// DOMElementState represents a DOM element state.
type DOMElementState int

// Valid DOM element states.
const (
	DOMElementStateAttached DOMElementState = iota
	DOMElementStateDetached
	DOMElementStateVisible
	DOMElementStateHidden
)

func (s DOMElementState) String() string {
	return domElementStateToString[s]
}

var domElementStateToString = map[DOMElementState]string{
	DOMElementStateAttached: "attached",
	DOMElementStateDetached: "detached",
	DOMElementStateVisible:  "visible",
	DOMElementStateHidden:   "hidden",
}

var domElementStateToID = map[string]DOMElementState{
	"attached": DOMElementStateAttached,
	"detached": DOMElementStateDetached,
	"visible":  DOMElementStateVisible,
	"hidden":   DOMElementStateHidden,
}

// MarshalJSON marshals the enum as a quoted JSON string.
func (s DOMElementState) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(domElementStateToString[s])
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

// UnmarshalJSON unmarshals a quoted JSON string to the enum value.
func (s *DOMElementState) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}
	// Note that if the string cannot be found then it will be set to the zero value.
	*s = domElementStateToID[j]
	return nil
}

type EmulatedSize struct {
	Viewport *Viewport
	Screen   *Screen
}

func NewEmulatedSize(viewport *Viewport, screen *Screen) *EmulatedSize {
	return &EmulatedSize{
		Viewport: viewport,
		Screen:   screen,
	}
}

type Geolocation struct {
	Latitude  float64 `js:"latitude"`
	Longitude float64 `js:"longitude"`
	Accurracy float64 `js:"accurracy"`
}

func NewGeolocation() *Geolocation {
	return &Geolocation{}
}

func (g *Geolocation) Parse(ctx context.Context, opts goja.Value) error {
	rt := k6common.GetRuntime(ctx)
	longitude := 0.0
	latitude := 0.0
	accuracy := 0.0

	if opts != nil && !goja.IsUndefined(opts) && !goja.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "accuracy":
				accuracy = opts.Get(k).ToFloat()
			case "latitude":
				latitude = opts.Get(k).ToFloat()
			case "longitude":
				longitude = opts.Get(k).ToFloat()
			}
		}
	}

	if longitude < -180 || longitude > 180 {
		return fmt.Errorf(`invalid longitude "%.2f": precondition -180 <= LONGITUDE <= 180 failed`, longitude)
	}
	if latitude < -90 || latitude > 90 {
		return fmt.Errorf(`invalid latitude "%.2f": precondition -90 <= LATITUDE <= 90 failed`, latitude)
	}
	if accuracy < 0 {
		return fmt.Errorf(`invalid accuracy "%.2f": precondition 0 <= ACCURACY failed`, accuracy)
	}

	g.Accurracy = accuracy
	g.Latitude = latitude
	g.Longitude = longitude
	return nil
}

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

type MediaType string

const (
	MediaTypeScreen MediaType = "screen"
	MediaTypePrint  MediaType = "print"
)

type PollingType int

const (
	PollingRaf PollingType = iota
	PollingMutation
	PollingInterval
)

func (p PollingType) String() string {
	return pollingTypeToString[p]
}

var pollingTypeToString = map[PollingType]string{
	PollingRaf:      "raf",
	PollingMutation: "mutation",
	PollingInterval: "interval",
}

var pollingTypeToID = map[string]PollingType{
	"raf":      PollingRaf,
	"mutation": PollingMutation,
	"interval": PollingInterval,
}

// MarshalJSON marshals the enum as a quoted JSON string.
func (p PollingType) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(pollingTypeToString[p])
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

// UnmarshalJSON unmarshals a quoted JSON string to the enum value.
func (p *PollingType) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}
	// Note that if the string cannot be found then it will be set to the zero value.
	*p = pollingTypeToID[j]
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

func (r *Rect) toApiRect() *api.Rect {
	return &api.Rect{X: r.X, Y: r.Y, Width: r.Width, Height: r.Height}
}

// ReducedMotion represents a browser reduce-motion setting.
type ReducedMotion string

// Valid reduce-motion options.
const (
	ReducedMotionReduce       ReducedMotion = "reduce"
	ReducedMotionNoPreference ReducedMotion = "no-preference"
)

func (r ReducedMotion) String() string {
	return reducedMotionToString[r]
}

var reducedMotionToString = map[ReducedMotion]string{
	ReducedMotionReduce:       "reduce",
	ReducedMotionNoPreference: "no-preference",
}

var reducedMotionToID = map[string]ReducedMotion{
	"reduce":        ReducedMotionReduce,
	"no-preference": ReducedMotionNoPreference,
}

// MarshalJSON marshals the enum as a quoted JSON string.
func (r ReducedMotion) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(reducedMotionToString[r])
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

// UnmarshalJSON unmarshals a quoted JSON string to the enum value.
func (r *ReducedMotion) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}
	// Note that if the string cannot be found then it will be set to the zero value.
	*r = reducedMotionToID[j]
	return nil
}

type ResourceTiming struct {
	StartTime             float64 `js:"startTime"`
	DomainLookupStart     float64 `js:"domainLookupStart"`
	DomainLookupEnd       float64 `js:"domainLookupEnd"`
	ConnectStart          float64 `js:"connectStart"`
	SecureConnectionStart float64 `js:"secureConnectionStart"`
	ConnectEnd            float64 `js:"connectEnd"`
	RequestStart          float64 `js:"requestStart"`
	ResponseStart         float64 `js:"responseStart"`
	ResponseEnd           float64 `js:"responseEnd"`
}

// Viewport represents a device screen.
type Screen struct {
	Width  int64 `js:"width"`
	Height int64 `js:"height"`
}

func (s *Screen) Parse(ctx context.Context, screen goja.Value) error {
	rt := k6common.GetRuntime(ctx)
	if screen != nil && !goja.IsUndefined(screen) && !goja.IsNull(screen) {
		screen := screen.ToObject(rt)
		for _, k := range screen.Keys() {
			switch k {
			case "width":
				s.Width = screen.Get(k).ToInteger()
			case "height":
				s.Height = screen.Get(k).ToInteger()
			}
		}
	}
	return nil
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
	rt := k6common.GetRuntime(ctx)
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

// Viewport represents a page viewport.
type Viewport struct {
	Width  int64 `js:"width"`
	Height int64 `js:"height"`
}

// Parse viewport details from a given goja viewport value.
func (v *Viewport) Parse(ctx context.Context, viewport goja.Value) error {
	rt := k6common.GetRuntime(ctx)
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

func NewCredentials() *Credentials {
	return &Credentials{}
}

func (c *Credentials) Parse(ctx context.Context, credentials goja.Value) error {
	rt := k6common.GetRuntime(ctx)
	if credentials != nil && !goja.IsUndefined(credentials) && !goja.IsNull(credentials) {
		credentials := credentials.ToObject(rt)
		for _, k := range credentials.Keys() {
			switch k {
			case "username":
				c.Username = credentials.Get(k).String()
			case "password":
				c.Password = credentials.Get(k).String()
			}
		}
	}
	return nil
}
