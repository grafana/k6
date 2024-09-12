package browser

import (
	"fmt"

	"github.com/grafana/sobek"

	"github.com/grafana/xk6-browser/common"
)

// syncMapBrowser is like mapBrowser but returns synchronous functions.
func syncMapBrowser(vu moduleVU) mapping { //nolint:funlen,cyclop
	rt := vu.Runtime()
	return mapping{
		"context": func() (mapping, error) {
			b, err := vu.browser()
			if err != nil {
				return nil, err
			}
			return syncMapBrowserContext(vu, b.Context()), nil
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
		"newContext": func(opts sobek.Value) (*sobek.Object, error) {
			popts := common.NewBrowserContextOptions()
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing browser.newContext options: %w", err)
			}

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

			m := syncMapBrowserContext(vu, bctx)

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
		"newPage": func(opts sobek.Value) (mapping, error) {
			popts := common.NewBrowserContextOptions()
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing browser.newContext options: %w", err)
			}

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

			return syncMapPage(vu, page), nil
		},
	}
}
