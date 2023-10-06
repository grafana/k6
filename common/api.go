package common

import (
	"context"

	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/dop251/goja"
)

// Cookie represents a browser cookie.
//
// https://datatracker.ietf.org/doc/html/rfc6265.
type Cookie struct {
	Name     string         `js:"name" json:"name"`         // Cookie name.
	Value    string         `js:"value" json:"value"`       // Cookie value.
	Domain   string         `js:"domain" json:"domain"`     // Cookie domain.
	Path     string         `js:"path" json:"path"`         // Cookie path.
	HTTPOnly bool           `js:"httpOnly" json:"httpOnly"` // True if cookie is http-only.
	Secure   bool           `js:"secure" json:"secure"`     // True if cookie is secure.
	SameSite CookieSameSite `js:"sameSite" json:"sameSite"` // Cookie SameSite type.
	URL      string         `js:"url" json:"url,omitempty"` // Cookie URL.
	// Cookie expiration date as the number of seconds since the UNIX epoch.
	Expires int64 `js:"expires" json:"expires"`
}

// CookieSameSite represents the cookie's 'SameSite' status.
//
// https://tools.ietf.org/html/draft-west-first-party-cookies.
type CookieSameSite string

const (
	// CookieSameSiteStrict sets the cookie to be sent only in a first-party
	// context and not be sent along with requests initiated by third party
	// websites.
	CookieSameSiteStrict CookieSameSite = "Strict"

	// CookieSameSiteLax sets the cookie to be sent along with "same-site"
	// requests, and with "cross-site" top-level navigations.
	CookieSameSiteLax CookieSameSite = "Lax"

	// CookieSameSiteNone sets the cookie to be sent in all contexts, i.e
	// potentially insecure third-party requests.
	CookieSameSiteNone CookieSameSite = "None"
)

// BrowserAPI is the public interface of a CDP browser.
type BrowserAPI interface {
	Close()
	Context() *BrowserContext
	IsConnected() bool
	NewContext(opts goja.Value) (*BrowserContext, error)
	NewPage(opts goja.Value) (PageAPI, error)
	On(string) (bool, error)
	UserAgent() string
	Version() string
}

// ConsoleMessageAPI represents a page console message.
type ConsoleMessageAPI struct {
	// Args represent the list of arguments passed to a console function call.
	Args []JSHandleAPI

	// Page is the page that produced the console message, if any.
	Page PageAPI

	// Text represents the text of the console message.
	Text string

	// Type is the type of the console message.
	// It can be one of 'log', 'debug', 'info', 'error', 'warning', 'dir', 'dirxml',
	// 'table', 'trace', 'clear', 'startGroup', 'startGroupCollapsed', 'endGroup',
	// 'assert', 'profile', 'profileEnd', 'count', 'timeEnd'.
	Type string
}

// ElementHandleAPI is the interface of an in-page DOM element.
type ElementHandleAPI interface {
	JSHandleAPI

	BoundingBox() *RectAPI
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

// FrameAPI is the interface of a CDP target frame.
type FrameAPI interface {
	AddScriptTag(opts goja.Value)
	AddStyleTag(opts goja.Value)
	Check(selector string, opts goja.Value)
	ChildFrames() []FrameAPI
	Click(selector string, opts goja.Value) error
	Content() string
	Dblclick(selector string, opts goja.Value)
	DispatchEvent(selector string, typ string, eventInit goja.Value, opts goja.Value)
	// EvaluateWithContext for internal use only
	EvaluateWithContext(ctx context.Context, pageFunc goja.Value, args ...goja.Value) (any, error)
	Evaluate(pageFunc goja.Value, args ...goja.Value) any
	EvaluateHandle(pageFunc goja.Value, args ...goja.Value) (JSHandleAPI, error)
	Fill(selector string, value string, opts goja.Value)
	Focus(selector string, opts goja.Value)
	FrameElement() (ElementHandleAPI, error)
	GetAttribute(selector string, name string, opts goja.Value) goja.Value
	Goto(url string, opts goja.Value) (ResponseAPI, error)
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
	Locator(selector string, opts goja.Value) LocatorAPI
	Name() string
	Query(selector string) (ElementHandleAPI, error)
	QueryAll(selector string) ([]ElementHandleAPI, error)
	Page() PageAPI
	ParentFrame() FrameAPI
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
	WaitForNavigation(opts goja.Value) (ResponseAPI, error)
	WaitForSelector(selector string, opts goja.Value) (ElementHandleAPI, error)
	WaitForTimeout(timeout int64)
}

// JSHandleAPI is the interface of an in-page JS object.
type JSHandleAPI interface {
	AsElement() ElementHandleAPI
	Dispose()
	Evaluate(pageFunc goja.Value, args ...goja.Value) any
	EvaluateHandle(pageFunc goja.Value, args ...goja.Value) (JSHandleAPI, error)
	GetProperties() (map[string]JSHandleAPI, error)
	GetProperty(propertyName string) JSHandleAPI
	JSONValue() goja.Value
	ObjectID() cdpruntime.RemoteObjectID
}

// Strict mode:
// All operations on locators throw an exception if more
// than one element matches the locator's selector.
//
// See Issue #100 for more details.

// LocatorAPI represents a way to find element(s) on a page at any moment.
type LocatorAPI interface {
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

// PageAPI is the interface of a single browser tab.
type PageAPI interface {
	AddInitScript(script goja.Value, arg goja.Value)
	AddScriptTag(opts goja.Value)
	AddStyleTag(opts goja.Value)
	BringToFront()
	Check(selector string, opts goja.Value)
	Click(selector string, opts goja.Value) error
	Close(opts goja.Value) error
	Content() string
	Context() *BrowserContext
	Dblclick(selector string, opts goja.Value)
	DispatchEvent(selector string, typ string, eventInit goja.Value, opts goja.Value)
	DragAndDrop(source string, target string, opts goja.Value)
	EmulateMedia(opts goja.Value)
	EmulateVisionDeficiency(typ string)
	Evaluate(pageFunc goja.Value, arg ...goja.Value) any
	EvaluateHandle(pageFunc goja.Value, arg ...goja.Value) (JSHandleAPI, error)
	ExposeBinding(name string, callback goja.Callable, opts goja.Value)
	ExposeFunction(name string, callback goja.Callable)
	Fill(selector string, value string, opts goja.Value)
	Focus(selector string, opts goja.Value)
	Frame(frameSelector goja.Value) FrameAPI
	Frames() []FrameAPI
	GetAttribute(selector string, name string, opts goja.Value) goja.Value
	GetKeyboard() *Keyboard
	GetMouse() *Mouse
	GetTouchscreen() *Touchscreen
	GoBack(opts goja.Value) ResponseAPI
	GoForward(opts goja.Value) ResponseAPI
	Goto(url string, opts goja.Value) (ResponseAPI, error)
	Hover(selector string, opts goja.Value)
	InnerHTML(selector string, opts goja.Value) string
	InnerText(selector string, opts goja.Value) string
	InputValue(selector string, opts goja.Value) string
	IsChecked(selector string, opts goja.Value) bool
	IsClosed() bool
	IsDisabled(selector string, opts goja.Value) bool
	IsEditable(selector string, opts goja.Value) bool
	IsEnabled(selector string, opts goja.Value) bool
	IsHidden(selector string, opts goja.Value) bool
	IsVisible(selector string, opts goja.Value) bool
	// Locator creates and returns a new locator for this page (main frame).
	Locator(selector string, opts goja.Value) LocatorAPI
	MainFrame() FrameAPI
	On(event string, handler func(*ConsoleMessageAPI) error) error
	Opener() PageAPI
	Pause()
	Pdf(opts goja.Value) []byte
	Press(selector string, key string, opts goja.Value)
	Query(selector string) (ElementHandleAPI, error)
	QueryAll(selector string) ([]ElementHandleAPI, error)
	Reload(opts goja.Value) ResponseAPI
	Route(url goja.Value, handler goja.Callable)
	Screenshot(opts goja.Value) goja.ArrayBuffer
	SelectOption(selector string, values goja.Value, opts goja.Value) []string
	SetContent(html string, opts goja.Value)
	SetDefaultNavigationTimeout(timeout int64)
	SetDefaultTimeout(timeout int64)
	SetExtraHTTPHeaders(headers map[string]string)
	SetInputFiles(selector string, files goja.Value, opts goja.Value)
	SetViewportSize(viewportSize goja.Value)
	Tap(selector string, opts goja.Value)
	TextContent(selector string, opts goja.Value) string
	Title() string
	Type(selector string, text string, opts goja.Value)
	Uncheck(selector string, opts goja.Value)
	Unroute(url goja.Value, handler goja.Callable)
	URL() string
	Video() any
	ViewportSize() map[string]float64
	WaitForEvent(event string, optsOrPredicate goja.Value) any
	WaitForFunction(fn, opts goja.Value, args ...goja.Value) (any, error)
	WaitForLoadState(state string, opts goja.Value)
	WaitForNavigation(opts goja.Value) (ResponseAPI, error)
	WaitForRequest(urlOrPredicate, opts goja.Value) RequestAPI
	WaitForResponse(urlOrPredicate, opts goja.Value) ResponseAPI
	WaitForSelector(selector string, opts goja.Value) (ElementHandleAPI, error)
	WaitForTimeout(timeout int64)
	Workers() []*Worker
}

// RequestAPI is the interface of an HTTP request.
type RequestAPI interface {
	AllHeaders() map[string]string
	Failure() goja.Value
	Frame() FrameAPI
	HeaderValue(string) goja.Value
	Headers() map[string]string
	HeadersArray() []HTTPHeaderAPI
	IsNavigationRequest() bool
	Method() string
	PostData() string
	PostDataBuffer() goja.ArrayBuffer
	PostDataJSON() string
	RedirectedFrom() RequestAPI
	RedirectedTo() RequestAPI
	ResourceType() string
	Response() ResponseAPI
	Size() HTTPMessageSizeAPI
	Timing() goja.Value
	URL() string
}

// ResponseAPI is the interface of an HTTP response.
type ResponseAPI interface {
	AllHeaders() map[string]string
	Body() goja.ArrayBuffer
	Finished() bool
	Frame() FrameAPI
	HeaderValue(string) goja.Value
	HeaderValues(string) []string
	Headers() map[string]string
	HeadersArray() []HTTPHeaderAPI
	JSON() goja.Value
	Ok() bool
	Request() RequestAPI
	SecurityDetails() goja.Value
	ServerAddr() goja.Value
	Size() HTTPMessageSizeAPI
	Status() int64
	StatusText() string
	URL() string
}

// HTTPHeaderAPI is a single HTTP header.
type HTTPHeaderAPI struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// HTTPMessageSizeAPI are the sizes in bytes of the HTTP message header and body.
type HTTPMessageSizeAPI struct {
	Headers int64 `json:"headers"`
	Body    int64 `json:"body"`
}

// Total returns the total size in bytes of the HTTP message.
func (s HTTPMessageSizeAPI) Total() int64 {
	return s.Headers + s.Body
}

// RectAPI is a rectangle.
type RectAPI struct {
	X      float64 `js:"x"`
	Y      float64 `js:"y"`
	Width  float64 `js:"width"`
	Height float64 `js:"height"`
}
