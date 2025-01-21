package browser

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/internal/js/modules/k6/browser/common"

	k6common "go.k6.io/k6/js/common"
	k6modulestest "go.k6.io/k6/js/modulestest"
	k6lib "go.k6.io/k6/lib"
	k6metrics "go.k6.io/k6/metrics"
)

// customMappings is a list of custom mappings for our module API.
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
		// See: https://go.k6.io/k6/js/modules/k6/browser/issues/913
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
			RuntimeField: sobek.New(),
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

			// sobek uses methods that starts with lowercase.
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
			apiInterface: (*consoleMessageAPI)(nil),
			mapp: func() mapping {
				return mapConsoleMessage(moduleVU{VU: vu}, common.PageOnEvent{
					ConsoleMessage: &common.ConsoleMessage{},
				})
			},
		},
		"mapMetricEvent": {
			apiInterface: (*metricEventAPI)(nil),
			mapp: func() mapping {
				return mapMetricEvent(moduleVU{VU: vu}, common.PageOnEvent{
					Metric: &common.MetricEvent{},
				})
			},
		},
		"mapTouchscreen": {
			apiInterface: (*touchscreenAPI)(nil),
			mapp: func() mapping {
				return mapTouchscreen(moduleVU{VU: vu}, &common.Touchscreen{})
			},
		},
		"mapKeyboard": {
			apiInterface: (*keyboardAPI)(nil),
			mapp: func() mapping {
				return mapKeyboard(moduleVU{VU: vu}, &common.Keyboard{})
			},
		},
		"mapMouse": {
			apiInterface: (*mouseAPI)(nil),
			mapp: func() mapping {
				return mapMouse(moduleVU{VU: vu}, &common.Mouse{})
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
	CloseContext()
	IsConnected() bool
	NewContext(opts *common.BrowserContextOptions) (*common.BrowserContext, error)
	NewPage(opts *common.BrowserContextOptions) (*common.Page, error)
	On(string) (bool, error)
	UserAgent() string
	Version() string
}

// browserContextAPI is the public interface of a CDP browser context.
type browserContextAPI interface { //nolint:interfacebloat
	AddCookies(cookies []*common.Cookie) error
	AddInitScript(script sobek.Value, arg sobek.Value) error
	Browser() *common.Browser
	ClearCookies() error
	ClearPermissions() error
	Close() error
	Cookies(urls ...string) ([]*common.Cookie, error)
	GrantPermissions(permissions []string, opts sobek.Value) error
	NewPage() (*common.Page, error)
	Pages() []*common.Page
	SetDefaultNavigationTimeout(timeout int64)
	SetDefaultTimeout(timeout int64)
	SetGeolocation(geolocation *common.Geolocation) error
	SetHTTPCredentials(httpCredentials common.Credentials) error
	SetOffline(offline bool) error
	WaitForEvent(event string, optsOrPredicate sobek.Value) (any, error)
}

// pageAPI is the interface of a single browser tab.
type pageAPI interface { //nolint:interfacebloat
	BringToFront() error
	Check(selector string, opts sobek.Value) error
	Click(selector string, opts sobek.Value) error
	Close(opts sobek.Value) error
	Content() (string, error)
	Context() *common.BrowserContext
	Dblclick(selector string, opts sobek.Value) error
	DispatchEvent(selector string, typ string, eventInit sobek.Value, opts sobek.Value)
	EmulateMedia(opts sobek.Value) error
	EmulateVisionDeficiency(typ string) error
	Evaluate(pageFunc sobek.Value, arg ...sobek.Value) (any, error)
	EvaluateHandle(pageFunc sobek.Value, arg ...sobek.Value) (common.JSHandleAPI, error)
	Fill(selector string, value string, opts sobek.Value) error
	Focus(selector string, opts sobek.Value) error
	Frames() []*common.Frame
	GetAttribute(selector string, name string, opts sobek.Value) (string, bool, error)
	GetKeyboard() *common.Keyboard
	GetMouse() *common.Mouse
	GetTouchscreen() *common.Touchscreen
	Goto(url string, opts sobek.Value) (*common.Response, error)
	Hover(selector string, opts sobek.Value) error
	InnerHTML(selector string, opts sobek.Value) (string, error)
	InnerText(selector string, opts sobek.Value) (string, error)
	InputValue(selector string, opts sobek.Value) (string, error)
	IsChecked(selector string, opts sobek.Value) (bool, error)
	IsClosed() bool
	IsDisabled(selector string, opts sobek.Value) (bool, error)
	IsEditable(selector string, opts sobek.Value) (bool, error)
	IsEnabled(selector string, opts sobek.Value) (bool, error)
	IsHidden(selector string, opts sobek.Value) (bool, error)
	IsVisible(selector string, opts sobek.Value) (bool, error)
	Locator(selector string, opts sobek.Value) *common.Locator
	MainFrame() *common.Frame
	On(event common.PageOnEventName, handler func(common.PageOnEvent) error) error
	Opener() pageAPI
	Press(selector string, key string, opts sobek.Value) error
	Query(selector string) (*common.ElementHandle, error)
	QueryAll(selector string) ([]*common.ElementHandle, error)
	Reload(opts sobek.Value) *common.Response
	Screenshot(opts sobek.Value) ([]byte, error)
	SelectOption(selector string, values sobek.Value, opts sobek.Value) ([]string, error)
	SetChecked(selector string, checked bool, opts sobek.Value) error
	SetContent(html string, opts sobek.Value) error
	SetDefaultNavigationTimeout(timeout int64)
	SetDefaultTimeout(timeout int64)
	SetExtraHTTPHeaders(headers map[string]string) error
	SetInputFiles(selector string, files sobek.Value, opts sobek.Value) error
	SetViewportSize(viewportSize sobek.Value) error
	Tap(selector string, opts sobek.Value) error
	TextContent(selector string, opts sobek.Value) (string, bool, error)
	ThrottleCPU(common.CPUProfile) error
	ThrottleNetwork(common.NetworkProfile) error
	Title() (string, error)
	Type(selector string, text string, opts sobek.Value) error
	Uncheck(selector string, opts sobek.Value) error
	URL() (string, error)
	ViewportSize() map[string]float64
	WaitForFunction(fn, opts sobek.Value, args ...sobek.Value) (any, error)
	WaitForLoadState(state string, opts sobek.Value) error
	WaitForNavigation(opts sobek.Value) (*common.Response, error)
	WaitForSelector(selector string, opts sobek.Value) (*common.ElementHandle, error)
	WaitForTimeout(timeout int64)
	Workers() []*common.Worker
}

// consoleMessageAPI is the interface of a console message.
type consoleMessageAPI interface {
	Args() []common.JSHandleAPI
	Page() *common.Page
	Text() string
	Type() string
}

// metricEventAPI is the interface of a metric event.
type metricEventAPI interface {
	Tag(matchesRegex common.K6BrowserCheckRegEx, patterns common.TagMatches) error
}

// frameAPI is the interface of a CDP target frame.
type frameAPI interface { //nolint:interfacebloat
	Check(selector string, opts sobek.Value) error
	ChildFrames() []*common.Frame
	Click(selector string, opts sobek.Value) error
	Content() (string, error)
	Dblclick(selector string, opts sobek.Value) error
	DispatchEvent(selector string, typ string, eventInit sobek.Value, opts sobek.Value) error
	// EvaluateWithContext for internal use only
	EvaluateWithContext(ctx context.Context, pageFunc sobek.Value, args ...sobek.Value) (any, error)
	Evaluate(pageFunc sobek.Value, args ...sobek.Value) (any, error)
	EvaluateHandle(pageFunc sobek.Value, args ...sobek.Value) (common.JSHandleAPI, error)
	Fill(selector string, value string, opts sobek.Value) error
	Focus(selector string, opts sobek.Value) error
	FrameElement() (*common.ElementHandle, error)
	GetAttribute(selector string, name string, opts sobek.Value) (string, bool, error)
	Goto(url string, opts sobek.Value) (*common.Response, error)
	Hover(selector string, opts sobek.Value) error
	InnerHTML(selector string, opts sobek.Value) (string, error)
	InnerText(selector string, opts sobek.Value) (string, error)
	InputValue(selector string, opts sobek.Value) (string, error)
	IsChecked(selector string, opts sobek.Value) (bool, error)
	IsDetached() bool
	IsDisabled(selector string, opts sobek.Value) (bool, error)
	IsEditable(selector string, opts sobek.Value) (bool, error)
	IsEnabled(selector string, opts sobek.Value) (bool, error)
	IsHidden(selector string, opts sobek.Value) (bool, error)
	IsVisible(selector string, opts sobek.Value) (bool, error)
	ID() string
	LoaderID() string
	Locator(selector string, opts sobek.Value) *common.Locator
	Name() string
	Query(selector string) (*common.ElementHandle, error)
	QueryAll(selector string) ([]*common.ElementHandle, error)
	Page() *common.Page
	ParentFrame() *common.Frame
	Press(selector string, key string, opts sobek.Value) error
	SelectOption(selector string, values sobek.Value, opts sobek.Value) ([]string, error)
	SetChecked(selector string, checked bool, opts sobek.Value) error
	SetContent(html string, opts sobek.Value) error
	SetInputFiles(selector string, files sobek.Value, opts sobek.Value)
	Tap(selector string, opts sobek.Value) error
	TextContent(selector string, opts sobek.Value) (string, bool, error)
	Title() string
	Type(selector string, text string, opts sobek.Value) error
	Uncheck(selector string, opts sobek.Value) error
	URL() string
	WaitForFunction(pageFunc, opts sobek.Value, args ...sobek.Value) (any, error)
	WaitForLoadState(state string, opts sobek.Value) error
	WaitForNavigation(opts sobek.Value) (*common.Response, error)
	WaitForSelector(selector string, opts sobek.Value) (*common.ElementHandle, error)
	WaitForTimeout(timeout int64)
}

// elementHandleAPI is the interface of an in-page DOM element.
type elementHandleAPI interface { //nolint:interfacebloat
	common.JSHandleAPI

	BoundingBox() (*common.Rect, error)
	Check(opts sobek.Value) error
	Click(opts sobek.Value) error
	ContentFrame() (*common.Frame, error)
	Dblclick(opts sobek.Value) error
	DispatchEvent(typ string, props sobek.Value) error
	Fill(value string, opts sobek.Value) error
	Focus() error
	GetAttribute(name string) (string, bool, error)
	Hover(opts sobek.Value) error
	InnerHTML() (string, error)
	InnerText() (string, error)
	InputValue(opts sobek.Value) (string, error)
	IsChecked() (bool, error)
	IsDisabled() (bool, error)
	IsEditable() (bool, error)
	IsEnabled() (bool, error)
	IsHidden() (bool, error)
	IsVisible() (bool, error)
	OwnerFrame() (*common.Frame, error)
	Press(key string, opts sobek.Value) error
	Query(selector string) (*common.ElementHandle, error)
	QueryAll(selector string) ([]*common.ElementHandle, error)
	Screenshot(opts sobek.Value) (sobek.ArrayBuffer, error)
	ScrollIntoViewIfNeeded(opts sobek.Value) error
	SelectOption(values sobek.Value, opts sobek.Value) ([]string, error)
	SelectText(opts sobek.Value) error
	SetChecked(checked bool, opts sobek.Value) error
	SetInputFiles(files sobek.Value, opts sobek.Value) error
	Tap(opts sobek.Value) error
	TextContent() (string, bool, error)
	Type(text string, opts sobek.Value) error
	Uncheck(opts sobek.Value) error
	WaitForElementState(state string, opts sobek.Value) error
	WaitForSelector(selector string, opts sobek.Value) (*common.ElementHandle, error)
}

// requestAPI is the interface of an HTTP request.
type requestAPI interface { //nolint:interfacebloat
	AllHeaders() map[string]string
	Frame() *common.Frame
	HeaderValue(string) sobek.Value
	Headers() map[string]string
	HeadersArray() []common.HTTPHeader
	IsNavigationRequest() bool
	Method() string
	PostData() string
	PostDataBuffer() sobek.ArrayBuffer
	ResourceType() string
	Response() *common.Response
	Size() common.HTTPMessageSize
	Timing() sobek.Value
	URL() string
}

// responseAPI is the interface of an HTTP response.
type responseAPI interface { //nolint:interfacebloat
	AllHeaders() map[string]string
	Body() ([]byte, error)
	Frame() *common.Frame
	HeaderValue(string) (string, bool)
	HeaderValues(string) []string
	Headers() map[string]string
	HeadersArray() []common.HTTPHeader
	JSON() (any, error)
	Ok() bool
	Request() *common.Request
	SecurityDetails() *common.SecurityDetails
	ServerAddr() *common.RemoteAddress
	Size() common.HTTPMessageSize
	Status() int64
	StatusText() string
	URL() string
	Text() (string, error)
}

// locatorAPI represents a way to find element(s) on a page at any moment.
type locatorAPI interface { //nolint:interfacebloat
	Clear(opts *common.FrameFillOptions) error
	Click(opts sobek.Value) error
	Dblclick(opts sobek.Value) error
	SetChecked(checked bool, opts sobek.Value) error
	Check(opts sobek.Value) error
	Uncheck(opts sobek.Value) error
	IsChecked(opts sobek.Value) (bool, error)
	IsEditable(opts sobek.Value) (bool, error)
	IsEnabled(opts sobek.Value) (bool, error)
	IsDisabled(opts sobek.Value) (bool, error)
	IsVisible(opts sobek.Value) (bool, error)
	IsHidden(opts sobek.Value) (bool, error)
	Fill(value string, opts sobek.Value) error
	Focus(opts sobek.Value) error
	GetAttribute(name string, opts sobek.Value) (string, bool, error)
	InnerHTML(opts sobek.Value) (string, error)
	InnerText(opts sobek.Value) (string, error)
	TextContent(opts sobek.Value) (string, bool, error)
	InputValue(opts sobek.Value) (string, error)
	SelectOption(values sobek.Value, opts sobek.Value) ([]string, error)
	Press(key string, opts sobek.Value) error
	Type(text string, opts sobek.Value) error
	Hover(opts sobek.Value) error
	Tap(opts sobek.Value) error
	DispatchEvent(typ string, eventInit, opts sobek.Value)
	WaitFor(opts sobek.Value) error
}

// keyboardAPI is the interface of a keyboard input device.
type keyboardAPI interface {
	Down(key string) error
	Up(key string) error
	InsertText(char string) error
	Press(key string, opts sobek.Value) error
	Type(text string, opts sobek.Value) error
}

// touchscreenAPI is the interface of a touchscreen.
type touchscreenAPI interface {
	Tap(x float64, y float64) error
}

// mouseAPI is the interface of a mouse input device.
type mouseAPI interface {
	Click(x float64, y float64, opts sobek.Value) error
	DblClick(x float64, y float64, opts sobek.Value) error
	Down(opts sobek.Value) error
	Up(opts sobek.Value) error
	Move(x float64, y float64, opts sobek.Value) error
}

// workerAPI is the interface of a web worker.
type workerAPI interface {
	URL() string
}
