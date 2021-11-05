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

	"github.com/dop251/goja"
	k6common "go.k6.io/k6/js/common"
)

// ColorScheme represents a browser color scheme
type ColorScheme string

// Valid color schemes
const (
	ColorSchemeLight        ColorScheme = "light"
	ColorSchemeDark         ColorScheme = "dark"
	ColorSchemeNoPreference ColorScheme = "no-preference"
)

func (c ColorScheme) String() string {
	return ColorSchemeToString[c]
}

var ColorSchemeToString = map[ColorScheme]string{
	ColorSchemeLight:        "light",
	ColorSchemeDark:         "dark",
	ColorSchemeNoPreference: "no-preference",
}

var ColorSchemeToID = map[string]ColorScheme{
	"light":         ColorSchemeLight,
	"dark":          ColorSchemeDark,
	"no-preference": ColorSchemeNoPreference,
}

// MarshalJSON marshals the enum as a quoted JSON string
func (c ColorScheme) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(ColorSchemeToString[c])
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

// UnmarshalJSON unmashals a quoted JSON string to the enum value
func (c *ColorScheme) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}
	// Note that if the string cannot be found then it will be set to the zero value.
	*c = ColorSchemeToID[j]
	return nil
}

// Credentials holds HTTP authentication credentials
type Credentials struct {
	Username string `js:"username"`
	Password string `js:"password"`
}

// DOMElementState represents a DOM element state
type DOMElementState int

// Valid DOM element states
const (
	DOMElementStateAttached DOMElementState = iota
	DOMElementStateDetached
	DOMElementStateVisible
	DOMElementStateHidden
)

func (s DOMElementState) String() string {
	return DOMElementStateToString[s]
}

var DOMElementStateToString = map[DOMElementState]string{
	DOMElementStateAttached: "attached",
	DOMElementStateDetached: "detached",
	DOMElementStateVisible:  "visible",
	DOMElementStateHidden:   "hidden",
}

var DOMElementStateToID = map[string]DOMElementState{
	"attached": DOMElementStateAttached,
	"detached": DOMElementStateDetached,
	"visible":  DOMElementStateVisible,
	"hidden":   DOMElementStateHidden,
}

// MarshalJSON marshals the enum as a quoted JSON string
func (s DOMElementState) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(DOMElementStateToString[s])
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

// UnmarshalJSON unmashals a quoted JSON string to the enum value
func (s *DOMElementState) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}
	// Note that if the string cannot be found then it will be set to the zero value.
	*s = DOMElementStateToID[j]
	return nil
}

type EmulatedSize struct {
	Viewport *Viewport
	Screen   *Screen
}

type Geolocation struct {
	Latitude  float64 `js:"latitude"`
	Longitude float64 `js:"longitude"`
	Accurracy float64 `js:"accurracy"`
}

type LifecycleEvent int

const (
	LifecycleEventLoad LifecycleEvent = iota
	LifecycleEventDOMContentLoad
	LifecycleEventNetworkIdle
)

func (l LifecycleEvent) String() string {
	return LifecycleEventToString[l]
}

var LifecycleEventToString = map[LifecycleEvent]string{
	LifecycleEventLoad:           "load",
	LifecycleEventDOMContentLoad: "domcontentloaded",
	LifecycleEventNetworkIdle:    "networkidle",
}

var LifecycleEventToID = map[string]LifecycleEvent{
	"load":             LifecycleEventLoad,
	"domcontentloaded": LifecycleEventDOMContentLoad,
	"networkidle":      LifecycleEventNetworkIdle,
}

// MarshalJSON marshals the enum as a quoted JSON string
func (l LifecycleEvent) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(LifecycleEventToString[l])
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

// UnmarshalJSON unmarshals a quoted JSON string to the enum value
func (l *LifecycleEvent) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}
	// Note that if the string cannot be found then it will be set to the zero value.
	*l = LifecycleEventToID[j]
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
	return PollingTypeToString[p]
}

var PollingTypeToString = map[PollingType]string{
	PollingRaf:      "raf",
	PollingMutation: "mutation",
	PollingInterval: "interval",
}

var PollingTypeToID = map[string]PollingType{
	"raf":      PollingRaf,
	"mutation": PollingMutation,
	"interval": PollingInterval,
}

// MarshalJSON marshals the enum as a quoted JSON string
func (p PollingType) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(PollingTypeToString[p])
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

// UnmarshalJSON unmashals a quoted JSON string to the enum value
func (p *PollingType) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}
	// Note that if the string cannot be found then it will be set to the zero value.
	*p = PollingTypeToID[j]
	return nil
}

type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// ReducedMotion represents a browser reduce-motion setting
type ReducedMotion string

// Valid reduce-motion options
const (
	ReducedMotionReduce       ReducedMotion = "reduce"
	ReducedMotionNoPreference ReducedMotion = "no-preference"
)

func (r ReducedMotion) String() string {
	return ReducedMotionToString[r]
}

var ReducedMotionToString = map[ReducedMotion]string{
	ReducedMotionReduce:       "reduce",
	ReducedMotionNoPreference: "no-preference",
}

var ReducedMotionToID = map[string]ReducedMotion{
	"reduce":        ReducedMotionReduce,
	"no-preference": ReducedMotionNoPreference,
}

// MarshalJSON marshals the enum as a quoted JSON string
func (r ReducedMotion) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(ReducedMotionToString[r])
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

// UnmarshalJSON unmashals a quoted JSON string to the enum value
func (r *ReducedMotion) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}
	// Note that if the string cannot be found then it will be set to the zero value.
	*r = ReducedMotionToID[j]
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

// Viewport represents a device screen
type Screen struct {
	Width  int64 `js:"width"`
	Height int64 `js:"height"`
}

type SelectOption struct {
	Value *string `json:"value"`
	Label *string `json:"label"`
	Index *int64  `json:"index"`
}

// Viewport represents a page viewport
type Viewport struct {
	Width  int64 `js:"width"`
	Height int64 `js:"height"`
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

func NewEmulatedSize(viewport *Viewport, screen *Screen) *EmulatedSize {
	return &EmulatedSize{
		Viewport: viewport,
		Screen:   screen,
	}
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

func NewScreen() *Screen {
	return &Screen{}
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

func NewViewport() *Viewport {
	return &Viewport{}
}

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
