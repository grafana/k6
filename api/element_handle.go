package api

import "github.com/dop251/goja"

// ElementHandle is the interface of an in-page DOM element.
type ElementHandle interface {
	JSHandle

	BoundingBox() *Rect
	Check(opts goja.Value)
	Click(opts goja.Value) error
	ContentFrame() (Frame, error)
	Dblclick(opts goja.Value)
	DispatchEvent(typ string, props goja.Value)
	Fill(value string, opts goja.Value)
	Focus()
	GetAttribute(name string) goja.Value
	Hover(opts goja.Value)
	InnerHTML() string
	InnerText() string
	InputValue(opts goja.Value) string
	IsChecked() bool
	IsDisabled() bool
	IsEditable() bool
	IsEnabled() bool
	IsHidden() bool
	IsVisible() bool
	OwnerFrame() (Frame, error)
	Press(key string, opts goja.Value)
	Query(selector string) (ElementHandle, error)
	QueryAll(selector string) ([]ElementHandle, error)
	Screenshot(opts goja.Value) goja.ArrayBuffer
	ScrollIntoViewIfNeeded(opts goja.Value)
	SelectOption(values goja.Value, opts goja.Value) []string
	SelectText(opts goja.Value)
	SetInputFiles(files goja.Value, opts goja.Value)
	Tap(opts goja.Value)
	TextContent() string
	Type(text string, opts goja.Value)
	Uncheck(opts goja.Value)
	WaitForElementState(state string, opts goja.Value)
	WaitForSelector(selector string, opts goja.Value) (ElementHandle, error)
}
