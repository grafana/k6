package api

import "github.com/dop251/goja"

// Frame is the interface of a CDP target frame.
type Frame interface {
	AddScriptTag(opts goja.Value)
	AddStyleTag(opts goja.Value)
	Check(selector string, opts goja.Value)
	ChildFrames() []Frame
	Click(selector string, opts goja.Value) *goja.Promise
	Content() string
	Dblclick(selector string, opts goja.Value)
	DispatchEvent(selector string, typ string, eventInit goja.Value, opts goja.Value)
	Evaluate(pageFunc goja.Value, args ...goja.Value) interface{}
	EvaluateHandle(pageFunc goja.Value, args ...goja.Value) JSHandle
	Fill(selector string, value string, opts goja.Value)
	Focus(selector string, opts goja.Value)
	FrameElement() ElementHandle
	GetAttribute(selector string, name string, opts goja.Value) goja.Value
	Goto(url string, opts goja.Value) Response
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
	Query(selector string) ElementHandle
	QueryAll(selector string) []ElementHandle
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
	WaitForFunction(pageFunc, opts goja.Value, args ...goja.Value) *goja.Promise
	WaitForLoadState(state string, opts goja.Value)
	WaitForNavigation(opts goja.Value) Response
	WaitForSelector(selector string, opts goja.Value) ElementHandle
	WaitForTimeout(timeout int64)
}
