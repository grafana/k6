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
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/dop251/goja"
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
	WaitUntil LifecycleEvent `json:"waitUntil"`
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
	FrameBaseOptions
}

type FrameIsVisibleOptions struct {
	FrameBaseOptions
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
	WaitUntil LifecycleEvent `json:"waitUntil"`
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
	WaitUntil LifecycleEvent `json:"waitUntil"`
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

func (o *FrameBaseOptions) Parse(ctx context.Context, opts goja.Value) error {
	rt := GetVU(ctx).Runtime()
	if opts != nil && !goja.IsUndefined(opts) && !goja.IsNull(opts) {
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

func (o *FrameCheckOptions) Parse(ctx context.Context, opts goja.Value) error {
	rt := GetVU(ctx).Runtime()
	if err := o.ElementHandleBasePointerOptions.Parse(ctx, opts); err != nil {
		return err
	}
	if opts != nil && !goja.IsUndefined(opts) && !goja.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "strict":
				o.Strict = opts.Get(k).ToBoolean()
			}
		}
	}
	return nil
}

func NewFrameClickOptions(defaultTimeout time.Duration) *FrameClickOptions {
	return &FrameClickOptions{
		ElementHandleClickOptions: *NewElementHandleClickOptions(defaultTimeout),
		Strict:                    false,
	}
}

func (o *FrameClickOptions) Parse(ctx context.Context, opts goja.Value) error {
	rt := GetVU(ctx).Runtime()
	if err := o.ElementHandleClickOptions.Parse(ctx, opts); err != nil {
		return err
	}
	if opts != nil && !goja.IsUndefined(opts) && !goja.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "strict":
				o.Strict = opts.Get(k).ToBoolean()
			}
		}
	}
	return nil
}

func NewFrameDblClickOptions(defaultTimeout time.Duration) *FrameDblclickOptions {
	return &FrameDblclickOptions{
		ElementHandleDblclickOptions: *NewElementHandleDblclickOptions(defaultTimeout),
		Strict:                       false,
	}
}

func (o *FrameDblclickOptions) Parse(ctx context.Context, opts goja.Value) error {
	rt := GetVU(ctx).Runtime()
	if err := o.ElementHandleDblclickOptions.Parse(ctx, opts); err != nil {
		return err
	}
	if opts != nil && !goja.IsUndefined(opts) && !goja.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "strict":
				o.Strict = opts.Get(k).ToBoolean()
			}
		}
	}
	return nil
}

func NewFrameFillOptions(defaultTimeout time.Duration) *FrameFillOptions {
	return &FrameFillOptions{
		ElementHandleBaseOptions: *NewElementHandleBaseOptions(defaultTimeout),
		Strict:                   false,
	}
}

func (o *FrameFillOptions) Parse(ctx context.Context, opts goja.Value) error {
	rt := GetVU(ctx).Runtime()
	if err := o.ElementHandleBaseOptions.Parse(ctx, opts); err != nil {
		return err
	}
	if opts != nil && !goja.IsUndefined(opts) && !goja.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "strict":
				o.Strict = opts.Get(k).ToBoolean()
			}
		}
	}
	return nil
}

func NewFrameGotoOptions(defaultReferer string, defaultTimeout time.Duration) *FrameGotoOptions {
	return &FrameGotoOptions{
		Referer:   defaultReferer,
		Timeout:   defaultTimeout,
		WaitUntil: LifecycleEventLoad,
	}
}

func (o *FrameGotoOptions) Parse(ctx context.Context, opts goja.Value) error {
	rt := GetVU(ctx).Runtime()
	if opts != nil && !goja.IsUndefined(opts) && !goja.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "referer":
				o.Referer = opts.Get(k).String()
			case "timeout":
				o.Timeout = time.Duration(opts.Get(k).ToInteger()) * time.Millisecond
			case "waitUntil":
				lifeCycle := opts.Get(k).String()
				if l, ok := lifecycleEventToID[lifeCycle]; ok {
					o.WaitUntil = l
				} else {
					return fmt.Errorf("%q is not a valid lifecycle", lifeCycle)
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

func (o *FrameHoverOptions) Parse(ctx context.Context, opts goja.Value) error {
	rt := GetVU(ctx).Runtime()
	if err := o.ElementHandleHoverOptions.Parse(ctx, opts); err != nil {
		return err
	}
	if opts != nil && !goja.IsUndefined(opts) && !goja.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "strict":
				o.Strict = opts.Get(k).ToBoolean()
			}
		}
	}
	return nil
}

func NewFrameInnerHTMLOptions(defaultTimeout time.Duration) *FrameInnerHTMLOptions {
	return &FrameInnerHTMLOptions{
		FrameBaseOptions: *NewFrameBaseOptions(defaultTimeout),
	}
}

func (o *FrameInnerHTMLOptions) Parse(ctx context.Context, opts goja.Value) error {
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

func (o *FrameInnerTextOptions) Parse(ctx context.Context, opts goja.Value) error {
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

func (o *FrameInputValueOptions) Parse(ctx context.Context, opts goja.Value) error {
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

func (o *FrameIsCheckedOptions) Parse(ctx context.Context, opts goja.Value) error {
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

func (o *FrameIsDisabledOptions) Parse(ctx context.Context, opts goja.Value) error {
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

func (o *FrameIsEditableOptions) Parse(ctx context.Context, opts goja.Value) error {
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

func (o *FrameIsEnabledOptions) Parse(ctx context.Context, opts goja.Value) error {
	if err := o.FrameBaseOptions.Parse(ctx, opts); err != nil {
		return err
	}
	return nil
}

func NewFrameIsHiddenOptions(defaultTimeout time.Duration) *FrameIsHiddenOptions {
	return &FrameIsHiddenOptions{
		FrameBaseOptions: *NewFrameBaseOptions(defaultTimeout),
	}
}

func (o *FrameIsHiddenOptions) Parse(ctx context.Context, opts goja.Value) error {
	if err := o.FrameBaseOptions.Parse(ctx, opts); err != nil {
		return err
	}
	return nil
}

func NewFrameIsVisibleOptions(defaultTimeout time.Duration) *FrameIsVisibleOptions {
	return &FrameIsVisibleOptions{
		FrameBaseOptions: *NewFrameBaseOptions(defaultTimeout),
	}
}

func (o *FrameIsVisibleOptions) Parse(ctx context.Context, opts goja.Value) error {
	if err := o.FrameBaseOptions.Parse(ctx, opts); err != nil {
		return err
	}
	return nil
}

func NewFramePressOptions(defaultTimeout time.Duration) *FramePressOptions {
	return &FramePressOptions{
		ElementHandlePressOptions: *NewElementHandlePressOptions(defaultTimeout),
		Strict:                    false,
	}
}

func (o *FramePressOptions) ToKeyboardOptions() *KeyboardOptions {
	o2 := NewKeyboardOptions()
	o2.Delay = o.Delay
	return o2
}

func NewFrameSelectOptionOptions(defaultTimeout time.Duration) *FrameSelectOptionOptions {
	return &FrameSelectOptionOptions{
		ElementHandleBaseOptions: *NewElementHandleBaseOptions(defaultTimeout),
		Strict:                   false,
	}
}

func (o *FrameSelectOptionOptions) Parse(ctx context.Context, opts goja.Value) error {
	rt := GetVU(ctx).Runtime()
	if err := o.ElementHandleBaseOptions.Parse(ctx, opts); err != nil {
		return err
	}
	if opts != nil && !goja.IsUndefined(opts) && !goja.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "strict":
				o.Strict = opts.Get(k).ToBoolean()
			}
		}
	}
	return nil
}

func NewFrameSetContentOptions(defaultTimeout time.Duration) *FrameSetContentOptions {
	return &FrameSetContentOptions{
		Timeout:   defaultTimeout,
		WaitUntil: LifecycleEventLoad,
	}
}

func (o *FrameSetContentOptions) Parse(ctx context.Context, opts goja.Value) error {
	rt := GetVU(ctx).Runtime()

	if opts != nil && !goja.IsUndefined(opts) && !goja.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "timeout":
				o.Timeout = time.Duration(opts.Get(k).ToInteger()) * time.Millisecond
			case "waitUntil":
				lifeCycle := opts.Get(k).String()
				if l, ok := lifecycleEventToID[lifeCycle]; ok {
					o.WaitUntil = l
				} else {
					return fmt.Errorf("%q is not a valid lifecycle", lifeCycle)
				}
			}
		}
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

func (o *FrameTapOptions) Parse(ctx context.Context, opts goja.Value) error {
	rt := GetVU(ctx).Runtime()
	if err := o.ElementHandleBasePointerOptions.Parse(ctx, opts); err != nil {
		return err
	}
	if opts != nil && !goja.IsUndefined(opts) && !goja.IsNull(opts) {
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

func (o *FrameTextContentOptions) Parse(ctx context.Context, opts goja.Value) error {
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

func (o *FrameTypeOptions) ToKeyboardOptions() *KeyboardOptions {
	o2 := NewKeyboardOptions()
	o2.Delay = o.Delay
	return o2
}

func NewFrameUncheckOptions(defaultTimeout time.Duration) *FrameUncheckOptions {
	return &FrameUncheckOptions{
		ElementHandleBasePointerOptions: *NewElementHandleBasePointerOptions(defaultTimeout),
		Strict:                          false,
	}
}

func (o *FrameUncheckOptions) Parse(ctx context.Context, opts goja.Value) error {
	rt := GetVU(ctx).Runtime()
	if err := o.ElementHandleBasePointerOptions.Parse(ctx, opts); err != nil {
		return err
	}
	if opts != nil && !goja.IsUndefined(opts) && !goja.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "strict":
				o.Strict = opts.Get(k).ToBoolean()
			}
		}
	}
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
func (o *FrameWaitForFunctionOptions) Parse(ctx context.Context, opts goja.Value) error {
	rt := GetVU(ctx).Runtime()

	if opts != nil && !goja.IsUndefined(opts) && !goja.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			v := opts.Get(k)
			switch k {
			case "timeout":
				o.Timeout = time.Duration(v.ToInteger()) * time.Millisecond
			case "polling":
				switch v.ExportType().Kind() { //nolint: exhaustive
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
	}

	return nil
}

func NewFrameWaitForLoadStateOptions(defaultTimeout time.Duration) *FrameWaitForLoadStateOptions {
	return &FrameWaitForLoadStateOptions{
		Timeout: defaultTimeout,
	}
}

func (o *FrameWaitForLoadStateOptions) Parse(ctx context.Context, opts goja.Value) error {
	rt := GetVU(ctx).Runtime()
	if opts != nil && !goja.IsUndefined(opts) && !goja.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "timeout":
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

func (o *FrameWaitForNavigationOptions) Parse(ctx context.Context, opts goja.Value) error {
	rt := GetVU(ctx).Runtime()
	if opts != nil && !goja.IsUndefined(opts) && !goja.IsNull(opts) {
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
					return fmt.Errorf("error parsing waitForNavigation options: %w", err)
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

func (o *FrameWaitForSelectorOptions) Parse(ctx context.Context, opts goja.Value) error {
	rt := GetVU(ctx).Runtime()

	if opts != nil && !goja.IsUndefined(opts) && !goja.IsNull(opts) {
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
