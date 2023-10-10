package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
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
