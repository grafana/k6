package common

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/grafana/sobek"

	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext"
	"go.k6.io/k6/js/common"
)

type ElementHandleBaseOptions struct {
	Force       bool          `json:"force"`
	NoWaitAfter bool          `json:"noWaitAfter"`
	Timeout     time.Duration `json:"timeout"`
}

type ElementHandleBasePointerOptions struct {
	ElementHandleBaseOptions
	Position *Position `json:"position"`
	Trial    bool      `json:"trial"`

	// We don't want to export this field. Internal use only to determine if we
	// should retry when using locator based APIs.
	retry bool
}

// ScrollPosition is a parameter for scrolling an element.
type ScrollPosition string

const (
	// ScrollPositionStart scrolls an element at the top of its parent.
	ScrollPositionStart ScrollPosition = "start"
	// ScrollPositionCenter scrolls an element at the center of its parent.
	ScrollPositionCenter ScrollPosition = "center"
	// ScrollPositionEnd scrolls an element at the end of its parent.
	ScrollPositionEnd ScrollPosition = "end"
	// ScrollPositionNearest scrolls an element at the nearest position of its parent.
	ScrollPositionNearest ScrollPosition = "nearest"
)

// ScrollIntoViewOptions change the behavior of ScrollIntoView.
// See: https://developer.mozilla.org/en-US/docs/Web/API/Element/scrollIntoView
type ScrollIntoViewOptions struct {
	// Block defines vertical alignment.
	// One of start, center, end, or nearest.
	// Defaults to start.
	Block ScrollPosition `json:"block"`

	// Inline defines horizontal alignment.
	// One of start, center, end, or nearest.
	// Defaults to nearest.
	Inline ScrollPosition `json:"inline"`
}

type ElementHandleCheckOptions struct {
	ElementHandleBasePointerOptions
}

type ElementHandleClickOptions struct {
	ElementHandleBasePointerOptions
	Button     string   `json:"button"`
	ClickCount int64    `json:"clickCount"`
	Delay      int64    `json:"delay"`
	Modifiers  []string `json:"modifiers"`
}

type ElementHandleDblclickOptions struct {
	ElementHandleBasePointerOptions
	Button    string   `json:"button"`
	Delay     int64    `json:"delay"`
	Modifiers []string `json:"modifiers"`
}

type ElementHandleHoverOptions struct {
	ElementHandleBasePointerOptions
	Modifiers []string `json:"modifiers"`
}

// File is the descriptor of a single file.
type File struct {
	Name     string `json:"name"`
	Mimetype string `json:"mimeType"`
	Buffer   string `json:"buffer"`
}

// Files is the input parameter for ElementHandle.SetInputFiles.
type Files struct {
	Payload []*File `json:"payload"`
}

// ElementHandleSetInputFilesOptions are options for ElementHandle.SetInputFiles.
type ElementHandleSetInputFilesOptions struct {
	ElementHandleBaseOptions
}

type ElementHandlePressOptions struct {
	Delay       int64         `json:"delay"`
	NoWaitAfter bool          `json:"noWaitAfter"`
	Timeout     time.Duration `json:"timeout"`
}

type ElementHandleScreenshotOptions struct {
	Path           string        `json:"path"`
	Format         ImageFormat   `json:"format"`
	OmitBackground bool          `json:"omitBackground"`
	Quality        int64         `json:"quality"`
	Timeout        time.Duration `json:"timeout"`
}

type ElementHandleSetCheckedOptions struct {
	ElementHandleBasePointerOptions
	Strict bool `json:"strict"`
}

type ElementHandleTapOptions struct {
	ElementHandleBasePointerOptions
	Modifiers []string `json:"modifiers"`
}

type ElementHandleTypeOptions struct {
	Delay       int64         `json:"delay"`
	NoWaitAfter bool          `json:"noWaitAfter"`
	Timeout     time.Duration `json:"timeout"`
}

type ElementHandleWaitForElementStateOptions struct {
	Timeout time.Duration `json:"timeout"`
}

func NewElementHandleBaseOptions(defaultTimeout time.Duration) *ElementHandleBaseOptions {
	return &ElementHandleBaseOptions{
		Force:       false,
		NoWaitAfter: false,
		Timeout:     defaultTimeout,
	}
}

func NewElementHandleBasePointerOptions(defaultTimeout time.Duration) *ElementHandleBasePointerOptions {
	return &ElementHandleBasePointerOptions{
		ElementHandleBaseOptions: *NewElementHandleBaseOptions(defaultTimeout),
		Position:                 nil,
		Trial:                    false,
	}
}

func NewElementHandleCheckOptions(defaultTimeout time.Duration) *ElementHandleCheckOptions {
	return &ElementHandleCheckOptions{
		ElementHandleBasePointerOptions: *NewElementHandleBasePointerOptions(defaultTimeout),
	}
}

// NewElementHandleSetInputFilesOptions creates a new ElementHandleSetInputFilesOption.
func NewElementHandleSetInputFilesOptions(defaultTimeout time.Duration) *ElementHandleSetInputFilesOptions {
	return &ElementHandleSetInputFilesOptions{
		ElementHandleBaseOptions: *NewElementHandleBaseOptions(defaultTimeout),
	}
}

// addFile to the struct. Input value can only be a file descriptor object.
func (f *Files) addFile(ctx context.Context, file sobek.Value) error {
	if common.IsNullish(file) {
		return nil
	}
	rt := k6ext.Runtime(ctx)
	fileType := file.ExportType()
	switch fileType.Kind() {
	case reflect.Map: // file descriptor object
		var parsedFile File
		if err := rt.ExportTo(file, &parsedFile); err != nil {
			return fmt.Errorf("parsing file descriptor: %w", err)
		}
		f.Payload = append(f.Payload, &parsedFile)
	default:
		return fmt.Errorf("invalid parameter type : %s", fileType.Kind().String())
	}

	return nil
}

// Parse parses the Files struct from the given sobek.Value.
func (f *Files) Parse(ctx context.Context, files sobek.Value) error {
	rt := k6ext.Runtime(ctx)
	if common.IsNullish(files) {
		return nil
	}

	optsType := files.ExportType()
	switch optsType.Kind() {
	case reflect.Slice: // array of filePaths or array of file descriptor objects
		gopts := files.ToObject(rt)
		for _, k := range gopts.Keys() {
			err := f.addFile(ctx, gopts.Get(k))
			if err != nil {
				return err
			}
		}
	default: // filePath or file descriptor object
		return f.addFile(ctx, files)
	}

	return nil
}

func NewElementHandleClickOptions(defaultTimeout time.Duration) *ElementHandleClickOptions {
	return &ElementHandleClickOptions{
		ElementHandleBasePointerOptions: *NewElementHandleBasePointerOptions(defaultTimeout),
		Button:                          "left",
		ClickCount:                      1,
		Delay:                           0,
		Modifiers:                       []string{},
	}
}

func (o *ElementHandleClickOptions) ToMouseClickOptions() *MouseClickOptions {
	o2 := NewMouseClickOptions()
	o2.Button = o.Button
	o2.ClickCount = o.ClickCount
	o2.Delay = o.Delay
	return o2
}

func NewElementHandleDblclickOptions(defaultTimeout time.Duration) *ElementHandleDblclickOptions {
	return &ElementHandleDblclickOptions{
		ElementHandleBasePointerOptions: *NewElementHandleBasePointerOptions(defaultTimeout),
		Button:                          "left",
		Delay:                           0,
		Modifiers:                       []string{},
	}
}

func (o *ElementHandleDblclickOptions) ToMouseClickOptions() *MouseClickOptions {
	o2 := NewMouseClickOptions()
	o2.Button = o.Button
	o2.ClickCount = 2
	o2.Delay = o.Delay
	return o2
}

func NewElementHandleHoverOptions(defaultTimeout time.Duration) *ElementHandleHoverOptions {
	return &ElementHandleHoverOptions{
		ElementHandleBasePointerOptions: *NewElementHandleBasePointerOptions(defaultTimeout),
		Modifiers:                       []string{},
	}
}

func NewElementHandlePressOptions(defaultTimeout time.Duration) *ElementHandlePressOptions {
	return &ElementHandlePressOptions{
		Delay:       0,
		NoWaitAfter: false,
		Timeout:     defaultTimeout,
	}
}

func (o *ElementHandlePressOptions) ToBaseOptions() *ElementHandleBaseOptions {
	o2 := ElementHandleBaseOptions{}
	o2.Force = false
	o2.NoWaitAfter = o.NoWaitAfter
	o2.Timeout = o.Timeout
	return &o2
}

func NewElementHandleScreenshotOptions(defaultTimeout time.Duration) *ElementHandleScreenshotOptions {
	return &ElementHandleScreenshotOptions{
		Path:           "",
		Format:         ImageFormatPNG,
		OmitBackground: false,
		Quality:        100,
		Timeout:        defaultTimeout,
	}
}

func NewElementHandleSetCheckedOptions(defaultTimeout time.Duration) *ElementHandleSetCheckedOptions {
	return &ElementHandleSetCheckedOptions{
		ElementHandleBasePointerOptions: *NewElementHandleBasePointerOptions(defaultTimeout),
		Strict:                          false,
	}
}

func NewElementHandleTapOptions(defaultTimeout time.Duration) *ElementHandleTapOptions {
	return &ElementHandleTapOptions{
		ElementHandleBasePointerOptions: *NewElementHandleBasePointerOptions(defaultTimeout),
		Modifiers:                       []string{},
	}
}

func NewElementHandleTypeOptions(defaultTimeout time.Duration) *ElementHandleTypeOptions {
	return &ElementHandleTypeOptions{
		Delay:       0,
		NoWaitAfter: false,
		Timeout:     defaultTimeout,
	}
}

func (o *ElementHandleTypeOptions) ToBaseOptions() *ElementHandleBaseOptions {
	o2 := ElementHandleBaseOptions{}
	o2.Force = false
	o2.NoWaitAfter = o.NoWaitAfter
	o2.Timeout = o.Timeout
	return &o2
}

func NewElementHandleWaitForElementStateOptions(defaultTimeout time.Duration) *ElementHandleWaitForElementStateOptions {
	return &ElementHandleWaitForElementStateOptions{
		Timeout: defaultTimeout,
	}
}

// ElementHandleDispatchEventOptions are options for ElementHandle.dispatchEvent.
type ElementHandleDispatchEventOptions struct {
	*ElementHandleBaseOptions
}

// NewElementHandleDispatchEventOptions returns a new ElementHandleDispatchEventOptions.
func NewElementHandleDispatchEventOptions(defaultTimeout time.Duration) *ElementHandleDispatchEventOptions {
	return &ElementHandleDispatchEventOptions{
		ElementHandleBaseOptions: NewElementHandleBaseOptions(defaultTimeout),
	}
}
