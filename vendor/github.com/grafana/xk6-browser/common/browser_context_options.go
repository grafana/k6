package common

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/grafana/xk6-browser/k6ext"

	"github.com/dop251/goja"
)

// Geolocation represents a geolocation.
type Geolocation struct {
	Latitude  float64 `js:"latitude"`
	Longitude float64 `js:"longitude"`
	Accurracy float64 `js:"accurracy"`
}

// NewGeolocation creates a new instance of Geolocation.
func NewGeolocation() *Geolocation {
	return &Geolocation{}
}

// Parse parses the geolocation options.
func (g *Geolocation) Parse(ctx context.Context, opts goja.Value) error { //nolint:cyclop
	rt := k6ext.Runtime(ctx)
	longitude := 0.0
	latitude := 0.0
	accuracy := 0.0

	if opts != nil && !goja.IsUndefined(opts) && !goja.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "accuracy":
				accuracy = opts.Get(k).ToFloat()
			case "latitude":
				latitude = opts.Get(k).ToFloat()
			case "longitude":
				longitude = opts.Get(k).ToFloat()
			}
		}
	}

	if longitude < -180 || longitude > 180 {
		return fmt.Errorf(`invalid longitude "%.2f": precondition -180 <= LONGITUDE <= 180 failed`, longitude)
	}
	if latitude < -90 || latitude > 90 {
		return fmt.Errorf(`invalid latitude "%.2f": precondition -90 <= LATITUDE <= 90 failed`, latitude)
	}
	if accuracy < 0 {
		return fmt.Errorf(`invalid accuracy "%.2f": precondition 0 <= ACCURACY failed`, accuracy)
	}

	g.Accurracy = accuracy
	g.Latitude = latitude
	g.Longitude = longitude

	return nil
}

// BrowserContextOptions stores browser context options.
type BrowserContextOptions struct {
	AcceptDownloads   bool              `js:"acceptDownloads"`
	BypassCSP         bool              `js:"bypassCSP"`
	ColorScheme       ColorScheme       `js:"colorScheme"`
	DeviceScaleFactor float64           `js:"deviceScaleFactor"`
	ExtraHTTPHeaders  map[string]string `js:"extraHTTPHeaders"`
	Geolocation       *Geolocation      `js:"geolocation"`
	HasTouch          bool              `js:"hasTouch"`
	HttpCredentials   *Credentials      `js:"httpCredentials"`
	IgnoreHTTPSErrors bool              `js:"ignoreHTTPSErrors"`
	IsMobile          bool              `js:"isMobile"`
	JavaScriptEnabled bool              `js:"javaScriptEnabled"`
	Locale            string            `js:"locale"`
	Offline           bool              `js:"offline"`
	Permissions       []string          `js:"permissions"`
	ReducedMotion     ReducedMotion     `js:"reducedMotion"`
	Screen            *Screen           `js:"screen"`
	TimezoneID        string            `js:"timezoneID"`
	UserAgent         string            `js:"userAgent"`
	VideosPath        string            `js:"videosPath"`
	Viewport          *Viewport         `js:"viewport"`
}

// NewBrowserContextOptions creates a default set of browser context options.
func NewBrowserContextOptions() *BrowserContextOptions {
	return &BrowserContextOptions{
		ColorScheme:       ColorSchemeLight,
		DeviceScaleFactor: 1.0,
		ExtraHTTPHeaders:  make(map[string]string),
		JavaScriptEnabled: true,
		Locale:            DefaultLocale,
		Permissions:       []string{},
		ReducedMotion:     ReducedMotionNoPreference,
		Screen:            &Screen{Width: DefaultScreenWidth, Height: DefaultScreenHeight},
		Viewport:          &Viewport{Width: DefaultScreenWidth, Height: DefaultScreenHeight},
	}
}

func (b *BrowserContextOptions) Parse(ctx context.Context, opts goja.Value) error {
	rt := k6ext.Runtime(ctx)
	if opts != nil && !goja.IsUndefined(opts) && !goja.IsNull(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			switch k {
			case "acceptDownloads":
				b.AcceptDownloads = opts.Get(k).ToBoolean()
			case "bypassCSP":
				b.BypassCSP = opts.Get(k).ToBoolean()
			case "colorScheme":
				switch ColorScheme(opts.Get(k).String()) {
				case "light":
					b.ColorScheme = ColorSchemeLight
				case "dark":
					b.ColorScheme = ColorSchemeDark
				default:
					b.ColorScheme = ColorSchemeNoPreference
				}
			case "deviceScaleFactor":
				b.DeviceScaleFactor = opts.Get(k).ToFloat()
			case "extraHTTPHeaders":
				headers := opts.Get(k).ToObject(rt)
				for _, k := range headers.Keys() {
					b.ExtraHTTPHeaders[k] = headers.Get(k).String()
				}
			case "geolocation":
				geolocation := NewGeolocation()
				if err := geolocation.Parse(ctx, opts.Get(k).ToObject(rt)); err != nil {
					return err
				}
				b.Geolocation = geolocation
			case "hasTouch":
				b.HasTouch = opts.Get(k).ToBoolean()
			case "httpCredentials":
				credentials := NewCredentials()
				if err := credentials.Parse(ctx, opts.Get(k).ToObject(rt)); err != nil {
					return err
				}
				b.HttpCredentials = credentials
			case "ignoreHTTPSErrors":
				b.IgnoreHTTPSErrors = opts.Get(k).ToBoolean()
			case "isMobile":
				b.IsMobile = opts.Get(k).ToBoolean()
			case "javaScriptEnabled":
				b.JavaScriptEnabled = opts.Get(k).ToBoolean()
			case "locale":
				b.Locale = opts.Get(k).String()
			case "offline":
				b.Offline = opts.Get(k).ToBoolean()
			case "permissions":
				if ps, ok := opts.Get(k).Export().([]any); ok {
					for _, p := range ps {
						b.Permissions = append(b.Permissions, fmt.Sprintf("%v", p))
					}
				}
			case "reducedMotion":
				switch ReducedMotion(opts.Get(k).String()) {
				case "reduce":
					b.ReducedMotion = ReducedMotionReduce
				default:
					b.ReducedMotion = ReducedMotionNoPreference
				}
			case "screen":
				screen := &Screen{}
				if err := screen.Parse(ctx, opts.Get(k).ToObject(rt)); err != nil {
					return err
				}
				b.Screen = screen
			case "timezoneID":
				b.TimezoneID = opts.Get(k).String()
			case "userAgent":
				b.UserAgent = opts.Get(k).String()
			case "viewport":
				viewport := &Viewport{}
				if err := viewport.Parse(ctx, opts.Get(k).ToObject(rt)); err != nil {
					return err
				}
				b.Viewport = viewport
			}
		}
	}
	return nil
}

// WaitForEventOptions are the options used by the browserContext.waitForEvent API.
type WaitForEventOptions struct {
	Timeout     time.Duration
	PredicateFn goja.Callable
}

// NewWaitForEventOptions created a new instance of WaitForEventOptions with a
// default timeout.
func NewWaitForEventOptions(defaultTimeout time.Duration) *WaitForEventOptions {
	return &WaitForEventOptions{
		Timeout: defaultTimeout,
	}
}

// Parse will parse the options or a callable predicate function. It can parse
// only a callable predicate function or an object which contains a callable
// predicate function and a timeout.
func (w *WaitForEventOptions) Parse(ctx context.Context, optsOrPredicate goja.Value) error {
	if !gojaValueExists(optsOrPredicate) {
		return nil
	}

	var (
		isCallable bool
		rt         = k6ext.Runtime(ctx)
	)

	w.PredicateFn, isCallable = goja.AssertFunction(optsOrPredicate)
	if isCallable {
		return nil
	}

	opts := optsOrPredicate.ToObject(rt)
	for _, k := range opts.Keys() {
		switch k {
		case "predicate":
			w.PredicateFn, isCallable = goja.AssertFunction(opts.Get(k))
			if !isCallable {
				return errors.New("predicate function is not callable")
			}
		case "timeout": //nolint:goconst
			w.Timeout = time.Duration(opts.Get(k).ToInteger()) * time.Millisecond
		}
	}

	return nil
}

// GrantPermissionsOptions is used by BrowserContext.GrantPermissions.
type GrantPermissionsOptions struct {
	Origin string
}

// NewGrantPermissionsOptions returns a new GrantPermissionsOptions.
func NewGrantPermissionsOptions() *GrantPermissionsOptions {
	return &GrantPermissionsOptions{}
}

// Parse parses the options from opts if opts exists in the Goja runtime.
func (g *GrantPermissionsOptions) Parse(ctx context.Context, opts goja.Value) {
	rt := k6ext.Runtime(ctx)

	if gojaValueExists(opts) {
		opts := opts.ToObject(rt)
		for _, k := range opts.Keys() {
			if k == "origin" {
				g.Origin = opts.Get(k).String()
				break
			}
		}
	}
}
