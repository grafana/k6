package browser

import (
	"fmt"

	"github.com/grafana/sobek"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/common"
)

// browserProvider is a function that provides a browser instance.
type browserProvider func() (*common.Browser, error)

// mapBrowser to the JS module API using the provider to get the browser instance.
// This lets the same mapping serve both the managed browser and a browser
// obtained via chromium.connectOverCDP, since they share the same API.
//
//nolint:gocognit,funlen
func mapBrowser(vu moduleVU, browser browserProvider) mapping {
	m := mapping{
		"context": func() (mapping, error) {
			b, err := browser()
			if err != nil {
				return nil, err
			}
			return mapBrowserContext(vu, b.Context()), nil
		},
		"closeContext": func() *sobek.Promise {
			return promise(vu, func() (any, error) {
				b, err := browser()
				if err != nil {
					return nil, err
				}
				return nil, b.CloseContext() //nolint:wrapcheck
			})
		},
		"isConnected": func() (bool, error) {
			b, err := browser()
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
			return promise(vu, func() (any, error) {
				b, err := browser()
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
			b, err := browser()
			if err != nil {
				return "", err
			}
			return b.UserAgent(), nil
		},
		"version": func() (string, error) {
			b, err := browser()
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
			return promise(vu, func() (any, error) {
				b, err := browser()
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

	addUserManagedClose(vu, m, browser)

	return m
}

// addUserManagedClose adds a close() method to the browser mapping when the
// browser is user-managed (created via chromium.connectOverCDP). close()
// untracks the browser from the registry (so the IterEnd/Exit sweep does not
// double-close) and closes it. Managed browsers don't get a close(): the
// registry owns their lifecycle.
func addUserManagedClose(vu moduleVU, m mapping, browser browserProvider) {
	// In the init context (e.g. the module-level browser export) there's no
	// state and no iteration browser to resolve; the managed browser never
	// exposes close() anyway.
	if vu.State() == nil {
		return
	}
	b, err := browser()
	if err != nil {
		return
	}
	iter, ok := vu.userManagedIter(b)
	if !ok {
		return
	}
	m["close"] = func() *sobek.Promise {
		return promise(vu, func() (any, error) {
			vu.untrackUserManagedBrowser(iter, b)
			b.Close()
			return nil, nil
		})
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
	if err := b.Proxy.Validate(); err != nil {
		return nil, err
	}
	return b, nil
}
