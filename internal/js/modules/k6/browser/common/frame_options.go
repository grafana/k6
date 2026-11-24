package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
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

type FrameIsInViewportOptions struct {
	FrameBaseOptions
	// Ratio is the minimum ratio of the element that must intersect the
	// viewport for it to be considered in viewport. It defaults to 0, meaning
	// any visible pixel counts.
	Ratio float64 `json:"ratio"`
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

func PollingIDFromString(format string) (PollingType, bool) {
	id, exists := pollingTypeToID[format]
	return id, exists
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

func NewFrameCheckOptions(defaultTimeout time.Duration) *FrameCheckOptions {
	return &FrameCheckOptions{
		ElementHandleBasePointerOptions: *NewElementHandleBasePointerOptions(defaultTimeout),
		Strict:                          false,
	}
}

func NewFrameClickOptions(defaultTimeout time.Duration) *FrameClickOptions {
	return &FrameClickOptions{
		ElementHandleClickOptions: *NewElementHandleClickOptions(defaultTimeout),
		Strict:                    false,
	}
}

func NewFrameDblClickOptions(defaultTimeout time.Duration) *FrameDblclickOptions {
	return &FrameDblclickOptions{
		ElementHandleDblclickOptions: *NewElementHandleDblclickOptions(defaultTimeout),
		Strict:                       false,
	}
}

func NewFrameFillOptions(defaultTimeout time.Duration) *FrameFillOptions {
	return &FrameFillOptions{
		ElementHandleBaseOptions: *NewElementHandleBaseOptions(defaultTimeout),
		Strict:                   false,
	}
}

func NewFrameGotoOptions(defaultReferer string, defaultTimeout time.Duration) *FrameGotoOptions {
	return &FrameGotoOptions{
		Referer:   defaultReferer,
		Timeout:   defaultTimeout,
		WaitUntil: LifecycleEventLoad,
	}
}

func NewFrameHoverOptions(defaultTimeout time.Duration) *FrameHoverOptions {
	return &FrameHoverOptions{
		ElementHandleHoverOptions: *NewElementHandleHoverOptions(defaultTimeout),
		Strict:                    false,
	}
}

func NewFrameInnerHTMLOptions(defaultTimeout time.Duration) *FrameInnerHTMLOptions {
	return &FrameInnerHTMLOptions{
		FrameBaseOptions: *NewFrameBaseOptions(defaultTimeout),
	}
}

func NewFrameInnerTextOptions(defaultTimeout time.Duration) *FrameInnerTextOptions {
	return &FrameInnerTextOptions{
		FrameBaseOptions: *NewFrameBaseOptions(defaultTimeout),
	}
}

func NewFrameInputValueOptions(defaultTimeout time.Duration) *FrameInputValueOptions {
	return &FrameInputValueOptions{
		FrameBaseOptions: *NewFrameBaseOptions(defaultTimeout),
	}
}

func NewFrameIsCheckedOptions(defaultTimeout time.Duration) *FrameIsCheckedOptions {
	return &FrameIsCheckedOptions{
		FrameBaseOptions: *NewFrameBaseOptions(defaultTimeout),
	}
}

func NewFrameIsDisabledOptions(defaultTimeout time.Duration) *FrameIsDisabledOptions {
	return &FrameIsDisabledOptions{
		FrameBaseOptions: *NewFrameBaseOptions(defaultTimeout),
	}
}

func NewFrameIsEditableOptions(defaultTimeout time.Duration) *FrameIsEditableOptions {
	return &FrameIsEditableOptions{
		FrameBaseOptions: *NewFrameBaseOptions(defaultTimeout),
	}
}

func NewFrameIsEnabledOptions(defaultTimeout time.Duration) *FrameIsEnabledOptions {
	return &FrameIsEnabledOptions{
		FrameBaseOptions: *NewFrameBaseOptions(defaultTimeout),
	}
}

func NewFrameIsInViewportOptions(defaultTimeout time.Duration) *FrameIsInViewportOptions {
	return &FrameIsInViewportOptions{
		FrameBaseOptions: *NewFrameBaseOptions(defaultTimeout),
	}
}

// NewFrameIsHiddenOptions creates and returns a new instance of FrameIsHiddenOptions.
func NewFrameIsHiddenOptions() *FrameIsHiddenOptions {
	return &FrameIsHiddenOptions{}
}

// NewFrameIsVisibleOptions creates and returns a new instance of FrameIsVisibleOptions.
func NewFrameIsVisibleOptions() *FrameIsVisibleOptions {
	return &FrameIsVisibleOptions{}
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

func NewFrameSetContentOptions(defaultTimeout time.Duration) *FrameSetContentOptions {
	return &FrameSetContentOptions{
		Timeout:   defaultTimeout,
		WaitUntil: LifecycleEventLoad,
	}
}

// NewFrameSetInputFilesOptions creates a new FrameSetInputFilesOptions.
func NewFrameSetInputFilesOptions(defaultTimeout time.Duration) *FrameSetInputFilesOptions {
	return &FrameSetInputFilesOptions{
		ElementHandleSetInputFilesOptions: *NewElementHandleSetInputFilesOptions(defaultTimeout),
		Strict:                            false,
	}
}

func NewFrameTapOptions(defaultTimeout time.Duration) *FrameTapOptions {
	return &FrameTapOptions{
		ElementHandleBasePointerOptions: *NewElementHandleBasePointerOptions(defaultTimeout),
		Modifiers:                       []string{},
		Strict:                          false,
	}
}

func NewFrameTextContentOptions(defaultTimeout time.Duration) *FrameTextContentOptions {
	return &FrameTextContentOptions{
		FrameBaseOptions: *NewFrameBaseOptions(defaultTimeout),
	}
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

func NewFrameWaitForFunctionOptions(defaultTimeout time.Duration) *FrameWaitForFunctionOptions {
	return &FrameWaitForFunctionOptions{
		Polling:  PollingRaf,
		Interval: 0,
		Timeout:  defaultTimeout,
	}
}

func NewFrameWaitForLoadStateOptions(defaultTimeout time.Duration) *FrameWaitForLoadStateOptions {
	return &FrameWaitForLoadStateOptions{
		Timeout: defaultTimeout,
	}
}

func NewFrameWaitForNavigationOptions(defaultTimeout time.Duration) *FrameWaitForNavigationOptions {
	return &FrameWaitForNavigationOptions{
		URL:       "",
		Timeout:   defaultTimeout,
		WaitUntil: LifecycleEventLoad,
	}
}

func NewFrameWaitForSelectorOptions(defaultTimeout time.Duration) *FrameWaitForSelectorOptions {
	return &FrameWaitForSelectorOptions{
		State:   DOMElementStateVisible,
		Strict:  false,
		Timeout: defaultTimeout,
	}
}

// FrameDispatchEventOptions are options for Frame.dispatchEvent.
type FrameDispatchEventOptions struct {
	FrameBaseOptions
}

// NewFrameDispatchEventOptions returns a new FrameDispatchEventOptions.
func NewFrameDispatchEventOptions(defaultTimeout time.Duration) *FrameDispatchEventOptions {
	return &FrameDispatchEventOptions{
		FrameBaseOptions: *NewFrameBaseOptions(defaultTimeout),
	}
}

// FrameWaitForURLOptions are options for Frame.waitForURL and Page.waitForURL.
type FrameWaitForURLOptions struct {
	Timeout   time.Duration
	WaitUntil LifecycleEvent
}

// NewFrameWaitForURLOptions returns a new FrameWaitForURLOptions.
func NewFrameWaitForURLOptions(defaultTimeout time.Duration) *FrameWaitForURLOptions {
	return &FrameWaitForURLOptions{
		Timeout:   defaultTimeout,
		WaitUntil: LifecycleEventLoad,
	}
}
