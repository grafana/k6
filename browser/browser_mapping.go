package browser

import (
	"fmt"

	"github.com/dop251/goja"

	"github.com/grafana/xk6-browser/common"
)

// mapBrowser to the JS module.
func mapBrowser(vu moduleVU) mapping { //nolint:funlen,cyclop
	rt := vu.Runtime()
	return mapping{
		"context": func() (*common.BrowserContext, error) {
			b, err := vu.browser()
			if err != nil {
				return nil, err
			}
			return b.Context(), nil
		},
		"closeContext": func() error {
			b, err := vu.browser()
			if err != nil {
				return err
			}
			return b.CloseContext() //nolint:wrapcheck
		},
		"isConnected": func() (bool, error) {
			b, err := vu.browser()
			if err != nil {
				return false, err
			}
			return b.IsConnected(), nil
		},
		"newContext": func(opts goja.Value) (*goja.Object, error) {
			b, err := vu.browser()
			if err != nil {
				return nil, err
			}
			bctx, err := b.NewContext(opts)
			if err != nil {
				return nil, err //nolint:wrapcheck
			}

			if err := initBrowserContext(bctx, vu.testRunID); err != nil {
				return nil, err
			}

			m := mapBrowserContext(vu, bctx)

			return rt.ToValue(m).ToObject(rt), nil
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
		"newPage": func(opts goja.Value) (mapping, error) {
			b, err := vu.browser()
			if err != nil {
				return nil, err
			}
			page, err := b.NewPage(opts)
			if err != nil {
				return nil, err //nolint:wrapcheck
			}

			if err := initBrowserContext(b.Context(), vu.testRunID); err != nil {
				return nil, err
			}

			return mapPage(vu, page), nil
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
