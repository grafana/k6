package api

import "github.com/dop251/goja"

// Strict mode:
// All operations on locators throw an exception if more
// than one element matches the locator's selector.
//
// See Issue #100 for more details.

// Locator represents a way to find element(s) on a page at any moment.
type Locator interface {
	// Click on an element using locator's selector with strict mode on.
	Click(opts goja.Value) error
	// Dblclick double clicks on an element using locator's selector with strict mode on.
	Dblclick(opts goja.Value)
	// Check element using locator's selector with strict mode on.
	Check(opts goja.Value)
	// Uncheck element using locator's selector with strict mode on.
	Uncheck(opts goja.Value)
	// IsChecked returns true if the element matches the locator's
	// selector and is checked. Otherwise, returns false.
	IsChecked(opts goja.Value) bool
	// IsEditable returns true if the element matches the locator's
	// selector and is editable. Otherwise, returns false.
	IsEditable(opts goja.Value) bool
	// IsEnabled returns true if the element matches the locator's
	// selector and is enabled. Otherwise, returns false.
	IsEnabled(opts goja.Value) bool
	// IsDisabled returns true if the element matches the locator's
	// selector and is disabled. Otherwise, returns false.
	IsDisabled(opts goja.Value) bool
	// IsVisible returns true if the element matches the locator's
	// selector and is visible. Otherwise, returns false.
	IsVisible(opts goja.Value) bool
	// IsHidden returns true if the element matches the locator's
	// selector and is hidden. Otherwise, returns false.
	IsHidden(opts goja.Value) bool
	// Fill out the element using locator's selector with strict mode on.
	Fill(value string, opts goja.Value)
	// Focus on the element using locator's selector with strict mode on.
	Focus(opts goja.Value)
	// GetAttribute of the element using locator's selector with strict mode on.
	GetAttribute(name string, opts goja.Value) goja.Value
	// InnerHTML returns the element's inner HTML that matches
	// the locator's selector with strict mode on.
	InnerHTML(opts goja.Value) string
	// InnerText returns the element's inner text that matches
	// the locator's selector with strict mode on.
	InnerText(opts goja.Value) string
	// TextContent returns the element's text content that matches
	// the locator's selector with strict mode on.
	TextContent(opts goja.Value) string
	// InputValue returns the element's input value that matches
	// the locator's selector with strict mode on.
	InputValue(opts goja.Value) string
	// SelectOption, filters option values of the first element that matches
	// the locator's selector (with strict mode on), selects the
	// options, and returns the filtered options.
	SelectOption(values goja.Value, opts goja.Value) []string
	// Press the given key on the element found that matches the locator's
	// selector with strict mode on.
	Press(key string, opts goja.Value)
	// Type text on the element found that matches the locator's
	// selector with strict mode on.
	Type(text string, opts goja.Value)
	// Hover moves the pointer over the element that matches the locator's
	// selector with strict mode on.
	Hover(opts goja.Value)
	// Tap the element found that matches the locator's selector with strict mode on.
	Tap(opts goja.Value)
	// DispatchEvent dispatches an event for the element matching the
	// locator's selector with strict mode on.
	DispatchEvent(typ string, eventInit, opts goja.Value)
	// WaitFor waits for the element matching the locator's selector
	// with strict mode on.
	WaitFor(opts goja.Value)
}
