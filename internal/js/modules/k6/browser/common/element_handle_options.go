package common

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/grafana/sobek"

	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext"
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

const (
	optionButton     = "button"
	optionDelay      = "delay"
	optionClickCount = "clickCount"
	optionModifiers  = "modifiers"
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

// Parse parses the ElementHandleBaseOptions from the given opts.
func (o *ElementHandleBaseOptions) Parse(ctx context.Context, opts sobek.Value) error {
	if !sobekValueExists(opts) {
		return nil
	}
	gopts := opts.ToObject(k6ext.Runtime(ctx))
	for _, k := range gopts.Keys() {
		switch k {
		case "force":
			o.Force = gopts.Get(k).ToBoolean()
		case "noWaitAfter":
			o.NoWaitAfter = gopts.Get(k).ToBoolean()
		case "timeout":
			o.Timeout = time.Duration(gopts.Get(k).ToInteger()) * time.Millisecond
		}
	}

	return nil
}

func NewElementHandleBasePointerOptions(defaultTimeout time.Duration) *ElementHandleBasePointerOptions {
	return &ElementHandleBasePointerOptions{
		ElementHandleBaseOptions: *NewElementHandleBaseOptions(defaultTimeout),
		Position:                 nil,
		Trial:                    false,
	}
}

// Parse parses the ElementHandleBasePointerOptions from the given opts.
func (o *ElementHandleBasePointerOptions) Parse(ctx context.Context, opts sobek.Value) error {
	rt := k6ext.Runtime(ctx)
	if err := o.ElementHandleBaseOptions.Parse(ctx, opts); err != nil {
		return err
	}
	if opts != nil && !sobek.IsUndefined(opts) && !sobek.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "position":
				var p map[string]float64
				o.Position = &Position{}
				if rt.ExportTo(opts.Get(k), &p) != nil {
					o.Position.X = p["x"]
					o.Position.Y = p["y"]
				}
			case "trial":
				o.Trial = opts.Get(k).ToBoolean()
			}
		}
	}
	return nil
}

func NewElementHandleCheckOptions(defaultTimeout time.Duration) *ElementHandleCheckOptions {
	return &ElementHandleCheckOptions{
		ElementHandleBasePointerOptions: *NewElementHandleBasePointerOptions(defaultTimeout),
	}
}

// Parse parses the ElementHandleCheckOptions from the given opts.
func (o *ElementHandleCheckOptions) Parse(ctx context.Context, opts sobek.Value) error {
	return o.ElementHandleBasePointerOptions.Parse(ctx, opts)
}

// NewElementHandleSetInputFilesOptions creates a new ElementHandleSetInputFilesOption.
func NewElementHandleSetInputFilesOptions(defaultTimeout time.Duration) *ElementHandleSetInputFilesOptions {
	return &ElementHandleSetInputFilesOptions{
		ElementHandleBaseOptions: *NewElementHandleBaseOptions(defaultTimeout),
	}
}

// addFile to the struct. Input value can only be a file descriptor object.
func (f *Files) addFile(ctx context.Context, file sobek.Value) error {
	if !sobekValueExists(file) {
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
	if !sobekValueExists(files) {
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

// Parse parses the ElementHandleSetInputFilesOption from the given opts.
func (o *ElementHandleSetInputFilesOptions) Parse(ctx context.Context, opts sobek.Value) error {
	if err := o.ElementHandleBaseOptions.Parse(ctx, opts); err != nil {
		return err
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

// Parse parses the ElementHandleClickOptions from the given opts.
func (o *ElementHandleClickOptions) Parse(ctx context.Context, opts sobek.Value) error {
	if err := o.ElementHandleBasePointerOptions.Parse(ctx, opts); err != nil {
		return err
	}

	if !sobekValueExists(opts) {
		return nil
	}

	rt := k6ext.Runtime(ctx)
	obj := opts.ToObject(rt)
	for _, k := range obj.Keys() {
		switch k {
		case optionButton:
			o.Button = obj.Get(k).String()
		case optionClickCount:
			o.ClickCount = obj.Get(k).ToInteger()
		case optionDelay:
			o.Delay = obj.Get(k).ToInteger()
		case optionModifiers:
			var m []string
			if err := rt.ExportTo(obj.Get(k), &m); err != nil {
				return fmt.Errorf("parsing element handle click option modifiers: %w", err)
			}
			o.Modifiers = m
		}
	}

	return nil
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

// Parse parses the ElementHandleDblclickOptions from the given opts.
func (o *ElementHandleDblclickOptions) Parse(ctx context.Context, opts sobek.Value) error {
	rt := k6ext.Runtime(ctx)
	if err := o.ElementHandleBasePointerOptions.Parse(ctx, opts); err != nil {
		return err
	}
	if opts != nil && !sobek.IsUndefined(opts) && !sobek.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "button":
				o.Button = opts.Get(k).String()
			case "delay":
				o.Delay = opts.Get(k).ToInteger()
			case "modifiers":
				var m []string
				if err := rt.ExportTo(opts.Get(k), &m); err != nil {
					return err
				}
				o.Modifiers = m
			}
		}
	}
	return nil
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

// Parse parses the ElementHandleHoverOptions from the given opts.
func (o *ElementHandleHoverOptions) Parse(ctx context.Context, opts sobek.Value) error {
	rt := k6ext.Runtime(ctx)
	if err := o.ElementHandleBasePointerOptions.Parse(ctx, opts); err != nil {
		return err
	}
	if opts != nil && !sobek.IsUndefined(opts) && !sobek.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			if k == "modifiers" {
				var m []string
				if err := rt.ExportTo(opts.Get(k), &m); err != nil {
					return err
				}
				o.Modifiers = m
			}
		}
	}
	return nil
}

func NewElementHandlePressOptions(defaultTimeout time.Duration) *ElementHandlePressOptions {
	return &ElementHandlePressOptions{
		Delay:       0,
		NoWaitAfter: false,
		Timeout:     defaultTimeout,
	}
}

// Parse parses the ElementHandlePressOptions from the given opts.
func (o *ElementHandlePressOptions) Parse(ctx context.Context, opts sobek.Value) error {
	rt := k6ext.Runtime(ctx)
	if opts != nil && !sobek.IsUndefined(opts) && !sobek.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "delay":
				o.Delay = opts.Get(k).ToInteger()
			case "noWaitAfter":
				o.NoWaitAfter = opts.Get(k).ToBoolean()
			case "timeout":
				o.Timeout = time.Duration(opts.Get(k).ToInteger()) * time.Millisecond
			}
		}
	}
	return nil
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

// Parse parses the ElementHandleScreenshotOptions from the given opts.
func (o *ElementHandleScreenshotOptions) Parse(ctx context.Context, opts sobek.Value) error {
	if !sobekValueExists(opts) {
		return nil
	}

	rt := k6ext.Runtime(ctx)
	formatSpecified := false
	obj := opts.ToObject(rt)
	for _, k := range obj.Keys() {
		switch k {
		case "omitBackground":
			o.OmitBackground = obj.Get(k).ToBoolean()
		case "path":
			o.Path = obj.Get(k).String()
		case "quality":
			o.Quality = obj.Get(k).ToInteger()
		case "type":
			if f, ok := imageFormatToID[obj.Get(k).String()]; ok {
				o.Format = f
				formatSpecified = true
			}
		case "timeout":
			o.Timeout = time.Duration(obj.Get(k).ToInteger()) * time.Millisecond
		}
	}

	// Infer file format by path if format not explicitly specified (default is PNG)
	if o.Path != "" && !formatSpecified {
		if strings.HasSuffix(o.Path, ".jpg") || strings.HasSuffix(o.Path, ".jpeg") {
			o.Format = ImageFormatJPEG
		}
	}

	return nil
}

func NewElementHandleSetCheckedOptions(defaultTimeout time.Duration) *ElementHandleSetCheckedOptions {
	return &ElementHandleSetCheckedOptions{
		ElementHandleBasePointerOptions: *NewElementHandleBasePointerOptions(defaultTimeout),
		Strict:                          false,
	}
}

// Parse parses the ElementHandleSetCheckedOptions from the given opts.
func (o *ElementHandleSetCheckedOptions) Parse(ctx context.Context, opts sobek.Value) error {
	rt := k6ext.Runtime(ctx)

	if err := o.ElementHandleBasePointerOptions.Parse(ctx, opts); err != nil {
		return err
	}

	if opts != nil && !sobek.IsUndefined(opts) && !sobek.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			if k == "strict" {
				o.Strict = opts.Get(k).ToBoolean()
			}
		}
	}
	return nil
}

func NewElementHandleTapOptions(defaultTimeout time.Duration) *ElementHandleTapOptions {
	return &ElementHandleTapOptions{
		ElementHandleBasePointerOptions: *NewElementHandleBasePointerOptions(defaultTimeout),
		Modifiers:                       []string{},
	}
}

// Parse parses the ElementHandleTapOptions from the given opts.
func (o *ElementHandleTapOptions) Parse(ctx context.Context, opts sobek.Value) error {
	rt := k6ext.Runtime(ctx)
	if err := o.ElementHandleBasePointerOptions.Parse(ctx, opts); err != nil {
		return err
	}
	if opts != nil && !sobek.IsUndefined(opts) && !sobek.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			if k == "modifiers" {
				var m []string
				if err := rt.ExportTo(opts.Get(k), &m); err != nil {
					return err
				}
				o.Modifiers = m
			}
		}
	}
	return nil
}

func NewElementHandleTypeOptions(defaultTimeout time.Duration) *ElementHandleTypeOptions {
	return &ElementHandleTypeOptions{
		Delay:       0,
		NoWaitAfter: false,
		Timeout:     defaultTimeout,
	}
}

// Parse parses the ElementHandleTypeOptions from the given opts.
func (o *ElementHandleTypeOptions) Parse(ctx context.Context, opts sobek.Value) error {
	rt := k6ext.Runtime(ctx)
	if opts != nil && !sobek.IsUndefined(opts) && !sobek.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "delay":
				o.Delay = opts.Get(k).ToInteger()
			case "noWaitAfter":
				o.NoWaitAfter = opts.Get(k).ToBoolean()
			case "timeout":
				o.Timeout = time.Duration(opts.Get(k).ToInteger()) * time.Millisecond
			}
		}
	}
	return nil
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

// Parse parses the ElementHandleWaitForElementStateOptions from the given opts.
func (o *ElementHandleWaitForElementStateOptions) Parse(ctx context.Context, opts sobek.Value) error {
	rt := k6ext.Runtime(ctx)
	if opts != nil && !sobek.IsUndefined(opts) && !sobek.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			if k == "timeout" {
				o.Timeout = time.Duration(opts.Get(k).ToInteger()) * time.Millisecond
			}
		}
	}
	return nil
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
