package browser

import (
	"context"
	"fmt"

	"github.com/grafana/sobek"

	"github.com/grafana/xk6-browser/common"
	"github.com/grafana/xk6-browser/k6ext"
)

// mapBrowser to the JS module.
func mapBrowser(vu moduleVU) mapping { //nolint:funlen,cyclop,gocognit
	return mapping{
		"context": func() (mapping, error) {
			b, err := vu.browser()
			if err != nil {
				return nil, err
			}
			return mapBrowserContext(vu, b.Context()), nil
		},
		"closeContext": func() *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				b, err := vu.browser()
				if err != nil {
					return nil, err
				}
				return nil, b.CloseContext() //nolint:wrapcheck
			})
		},
		"isConnected": func() (bool, error) {
			b, err := vu.browser()
			if err != nil {
				return false, err
			}
			return b.IsConnected(), nil
		},
		"newContext": func(opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parseBrowserContextOptions(vu.Context(), opts)
			if err != nil {
				return nil, fmt.Errorf("parsing browser.newContext options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				b, err := vu.browser()
				if err != nil {
					return nil, err
				}
				bctx, err := b.NewContext(popts)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				if err := initBrowserContext(bctx, vu.testRunID); err != nil {
					return nil, err
				}

				return mapBrowserContext(vu, bctx), nil
			}), nil
		},
		"userAgent": func() (string, error) {
			b, err := vu.browser()
			if err != nil {
				return "", err
			}
			return b.UserAgent(), nil
		},
		"version": func() (string, error) {
			b, err := vu.browser()
			if err != nil {
				return "", err
			}
			return b.Version(), nil
		},
		"newPage": func(opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parseBrowserContextOptions(vu.Context(), opts)
			if err != nil {
				return nil, fmt.Errorf("parsing browser.newPage options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				b, err := vu.browser()
				if err != nil {
					return nil, err
				}
				page, err := b.NewPage(popts)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				if err := initBrowserContext(b.Context(), vu.testRunID); err != nil {
					return nil, err
				}

				return mapPage(vu, page), nil
			}), nil
		},
	}
}

func initBrowserContext(bctx *common.BrowserContext, testRunID string) error {
	// Setting a k6 object which will contain k6 specific metadata
	// on the current test run. This allows external applications
	// (such as Grafana Faro) to identify that the session is a k6
	// automated one and not one driven by a real person.
	if err := bctx.AddInitScript(
		fmt.Sprintf(`window.k6 = { testRunId: %q }`, testRunID),
	); err != nil {
		return fmt.Errorf("adding k6 object to new browser context: %w", err)
	}

	return nil
}

// parseBrowserContextOptions parses the [common.BrowserContext] options from a Sobek value.
func parseBrowserContextOptions(ctx context.Context, opts sobek.Value) (*common.BrowserContextOptions, error) { //nolint:cyclop,funlen,gocognit,lll
	if !sobekValueExists(opts) {
		return nil, nil //nolint:nilnil
	}

	b := common.NewBrowserContextOptions()

	rt := k6ext.Runtime(ctx)
	o := opts.ToObject(rt)
	for _, k := range o.Keys() {
		switch k {
		case "acceptDownloads":
			b.AcceptDownloads = o.Get(k).ToBoolean()
		case "downloadsPath":
			b.DownloadsPath = o.Get(k).String()
		case "bypassCSP":
			b.BypassCSP = o.Get(k).ToBoolean()
		case "colorScheme":
			switch common.ColorScheme(o.Get(k).String()) { //nolint:exhaustive
			case "light":
				b.ColorScheme = common.ColorSchemeLight
			case "dark":
				b.ColorScheme = common.ColorSchemeDark
			default:
				b.ColorScheme = common.ColorSchemeNoPreference
			}
		case "deviceScaleFactor":
			b.DeviceScaleFactor = o.Get(k).ToFloat()
		case "extraHTTPHeaders":
			headers := o.Get(k).ToObject(rt)
			for _, k := range headers.Keys() {
				b.ExtraHTTPHeaders[k] = headers.Get(k).String()
			}
		case "geolocation":
			gl, err := exportTo[*common.Geolocation](rt, o.Get(k))
			if err != nil {
				return nil, fmt.Errorf("parsing geolocation options: %w", err)
			}
			b.Geolocation = gl
		case "hasTouch":
			b.HasTouch = o.Get(k).ToBoolean()
		case "httpCredentials":
			var err error
			b.HTTPCredentials, err = exportTo[common.Credentials](rt, o.Get(k))
			if err != nil {
				return nil, fmt.Errorf("parsing HTTP credential options: %w", err)
			}
		case "ignoreHTTPSErrors":
			b.IgnoreHTTPSErrors = o.Get(k).ToBoolean()
		case "isMobile":
			b.IsMobile = o.Get(k).ToBoolean()
		case "javaScriptEnabled":
			b.JavaScriptEnabled = o.Get(k).ToBoolean()
		case "locale":
			b.Locale = o.Get(k).String()
		case "offline":
			b.Offline = o.Get(k).ToBoolean()
		case "permissions":
			var err error
			b.Permissions, err = exportTo[[]string](rt, o.Get(k))
			if err != nil {
				return nil, fmt.Errorf("parsing permissions options: %w", err)
			}
		case "reducedMotion":
			switch common.ReducedMotion(o.Get(k).String()) { //nolint:exhaustive
			case "reduce":
				b.ReducedMotion = common.ReducedMotionReduce
			default:
				b.ReducedMotion = common.ReducedMotionNoPreference
			}
		case "screen":
			var screen common.Screen
			if err := screen.Parse(ctx, o.Get(k).ToObject(rt)); err != nil {
				return nil, fmt.Errorf("parsing screen options: %w", err)
			}
			b.Screen = screen
		case "timezoneID":
			b.TimezoneID = o.Get(k).String()
		case "userAgent":
			b.UserAgent = o.Get(k).String()
		case "viewport":
			var viewport common.Viewport
			if err := viewport.Parse(ctx, o.Get(k).ToObject(rt)); err != nil {
				return nil, fmt.Errorf("parsing viewport options: %w", err)
			}
			b.Viewport = &viewport
		}
	}

	return b, nil
}
