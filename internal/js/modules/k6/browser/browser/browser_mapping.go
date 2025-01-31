package browser

import (
	"fmt"

	"github.com/grafana/sobek"

	"go.k6.io/k6/internal/js/modules/k6/browser/common"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext"
)

// mapBrowser to the JS module.
//
//nolint:gocognit
func mapBrowser(vu moduleVU) mapping {
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
			popts, err := parseBrowserContextOptions(vu.Runtime(), opts)
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
			popts, err := parseBrowserContextOptions(vu.Runtime(), opts)
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
func parseBrowserContextOptions(rt *sobek.Runtime, opts sobek.Value) (*common.BrowserContextOptions, error) {
	b := common.DefaultBrowserContextOptions()
	if err := mergeWith(rt, b, opts); err != nil {
		return nil, err
	}
	return b, nil
}
