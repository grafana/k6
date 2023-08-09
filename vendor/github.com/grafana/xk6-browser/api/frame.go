package api

import (
	"context"

	"github.com/dop251/goja"
)

// Frame is the interface of a CDP target frame.
type Frame interface {
	AddScriptTag(opts goja.Value)
	AddStyleTag(opts goja.Value)
	Check(selector string, opts goja.Value)
	ChildFrames() []Frame
	Click(selector string, opts goja.Value) error
	Content() string
	Dblclick(selector string, opts goja.Value)
	DispatchEvent(selector string, typ string, eventInit goja.Value, opts goja.Value)
	// EvaluateWithContext for internal use only
	EvaluateWithContext(ctx context.Context, pageFunc goja.Value, args ...goja.Value) (any, error)
	Evaluate(pageFunc goja.Value, args ...goja.Value) any
	EvaluateHandle(pageFunc goja.Value, args ...goja.Value) (JSHandle, error)
	Fill(selector string, value string, opts goja.Value)
	Focus(selector string, opts goja.Value)
	FrameElement() (ElementHandle, error)
	GetAttribute(selector string, name string, opts goja.Value) goja.Value
	Goto(url string, opts goja.Value) (Response, error)
	Hover(selector string, opts goja.Value)
	InnerHTML(selector string, opts goja.Value) string
	InnerText(selector string, opts goja.Value) string
	InputValue(selector string, opts goja.Value) string
	IsChecked(selector string, opts goja.Value) bool
	IsDetached() bool
	IsDisabled(selector string, opts goja.Value) bool
	IsEditable(selector string, opts goja.Value) bool
	IsEnabled(selector string, opts goja.Value) bool
	IsHidden(selector string, opts goja.Value) bool
	IsVisible(selector string, opts goja.Value) bool
	ID() string
	LoaderID() string
	// Locator creates and returns a new locator for this frame.
	Locator(selector string, opts goja.Value) Locator
	Name() string
	Query(selector string) (ElementHandle, error)
	QueryAll(selector string) ([]ElementHandle, error)
	Page() Page
	ParentFrame() Frame
	Press(selector string, key string, opts goja.Value)
	SelectOption(selector string, values goja.Value, opts goja.Value) []string
	SetContent(html string, opts goja.Value)
	SetInputFiles(selector string, files goja.Value, opts goja.Value)
	Tap(selector string, opts goja.Value)
	TextContent(selector string, opts goja.Value) string
	Title() string
	Type(selector string, text string, opts goja.Value)
	Uncheck(selector string, opts goja.Value)
	URL() string
	WaitForFunction(pageFunc, opts goja.Value, args ...goja.Value) (any, error)
	WaitForLoadState(state string, opts goja.Value)
	WaitForNavigation(opts goja.Value) (Response, error)
	WaitForSelector(selector string, opts goja.Value) (ElementHandle, error)
	WaitForTimeout(timeout int64)
}
