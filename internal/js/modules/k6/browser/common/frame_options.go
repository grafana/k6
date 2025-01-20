package common

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/grafana/sobek"

	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext"
)

type FrameBaseOptions struct {
	Timeout time.Duration `json:"timeout"`
	Strict  bool          `json:"strict"`
}

type FrameCheckOptions struct {
	ElementHandleBasePointerOptions
	Strict bool `json:"strict"`
}

type FrameClickOptions struct {
	ElementHandleClickOptions
	Strict bool `json:"strict"`
}

type FrameDblclickOptions struct {
	ElementHandleDblclickOptions
	Strict bool `json:"strict"`
}

type FrameFillOptions struct {
	ElementHandleBaseOptions
	Strict bool `json:"strict"`
}

type FrameGotoOptions struct {
	Referer   string         `json:"referer"`
	Timeout   time.Duration  `json:"timeout"`
	WaitUntil LifecycleEvent `json:"waitUntil" js:"waitUntil"`
}

type FrameHoverOptions struct {
	ElementHandleHoverOptions
	Strict bool `json:"strict"`
}

type FrameInnerHTMLOptions struct {
	FrameBaseOptions
}

type FrameInnerTextOptions struct {
	FrameBaseOptions
}

type FrameInputValueOptions struct {
	FrameBaseOptions
}

type FrameIsCheckedOptions struct {
	FrameBaseOptions
}

type FrameIsDisabledOptions struct {
	FrameBaseOptions
}

type FrameIsEditableOptions struct {
	FrameBaseOptions
}

type FrameIsEnabledOptions struct {
	FrameBaseOptions
}

type FrameIsHiddenOptions struct {
	Strict bool `json:"strict"`
}

type FrameIsVisibleOptions struct {
	Strict bool `json:"strict"`
}

type FramePressOptions struct {
	ElementHandlePressOptions
	Strict bool `json:"strict"`
}

type FrameSelectOptionOptions struct {
	ElementHandleBaseOptions
	Strict bool `json:"strict"`
}

type FrameSetContentOptions struct {
	Timeout   time.Duration  `json:"timeout"`
	WaitUntil LifecycleEvent `json:"waitUntil" js:"waitUntil"`
}

// FrameSetInputFilesOptions are options for Frame.setInputFiles.
type FrameSetInputFilesOptions struct {
	ElementHandleSetInputFilesOptions
	Strict bool `json:"strict"`
}

type FrameTapOptions struct {
	ElementHandleBasePointerOptions
	Modifiers []string `json:"modifiers"`
	Strict    bool     `json:"strict"`
}

type FrameTextContentOptions struct {
	FrameBaseOptions
}

type FrameTypeOptions struct {
	ElementHandleTypeOptions
	Strict bool `json:"strict"`
}

type FrameUncheckOptions struct {
	ElementHandleBasePointerOptions
	Strict bool `json:"strict"`
}

// PollingType is the type of polling to use.
type PollingType int

const (
	// PollingRaf is the requestAnimationFrame polling type.
	PollingRaf PollingType = iota

	// PollingMutation is the mutation polling type.
	PollingMutation

	// PollingInterval is the interval polling type.
	PollingInterval
)

func (p PollingType) String() string {
	return pollingTypeToString[p]
}

var pollingTypeToString = map[PollingType]string{ //nolint:gochecknoglobals
	PollingRaf:      "raf",
	PollingMutation: "mutation",
	PollingInterval: "interval",
}

var pollingTypeToID = map[string]PollingType{ //nolint:gochecknoglobals
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
		return fmt.Errorf("unmarshaling polling type: %w", err)
	}
	// Note that if the string cannot be found then it will be set to the zero value.
	*p = pollingTypeToID[j]
	return nil
}

type FrameWaitForFunctionOptions struct {
	Polling  PollingType   `json:"polling"`
	Interval int64         `json:"interval"`
	Timeout  time.Duration `json:"timeout"`
}

type FrameWaitForLoadStateOptions struct {
	Timeout time.Duration `json:"timeout"`
}

type FrameWaitForNavigationOptions struct {
	URL       string         `json:"url"`
	WaitUntil LifecycleEvent `json:"waitUntil" js:"waitUntil"`
	Timeout   time.Duration  `json:"timeout"`
}

type FrameWaitForSelectorOptions struct {
	State   DOMElementState `json:"state"`
	Strict  bool            `json:"strict"`
	Timeout time.Duration   `json:"timeout"`
}

func NewFrameBaseOptions(defaultTimeout time.Duration) *FrameBaseOptions {
	return &FrameBaseOptions{
		Timeout: defaultTimeout,
		Strict:  false,
	}
}

// Parse parses the frame base options.
func (o *FrameBaseOptions) Parse(ctx context.Context, opts sobek.Value) error {
	rt := k6ext.Runtime(ctx)
	if opts != nil && !sobek.IsUndefined(opts) && !sobek.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "strict":
				o.Strict = opts.Get(k).ToBoolean()
			case "timeout":
				o.Timeout = time.Duration(opts.Get(k).ToInteger()) * time.Millisecond
			}
		}
	}
	return nil
}

func NewFrameCheckOptions(defaultTimeout time.Duration) *FrameCheckOptions {
	return &FrameCheckOptions{
		ElementHandleBasePointerOptions: *NewElementHandleBasePointerOptions(defaultTimeout),
		Strict:                          false,
	}
}

// Parse parses the frame check options.
func (o *FrameCheckOptions) Parse(ctx context.Context, opts sobek.Value) error {
	if err := o.ElementHandleBasePointerOptions.Parse(ctx, opts); err != nil {
		return err
	}
	o.Strict = parseStrict(ctx, opts)
	return nil
}

func NewFrameClickOptions(defaultTimeout time.Duration) *FrameClickOptions {
	return &FrameClickOptions{
		ElementHandleClickOptions: *NewElementHandleClickOptions(defaultTimeout),
		Strict:                    false,
	}
}

// Parse parses the frame click options.
func (o *FrameClickOptions) Parse(ctx context.Context, opts sobek.Value) error {
	if err := o.ElementHandleClickOptions.Parse(ctx, opts); err != nil {
		return err
	}
	o.Strict = parseStrict(ctx, opts)
	return nil
}

func NewFrameDblClickOptions(defaultTimeout time.Duration) *FrameDblclickOptions {
	return &FrameDblclickOptions{
		ElementHandleDblclickOptions: *NewElementHandleDblclickOptions(defaultTimeout),
		Strict:                       false,
	}
}

// Parse parses the frame dblclick options.
func (o *FrameDblclickOptions) Parse(ctx context.Context, opts sobek.Value) error {
	if err := o.ElementHandleDblclickOptions.Parse(ctx, opts); err != nil {
		return err
	}
	o.Strict = parseStrict(ctx, opts)
	return nil
}

func NewFrameFillOptions(defaultTimeout time.Duration) *FrameFillOptions {
	return &FrameFillOptions{
		ElementHandleBaseOptions: *NewElementHandleBaseOptions(defaultTimeout),
		Strict:                   false,
	}
}

// Parse parses the frame fill options.
func (o *FrameFillOptions) Parse(ctx context.Context, opts sobek.Value) error {
	if err := o.ElementHandleBaseOptions.Parse(ctx, opts); err != nil {
		return err
	}
	o.Strict = parseStrict(ctx, opts)
	return nil
}

func NewFrameGotoOptions(defaultReferer string, defaultTimeout time.Duration) *FrameGotoOptions {
	return &FrameGotoOptions{
		Referer:   defaultReferer,
		Timeout:   defaultTimeout,
		WaitUntil: LifecycleEventLoad,
	}
}

// Parse parses the frame goto options.
func (o *FrameGotoOptions) Parse(ctx context.Context, opts sobek.Value) error {
	rt := k6ext.Runtime(ctx)
	if opts != nil && !sobek.IsUndefined(opts) && !sobek.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "referer":
				o.Referer = opts.Get(k).String()
			case "timeout":
				o.Timeout = time.Duration(opts.Get(k).ToInteger()) * time.Millisecond
			case "waitUntil":
				lifeCycle := opts.Get(k).String()
				if err := o.WaitUntil.UnmarshalText([]byte(lifeCycle)); err != nil {
					return fmt.Errorf("parsing goto options: %w", err)
				}
			}
		}
	}
	return nil
}

func NewFrameHoverOptions(defaultTimeout time.Duration) *FrameHoverOptions {
	return &FrameHoverOptions{
		ElementHandleHoverOptions: *NewElementHandleHoverOptions(defaultTimeout),
		Strict:                    false,
	}
}

// Parse parses the frame hover options.
func (o *FrameHoverOptions) Parse(ctx context.Context, opts sobek.Value) error {
	if err := o.ElementHandleHoverOptions.Parse(ctx, opts); err != nil {
		return err
	}
	o.Strict = parseStrict(ctx, opts)
	return nil
}

func NewFrameInnerHTMLOptions(defaultTimeout time.Duration) *FrameInnerHTMLOptions {
	return &FrameInnerHTMLOptions{
		FrameBaseOptions: *NewFrameBaseOptions(defaultTimeout),
	}
}

// Parse parses the frame innerHTML options.
func (o *FrameInnerHTMLOptions) Parse(ctx context.Context, opts sobek.Value) error {
	if err := o.FrameBaseOptions.Parse(ctx, opts); err != nil {
		return err
	}
	return nil
}

func NewFrameInnerTextOptions(defaultTimeout time.Duration) *FrameInnerTextOptions {
	return &FrameInnerTextOptions{
		FrameBaseOptions: *NewFrameBaseOptions(defaultTimeout),
	}
}

// Parse parses the frame innerText options.
func (o *FrameInnerTextOptions) Parse(ctx context.Context, opts sobek.Value) error {
	if err := o.FrameBaseOptions.Parse(ctx, opts); err != nil {
		return err
	}
	return nil
}

func NewFrameInputValueOptions(defaultTimeout time.Duration) *FrameInputValueOptions {
	return &FrameInputValueOptions{
		FrameBaseOptions: *NewFrameBaseOptions(defaultTimeout),
	}
}

// Parse parses the frame inputValue options.
func (o *FrameInputValueOptions) Parse(ctx context.Context, opts sobek.Value) error {
	if err := o.FrameBaseOptions.Parse(ctx, opts); err != nil {
		return err
	}
	return nil
}

func NewFrameIsCheckedOptions(defaultTimeout time.Duration) *FrameIsCheckedOptions {
	return &FrameIsCheckedOptions{
		FrameBaseOptions: *NewFrameBaseOptions(defaultTimeout),
	}
}

// Parse parses the frame isChecked options.
func (o *FrameIsCheckedOptions) Parse(ctx context.Context, opts sobek.Value) error {
	if err := o.FrameBaseOptions.Parse(ctx, opts); err != nil {
		return err
	}
	return nil
}

func NewFrameIsDisabledOptions(defaultTimeout time.Duration) *FrameIsDisabledOptions {
	return &FrameIsDisabledOptions{
		FrameBaseOptions: *NewFrameBaseOptions(defaultTimeout),
	}
}

// Parse parses the frame isDisabled options.
func (o *FrameIsDisabledOptions) Parse(ctx context.Context, opts sobek.Value) error {
	if err := o.FrameBaseOptions.Parse(ctx, opts); err != nil {
		return err
	}
	return nil
}

func NewFrameIsEditableOptions(defaultTimeout time.Duration) *FrameIsEditableOptions {
	return &FrameIsEditableOptions{
		FrameBaseOptions: *NewFrameBaseOptions(defaultTimeout),
	}
}

// Parse parses the frame isEditable options.
func (o *FrameIsEditableOptions) Parse(ctx context.Context, opts sobek.Value) error {
	if err := o.FrameBaseOptions.Parse(ctx, opts); err != nil {
		return err
	}
	return nil
}

func NewFrameIsEnabledOptions(defaultTimeout time.Duration) *FrameIsEnabledOptions {
	return &FrameIsEnabledOptions{
		FrameBaseOptions: *NewFrameBaseOptions(defaultTimeout),
	}
}

// Parse parses the frame isEnabled options.
func (o *FrameIsEnabledOptions) Parse(ctx context.Context, opts sobek.Value) error {
	if err := o.FrameBaseOptions.Parse(ctx, opts); err != nil {
		return err
	}
	return nil
}

// NewFrameIsHiddenOptions creates and returns a new instance of FrameIsHiddenOptions.
func NewFrameIsHiddenOptions() *FrameIsHiddenOptions {
	return &FrameIsHiddenOptions{}
}

// Parse parses FrameIsHiddenOptions from sobek.Value.
func (o *FrameIsHiddenOptions) Parse(ctx context.Context, opts sobek.Value) error {
	o.Strict = parseStrict(ctx, opts)
	return nil
}

// NewFrameIsVisibleOptions creates and returns a new instance of FrameIsVisibleOptions.
func NewFrameIsVisibleOptions() *FrameIsVisibleOptions {
	return &FrameIsVisibleOptions{}
}

// Parse parses FrameIsVisibleOptions from sobek.Value.
func (o *FrameIsVisibleOptions) Parse(ctx context.Context, opts sobek.Value) error {
	o.Strict = parseStrict(ctx, opts)
	return nil
}

func NewFramePressOptions(defaultTimeout time.Duration) *FramePressOptions {
	return &FramePressOptions{
		ElementHandlePressOptions: *NewElementHandlePressOptions(defaultTimeout),
		Strict:                    false,
	}
}

// ToKeyboardOptions converts FramePressOptions to KeyboardOptions.
func (o *FramePressOptions) ToKeyboardOptions() KeyboardOptions {
	var o2 KeyboardOptions
	o2.Delay = o.Delay
	return o2
}

// NewFrameSelectOptionOptions creates and returns a new instance of FrameSelectOptionOptions.
func NewFrameSelectOptionOptions(defaultTimeout time.Duration) *FrameSelectOptionOptions {
	return &FrameSelectOptionOptions{
		ElementHandleBaseOptions: *NewElementHandleBaseOptions(defaultTimeout),
		Strict:                   false,
	}
}

// Parse parses the frame selectOption options.
func (o *FrameSelectOptionOptions) Parse(ctx context.Context, opts sobek.Value) error {
	if err := o.ElementHandleBaseOptions.Parse(ctx, opts); err != nil {
		return err
	}
	o.Strict = parseStrict(ctx, opts)
	return nil
}

func NewFrameSetContentOptions(defaultTimeout time.Duration) *FrameSetContentOptions {
	return &FrameSetContentOptions{
		Timeout:   defaultTimeout,
		WaitUntil: LifecycleEventLoad,
	}
}

// Parse parses the frame setContent options.
func (o *FrameSetContentOptions) Parse(ctx context.Context, opts sobek.Value) error {
	rt := k6ext.Runtime(ctx)

	if opts != nil && !sobek.IsUndefined(opts) && !sobek.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "timeout":
				o.Timeout = time.Duration(opts.Get(k).ToInteger()) * time.Millisecond
			case "waitUntil":
				lifeCycle := opts.Get(k).String()
				if err := o.WaitUntil.UnmarshalText([]byte(lifeCycle)); err != nil {
					return fmt.Errorf("parsing setContent options: %w", err)
				}
			}
		}
	}

	return nil
}

// NewFrameSetInputFilesOptions creates a new FrameSetInputFilesOptions.
func NewFrameSetInputFilesOptions(defaultTimeout time.Duration) *FrameSetInputFilesOptions {
	return &FrameSetInputFilesOptions{
		ElementHandleSetInputFilesOptions: *NewElementHandleSetInputFilesOptions(defaultTimeout),
		Strict:                            false,
	}
}

// Parse parses FrameSetInputFilesOptions from sobek.Value.
func (o *FrameSetInputFilesOptions) Parse(ctx context.Context, opts sobek.Value) error {
	if err := o.ElementHandleSetInputFilesOptions.Parse(ctx, opts); err != nil {
		return err
	}
	return nil
}

func NewFrameTapOptions(defaultTimeout time.Duration) *FrameTapOptions {
	return &FrameTapOptions{
		ElementHandleBasePointerOptions: *NewElementHandleBasePointerOptions(defaultTimeout),
		Modifiers:                       []string{},
		Strict:                          false,
	}
}

// Parse parses the frame tap options.
func (o *FrameTapOptions) Parse(ctx context.Context, opts sobek.Value) error {
	rt := k6ext.Runtime(ctx)
	if err := o.ElementHandleBasePointerOptions.Parse(ctx, opts); err != nil {
		return err
	}
	if opts != nil && !sobek.IsUndefined(opts) && !sobek.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "modifiers":
				var m []string
				if err := rt.ExportTo(opts.Get(k), &m); err != nil {
					return err
				}
				o.Modifiers = m
			case "strict":
				o.Strict = opts.Get(k).ToBoolean()
			}
		}
	}
	return nil
}

func NewFrameTextContentOptions(defaultTimeout time.Duration) *FrameTextContentOptions {
	return &FrameTextContentOptions{
		FrameBaseOptions: *NewFrameBaseOptions(defaultTimeout),
	}
}

// Parse parses the frame textContent options.
func (o *FrameTextContentOptions) Parse(ctx context.Context, opts sobek.Value) error {
	if err := o.FrameBaseOptions.Parse(ctx, opts); err != nil {
		return err
	}
	return nil
}

func NewFrameTypeOptions(defaultTimeout time.Duration) *FrameTypeOptions {
	return &FrameTypeOptions{
		ElementHandleTypeOptions: *NewElementHandleTypeOptions(defaultTimeout),
		Strict:                   false,
	}
}

// ToKeyboardOptions converts FrameTypeOptions to KeyboardOptions.
func (o *FrameTypeOptions) ToKeyboardOptions() KeyboardOptions {
	var o2 KeyboardOptions
	o2.Delay = o.Delay
	return o2
}

func NewFrameUncheckOptions(defaultTimeout time.Duration) *FrameUncheckOptions {
	return &FrameUncheckOptions{
		ElementHandleBasePointerOptions: *NewElementHandleBasePointerOptions(defaultTimeout),
		Strict:                          false,
	}
}

// Parse parses the frame uncheck options.
func (o *FrameUncheckOptions) Parse(ctx context.Context, opts sobek.Value) error {
	if err := o.ElementHandleBasePointerOptions.Parse(ctx, opts); err != nil {
		return err
	}
	o.Strict = parseStrict(ctx, opts)
	return nil
}

func NewFrameWaitForFunctionOptions(defaultTimeout time.Duration) *FrameWaitForFunctionOptions {
	return &FrameWaitForFunctionOptions{
		Polling:  PollingRaf,
		Interval: 0,
		Timeout:  defaultTimeout,
	}
}

// Parse JavaScript waitForFunction options.
func (o *FrameWaitForFunctionOptions) Parse(ctx context.Context, opts sobek.Value) error {
	if !sobekValueExists(opts) {
		return nil
	}

	rt := k6ext.Runtime(ctx)
	obj := opts.ToObject(rt)
	for _, k := range obj.Keys() {
		v := obj.Get(k)
		switch k {
		case "timeout":
			o.Timeout = time.Duration(v.ToInteger()) * time.Millisecond
		case "polling":
			switch v.ExportType().Kind() {
			case reflect.Int64:
				o.Polling = PollingInterval
				o.Interval = v.ToInteger()
			case reflect.String:
				if p, ok := pollingTypeToID[v.ToString().String()]; ok {
					o.Polling = p
					break
				}
				fallthrough
			default:
				return fmt.Errorf("wrong polling option value: %q; "+
					`possible values: "raf", "mutation" or number`, v)
			}
		}
	}

	return nil
}

func NewFrameWaitForLoadStateOptions(defaultTimeout time.Duration) *FrameWaitForLoadStateOptions {
	return &FrameWaitForLoadStateOptions{
		Timeout: defaultTimeout,
	}
}

// Parse parses the frame waitForLoadState options.
func (o *FrameWaitForLoadStateOptions) Parse(ctx context.Context, opts sobek.Value) error {
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

func NewFrameWaitForNavigationOptions(defaultTimeout time.Duration) *FrameWaitForNavigationOptions {
	return &FrameWaitForNavigationOptions{
		URL:       "",
		Timeout:   defaultTimeout,
		WaitUntil: LifecycleEventLoad,
	}
}

// Parse parses the frame waitForNavigation options.
func (o *FrameWaitForNavigationOptions) Parse(ctx context.Context, opts sobek.Value) error {
	rt := k6ext.Runtime(ctx)
	if opts != nil && !sobek.IsUndefined(opts) && !sobek.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "url":
				o.URL = opts.Get(k).String()
			case "timeout":
				o.Timeout = time.Duration(opts.Get(k).ToInteger()) * time.Millisecond
			case "waitUntil":
				lifeCycle := opts.Get(k).String()
				if err := o.WaitUntil.UnmarshalText([]byte(lifeCycle)); err != nil {
					return fmt.Errorf("parsing waitForNavigation options: %w", err)
				}
			}
		}
	}
	return nil
}

func NewFrameWaitForSelectorOptions(defaultTimeout time.Duration) *FrameWaitForSelectorOptions {
	return &FrameWaitForSelectorOptions{
		State:   DOMElementStateVisible,
		Strict:  false,
		Timeout: defaultTimeout,
	}
}

// Parse parses the frame waitForSelector options.
func (o *FrameWaitForSelectorOptions) Parse(ctx context.Context, opts sobek.Value) error {
	rt := k6ext.Runtime(ctx)

	if opts != nil && !sobek.IsUndefined(opts) && !sobek.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "state":
				state := opts.Get(k).String()
				if s, ok := domElementStateToID[state]; ok {
					o.State = s
				} else {
					return fmt.Errorf("%q is not a valid DOM state", state)
				}
			case "strict":
				o.Strict = opts.Get(k).ToBoolean()
			case "timeout":
				o.Timeout = time.Duration(opts.Get(k).ToInteger()) * time.Millisecond
			}
		}
	}

	return nil
}

// FrameDispatchEventOptions are options for Frame.dispatchEvent.
type FrameDispatchEventOptions struct {
	*FrameBaseOptions
}

// NewFrameDispatchEventOptions returns a new FrameDispatchEventOptions.
func NewFrameDispatchEventOptions(defaultTimeout time.Duration) *FrameDispatchEventOptions {
	return &FrameDispatchEventOptions{
		FrameBaseOptions: NewFrameBaseOptions(defaultTimeout),
	}
}

func parseStrict(ctx context.Context, opts sobek.Value) bool {
	var strict bool

	rt := k6ext.Runtime(ctx)
	if opts != nil && !sobek.IsUndefined(opts) && !sobek.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			if k == "strict" {
				strict = opts.Get(k).ToBoolean()
			}
		}
	}

	return strict
}
