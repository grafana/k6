package browser

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/require"

	"github.com/grafana/xk6-browser/common"

	k6common "go.k6.io/k6/js/common"
	k6modulestest "go.k6.io/k6/js/modulestest"
	k6lib "go.k6.io/k6/lib"
	k6metrics "go.k6.io/k6/metrics"
)

// customMappings is a list of custom mappings for our API (api/).
// Some of them are wildcards, such as query to $ mapping; and
// others are for publicly accessible fields, such as mapping
// of page.keyboard to Page.getKeyboard.
func customMappings() map[string]string {
	return map[string]string{
		// wildcards
		"pageAPI.query":             "$",
		"pageAPI.queryAll":          "$$",
		"frameAPI.query":            "$",
		"frameAPI.queryAll":         "$$",
		"elementHandleAPI.query":    "$",
		"elementHandleAPI.queryAll": "$$",
		// getters
		"pageAPI.getKeyboard":    "keyboard",
		"pageAPI.getMouse":       "mouse",
		"pageAPI.getTouchscreen": "touchscreen",
		// internal methods
		"elementHandleAPI.objectID":    "",
		"frameAPI.id":                  "",
		"frameAPI.loaderID":            "",
		"JSHandleAPI.objectID":         "",
		"browserAPI.close":             "",
		"frameAPI.evaluateWithContext": "",
		// TODO: browser.on method is unexposed until more event
		// types other than 'disconnect' are supported.
		// See: https://github.com/grafana/xk6-browser/issues/913
		"browserAPI.on": "",
	}
}

// TestMappings tests that all the methods of the API (api/) are
// to the module. This is to ensure that we don't forget to map
// a new method to the module.
func TestMappings(t *testing.T) {
	t.Parallel()

	type test struct {
		apiInterface any
		mapp         func() mapping
	}

	var (
		vu = &k6modulestest.VU{
			RuntimeField: goja.New(),
			InitEnvField: &k6common.InitEnvironment{
				TestPreInitState: &k6lib.TestPreInitState{
					Registry: k6metrics.NewRegistry(),
				},
			},
		}
		customMappings = customMappings()
	)

	// testMapping tests that all the methods of an API are mapped
	// to the module. And wildcards are mapped correctly and their
	// methods are not mapped.
	testMapping := func(t *testing.T, tt test) {
		t.Helper()

		var (
			typ    = reflect.TypeOf(tt.apiInterface).Elem()
			mapped = tt.mapp()
			tested = make(map[string]bool)
		)
		for i := 0; i < typ.NumMethod(); i++ {
			method := typ.Method(i)
			require.NotNil(t, method)

			// goja uses methods that starts with lowercase.
			// so we need to convert the first letter to lowercase.
			m := toFirstLetterLower(method.Name)

			cm, cmok := isCustomMapping(customMappings, typ.Name(), m)
			// if the method is a custom mapping, it should not be
			// mapped to the module. so we should not find it in
			// the mapped methods.
			if _, ok := mapped[m]; cmok && ok {
				t.Errorf("method %q should not be mapped", m)
			}
			// a custom mapping with an empty string means that
			// the method should not exist on the API.
			if cmok && cm == "" {
				continue
			}
			// change the method name if it is mapped to a custom
			// method. these custom methods are not exist on our
			// API. so we need to use the mapped method instead.
			if cmok {
				m = cm
			}
			if _, ok := mapped[m]; !ok {
				t.Errorf("method %q not found", m)
			}
			// to detect if a method is redundantly mapped.
			tested[m] = true
		}
		// detect redundant mappings.
		for m := range mapped {
			if !tested[m] {
				t.Errorf("method %q is redundant", m)
			}
		}
	}

	for name, tt := range map[string]test{
		"browser": {
			apiInterface: (*browserAPI)(nil),
			mapp: func() mapping {
				return mapBrowser(moduleVU{VU: vu})
			},
		},
		"browserContext": {
			apiInterface: (*browserContextAPI)(nil),
			mapp: func() mapping {
				return mapBrowserContext(moduleVU{VU: vu}, &common.BrowserContext{})
			},
		},
		"page": {
			apiInterface: (*pageAPI)(nil),
			mapp: func() mapping {
				return mapPage(moduleVU{VU: vu}, &common.Page{
					Keyboard:    &common.Keyboard{},
					Mouse:       &common.Mouse{},
					Touchscreen: &common.Touchscreen{},
				})
			},
		},
		"elementHandle": {
			apiInterface: (*elementHandleAPI)(nil),
			mapp: func() mapping {
				return mapElementHandle(moduleVU{VU: vu}, &common.ElementHandle{})
			},
		},
		"jsHandle": {
			apiInterface: (*common.JSHandleAPI)(nil),
			mapp: func() mapping {
				return mapJSHandle(moduleVU{VU: vu}, &common.BaseJSHandle{})
			},
		},
		"frame": {
			apiInterface: (*frameAPI)(nil),
			mapp: func() mapping {
				return mapFrame(moduleVU{VU: vu}, &common.Frame{})
			},
		},
		"mapRequest": {
			apiInterface: (*requestAPI)(nil),
			mapp: func() mapping {
				return mapRequest(moduleVU{VU: vu}, &common.Request{})
			},
		},
		"mapResponse": {
			apiInterface: (*responseAPI)(nil),
			mapp: func() mapping {
				return mapResponse(moduleVU{VU: vu}, &common.Response{})
			},
		},
		"mapWorker": {
			apiInterface: (*workerAPI)(nil),
			mapp: func() mapping {
				return mapWorker(moduleVU{VU: vu}, &common.Worker{})
			},
		},
		"mapLocator": {
			apiInterface: (*locatorAPI)(nil),
			mapp: func() mapping {
				return mapLocator(moduleVU{VU: vu}, &common.Locator{})
			},
		},
		"mapConsoleMessage": {
			apiInterface: (*interface {
				Args() []common.JSHandleAPI
				Page() *common.Page
				Text() string
				Type() string
			})(nil),
			mapp: func() mapping {
				return mapConsoleMessage(moduleVU{VU: vu}, &common.ConsoleMessageAPI{})
			},
		},
	} {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			testMapping(t, tt)
		})
	}
}

// toFirstLetterLower converts the first letter of the string to lower case.
func toFirstLetterLower(s string) string {
	// Special cases.
	// Instead of loading up an acronyms list, just do this.
	// Good enough for our purposes.
	special := map[string]string{
		"ID":        "id",
		"JSON":      "json",
		"JSONValue": "jsonValue",
		"URL":       "url",
	}
	if v, ok := special[s]; ok {
		return v
	}
	if s == "" {
		return ""
	}

	return strings.ToLower(s[:1]) + s[1:]
}

// isCustomMapping returns true if the method is a custom mapping
// and returns the name of the method to be called instead of the
// original one.
func isCustomMapping(customMappings map[string]string, typ, method string) (string, bool) {
	name := typ + "." + method

	if s, ok := customMappings[name]; ok {
		return s, ok
	}

	return "", false
}

// ----------------------------------------------------------------------------
// JavaScript API definitions.
// ----------------------------------------------------------------------------

// browserAPI is the public interface of a CDP browser.
type browserAPI interface {
	Close()
	Context() *common.BrowserContext
	IsConnected() bool
	NewContext(opts goja.Value) (*common.BrowserContext, error)
	NewPage(opts goja.Value) (*common.Page, error)
	On(string) (bool, error)
	UserAgent() string
	Version() string
}

// browserContextAPI is the public interface of a CDP browser context.
type browserContextAPI interface {
	AddCookies(cookies []*common.Cookie) error
	AddInitScript(script goja.Value, arg goja.Value) error
	Browser() *common.Browser
	ClearCookies() error
	ClearPermissions()
	Close()
	Cookies(urls ...string) ([]*common.Cookie, error)
	ExposeBinding(name string, callback goja.Callable, opts goja.Value)
	ExposeFunction(name string, callback goja.Callable)
	GrantPermissions(permissions []string, opts goja.Value)
	NewCDPSession() any
	NewPage() (*common.Page, error)
	Pages() []*common.Page
	Route(url goja.Value, handler goja.Callable)
	SetDefaultNavigationTimeout(timeout int64)
	SetDefaultTimeout(timeout int64)
	SetExtraHTTPHeaders(headers map[string]string) error
	SetGeolocation(geolocation goja.Value)
	// SetHTTPCredentials sets username/password credentials to use for HTTP authentication.
	//
	// Deprecated: Create a new BrowserContext with httpCredentials instead.
	// See for details:
	// - https://github.com/microsoft/playwright/issues/2196#issuecomment-627134837
	// - https://github.com/microsoft/playwright/pull/2763
	SetHTTPCredentials(httpCredentials goja.Value)
	SetOffline(offline bool)
	StorageState(opts goja.Value)
	Unroute(url goja.Value, handler goja.Callable)
	WaitForEvent(event string, optsOrPredicate goja.Value) (any, error)
}

// pageAPI is the interface of a single browser tab.
type pageAPI interface {
	AddInitScript(script goja.Value, arg goja.Value)
	AddScriptTag(opts goja.Value)
	AddStyleTag(opts goja.Value)
	BringToFront()
	Check(selector string, opts goja.Value)
	Click(selector string, opts goja.Value) error
	Close(opts goja.Value) error
	Content() string
	Context() *common.BrowserContext
	Dblclick(selector string, opts goja.Value)
	DispatchEvent(selector string, typ string, eventInit goja.Value, opts goja.Value)
	DragAndDrop(source string, target string, opts goja.Value)
	EmulateMedia(opts goja.Value)
	EmulateVisionDeficiency(typ string)
	Evaluate(pageFunc goja.Value, arg ...goja.Value) any
	EvaluateHandle(pageFunc goja.Value, arg ...goja.Value) (common.JSHandleAPI, error)
	ExposeBinding(name string, callback goja.Callable, opts goja.Value)
	ExposeFunction(name string, callback goja.Callable)
	Fill(selector string, value string, opts goja.Value)
	Focus(selector string, opts goja.Value)
	Frame(frameSelector goja.Value) *common.Frame
	Frames() []*common.Frame
	GetAttribute(selector string, name string, opts goja.Value) goja.Value
	GetKeyboard() *common.Keyboard
	GetMouse() *common.Mouse
	GetTouchscreen() *common.Touchscreen
	GoBack(opts goja.Value) *common.Response
	GoForward(opts goja.Value) *common.Response
	Goto(url string, opts goja.Value) (*common.Response, error)
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
	Locator(selector string, opts goja.Value) *common.Locator
	MainFrame() *common.Frame
	On(event string, handler func(*common.ConsoleMessageAPI) error) error
	Opener() pageAPI
	Pause()
	Pdf(opts goja.Value) []byte
	Press(selector string, key string, opts goja.Value)
	Query(selector string) (*common.ElementHandle, error)
	QueryAll(selector string) ([]*common.ElementHandle, error)
	Reload(opts goja.Value) *common.Response
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
	WaitForNavigation(opts goja.Value) (*common.Response, error)
	WaitForRequest(urlOrPredicate, opts goja.Value) *common.Request
	WaitForResponse(urlOrPredicate, opts goja.Value) *common.Response
	WaitForSelector(selector string, opts goja.Value) (*common.ElementHandle, error)
	WaitForTimeout(timeout int64)
	Workers() []*common.Worker
}

// frameAPI is the interface of a CDP target frame.
type frameAPI interface {
	AddScriptTag(opts goja.Value)
	AddStyleTag(opts goja.Value)
	Check(selector string, opts goja.Value)
	ChildFrames() []*common.Frame
	Click(selector string, opts goja.Value) error
	Content() string
	Dblclick(selector string, opts goja.Value)
	DispatchEvent(selector string, typ string, eventInit goja.Value, opts goja.Value)
	// EvaluateWithContext for internal use only
	EvaluateWithContext(ctx context.Context, pageFunc goja.Value, args ...goja.Value) (any, error)
	Evaluate(pageFunc goja.Value, args ...goja.Value) any
	EvaluateHandle(pageFunc goja.Value, args ...goja.Value) (common.JSHandleAPI, error)
	Fill(selector string, value string, opts goja.Value)
	Focus(selector string, opts goja.Value)
	FrameElement() (*common.ElementHandle, error)
	GetAttribute(selector string, name string, opts goja.Value) goja.Value
	Goto(url string, opts goja.Value) (*common.Response, error)
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
	Locator(selector string, opts goja.Value) *common.Locator
	Name() string
	Query(selector string) (*common.ElementHandle, error)
	QueryAll(selector string) ([]*common.ElementHandle, error)
	Page() *common.Page
	ParentFrame() *common.Frame
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
	WaitForNavigation(opts goja.Value) (*common.Response, error)
	WaitForSelector(selector string, opts goja.Value) (*common.ElementHandle, error)
	WaitForTimeout(timeout int64)
}

// elementHandleAPI is the interface of an in-page DOM element.
type elementHandleAPI interface {
	common.JSHandleAPI

	BoundingBox() *common.RectAPI
	Check(opts goja.Value)
	Click(opts goja.Value) error
	ContentFrame() (*common.Frame, error)
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
	OwnerFrame() (*common.Frame, error)
	Press(key string, opts goja.Value)
	Query(selector string) (*common.ElementHandle, error)
	QueryAll(selector string) ([]*common.ElementHandle, error)
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
	WaitForSelector(selector string, opts goja.Value) (*common.ElementHandle, error)
}

// requestAPI is the interface of an HTTP request.
type requestAPI interface {
	AllHeaders() map[string]string
	Failure() goja.Value
	Frame() *common.Frame
	HeaderValue(string) goja.Value
	Headers() map[string]string
	HeadersArray() []common.HTTPHeader
	IsNavigationRequest() bool
	Method() string
	PostData() string
	PostDataBuffer() goja.ArrayBuffer
	PostDataJSON() string
	RedirectedFrom() requestAPI
	RedirectedTo() requestAPI
	ResourceType() string
	Response() *common.Response
	Size() common.HTTPMessageSize
	Timing() goja.Value
	URL() string
}

// responseAPI is the interface of an HTTP response.
type responseAPI interface {
	AllHeaders() map[string]string
	Body() goja.ArrayBuffer
	Finished() bool
	Frame() *common.Frame
	HeaderValue(string) goja.Value
	HeaderValues(string) []string
	Headers() map[string]string
	HeadersArray() []common.HTTPHeader
	JSON() goja.Value
	Ok() bool
	Request() *common.Request
	SecurityDetails() goja.Value
	ServerAddr() goja.Value
	Size() common.HTTPMessageSize
	Status() int64
	StatusText() string
	URL() string
}

// Strict mode:
// All operations on locators throw an exception if more
// than one element matches the locator's selector.
//
// See Issue #100 for more details.

// locatorAPI represents a way to find element(s) on a page at any moment.
type locatorAPI interface {
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

// keyboardAPI is the interface of a keyboard input device.
// TODO: map this to page.GetKeyboard().
type keyboardAPI interface { //nolint: unused
	Down(key string)
	InsertText(char string)
	Press(key string, opts goja.Value)
	Type(text string, opts goja.Value)
	Up(key string)
}

// touchscreenAPI is the interface of a touchscreen.
// TODO: map this to page.GetTouchscreen().
type touchscreenAPI interface { //nolint: unused
	Tap(x float64, y float64)
}

// cdpSessionAPI is the interface of a raw CDP session.
type cdpSessionAPI interface { //nolint: unused
	Detach()
	Send(method string, params goja.Value) goja.Value
}

// mouseAPI is the interface of a mouse input device.
// TODO: map this to page.GetMouse().
type mouseAPI interface { //nolint: unused
	Click(x float64, y float64, opts goja.Value)
	DblClick(x float64, y float64, opts goja.Value)
	Down(x float64, y float64, opts goja.Value)
	Move(x float64, y float64, opts goja.Value)
	Up(x float64, y float64, opts goja.Value)
	// Wheel(opts goja.Value)
}

// videoAPI is the interface of a recorded video.
type videoAPI interface { //nolint: unused
	Path() string
}

// routeAPI is the interface of a route for managing request interception.
type routeAPI interface { //nolint: unused
	Abort(errorCode string)
	Continue(opts goja.Value)
	Fulfill(opts goja.Value)
	Request() *common.Request
}

// workerAPI is the interface of a web worker.
type workerAPI interface {
	Evaluate(pageFunc goja.Value, args ...goja.Value) any
	EvaluateHandle(pageFunc goja.Value, args ...goja.Value) (common.JSHandleAPI, error)
	URL() string
}
