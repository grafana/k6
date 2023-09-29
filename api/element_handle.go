package api

import "github.com/dop251/goja"

// ElementHandleAPI is the interface of an in-page DOM element.
type ElementHandleAPI interface {
	JSHandle

	BoundingBox() *Rect
	Check(opts goja.Value)
	Click(opts goja.Value) error
	ContentFrame() (FrameAPI, error)
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
	OwnerFrame() (FrameAPI, error)
	Press(key string, opts goja.Value)
	Query(selector string) (ElementHandleAPI, error)
	QueryAll(selector string) ([]ElementHandleAPI, error)
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
	WaitForSelector(selector string, opts goja.Value) (ElementHandleAPI, error)
}
