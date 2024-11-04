package common

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/grafana/sobek"

	"github.com/grafana/xk6-browser/k6ext"
)

// Geolocation represents a geolocation.
type Geolocation struct {
	Latitude  float64 `js:"latitude"`
	Longitude float64 `js:"longitude"`
	Accuracy  float64 `js:"accuracy"`
}

// NewGeolocation creates a new instance of Geolocation.
func NewGeolocation() *Geolocation {
	return &Geolocation{}
}

// Parse parses the geolocation options.
func (g *Geolocation) Parse(ctx context.Context, sopts sobek.Value) error { //nolint:cyclop
	var newgl Geolocation

	if !sobekValueExists(sopts) {
		return fmt.Errorf("geolocation options are required")
	}

	opts := sopts.ToObject(k6ext.Runtime(ctx))
	for _, k := range opts.Keys() {
		switch k {
		case "accuracy":
			newgl.Accuracy = opts.Get(k).ToFloat()
		case "latitude":
			newgl.Latitude = opts.Get(k).ToFloat()
		case "longitude":
			newgl.Longitude = opts.Get(k).ToFloat()
		}
	}

	if newgl.Longitude < -180 || newgl.Longitude > 180 {
		return fmt.Errorf(`invalid longitude "%.2f": precondition -180 <= LONGITUDE <= 180 failed`, newgl.Longitude)
	}
	if newgl.Latitude < -90 || newgl.Latitude > 90 {
		return fmt.Errorf(`invalid latitude "%.2f": precondition -90 <= LATITUDE <= 90 failed`, newgl.Latitude)
	}
	if newgl.Accuracy < 0 {
		return fmt.Errorf(`invalid accuracy "%.2f": precondition 0 <= ACCURACY failed`, newgl.Accuracy)
	}

	*g = newgl

	return nil
}

// BrowserContextOptions stores browser context options.
type BrowserContextOptions struct {
	AcceptDownloads   bool              `js:"acceptDownloads"`
	DownloadsPath     string            `js:"downloadsPath"`
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

// WaitForEventOptions are the options used by the browserContext.waitForEvent API.
type WaitForEventOptions struct {
	Timeout     time.Duration
	PredicateFn sobek.Callable
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
func (w *WaitForEventOptions) Parse(ctx context.Context, optsOrPredicate sobek.Value) error {
	if !sobekValueExists(optsOrPredicate) {
		return nil
	}

	var (
		isCallable bool
		rt         = k6ext.Runtime(ctx)
	)

	w.PredicateFn, isCallable = sobek.AssertFunction(optsOrPredicate)
	if isCallable {
		return nil
	}

	opts := optsOrPredicate.ToObject(rt)
	for _, k := range opts.Keys() {
		switch k {
		case "predicate":
			w.PredicateFn, isCallable = sobek.AssertFunction(opts.Get(k))
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
