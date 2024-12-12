package browser

import (
	"fmt"
	"reflect"

	"github.com/grafana/sobek"

	"go.k6.io/k6/js/modules/k6/browser/common"
	"go.k6.io/k6/js/modules/k6/browser/k6error"
	"go.k6.io/k6/js/modules/k6/browser/k6ext"
)

// syncMapBrowserContext is like mapBrowserContext but returns synchronous functions.
func syncMapBrowserContext(vu moduleVU, bc *common.BrowserContext) mapping { //nolint:funlen,gocognit
	rt := vu.Runtime()
	return mapping{
		"addCookies": bc.AddCookies,
		"addInitScript": func(script sobek.Value) error {
			if !sobekValueExists(script) {
				return nil
			}

			source := ""
			switch script.ExportType() {
			case reflect.TypeOf(string("")):
				source = script.String()
			case reflect.TypeOf(sobek.Object{}):
				opts := script.ToObject(rt)
				for _, k := range opts.Keys() {
					if k == "content" {
						source = opts.Get(k).String()
					}
				}
			default:
				_, isCallable := sobek.AssertFunction(script)
				if !isCallable {
					source = fmt.Sprintf("(%s);", script.ToString().String())
				} else {
					source = fmt.Sprintf("(%s)(...args);", script.ToString().String())
				}
			}

			return bc.AddInitScript(source) //nolint:wrapcheck
		},
		"browser":          bc.Browser,
		"clearCookies":     bc.ClearCookies,
		"clearPermissions": bc.ClearPermissions,
		"close":            bc.Close,
		"cookies":          bc.Cookies,
		"grantPermissions": func(permissions []string, opts sobek.Value) error {
			popts, err := exportTo[common.GrantPermissionsOptions](vu.Runtime(), opts)
			if err != nil {
				return fmt.Errorf("parsing grant permission options: %w", err)
			}
			return bc.GrantPermissions(permissions, popts)
		},
		"setDefaultNavigationTimeout": bc.SetDefaultNavigationTimeout,
		"setDefaultTimeout":           bc.SetDefaultTimeout,
		"setGeolocation":              bc.SetGeolocation,
		"setHTTPCredentials":          bc.SetHTTPCredentials, //nolint:staticcheck
		"setOffline":                  bc.SetOffline,
		"waitForEvent": func(event string, optsOrPredicate sobek.Value) (*sobek.Promise, error) {
			popts, err := parseWaitForEventOptions(vu.Runtime(), optsOrPredicate, bc.Timeout())
			if err != nil {
				return nil, fmt.Errorf("parsing wait for event options: %w", err)
			}

			ctx := vu.Context()
			return k6ext.Promise(ctx, func() (result any, reason error) {
				var runInTaskQueue func(p *common.Page) (bool, error)
				if popts.PredicateFn != nil {
					runInTaskQueue = func(p *common.Page) (bool, error) {
						tq := vu.taskQueueRegistry.get(vu.Context(), p.TargetID())

						var rtn bool
						var err error
						// The function on the taskqueue runs in its own goroutine
						// so we need to use a channel to wait for it to complete
						// before returning the result to the caller.
						c := make(chan bool)
						tq.Queue(func() error {
							var resp sobek.Value
							resp, err = popts.PredicateFn(vu.Runtime().ToValue(p))
							rtn = resp.ToBoolean()
							close(c)
							return nil
						})
						<-c

						return rtn, err //nolint:wrapcheck
					}
				}

				resp, err := bc.WaitForEvent(event, runInTaskQueue, popts.Timeout)
				panicIfFatalError(ctx, err)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				p, ok := resp.(*common.Page)
				if !ok {
					panicIfFatalError(ctx, fmt.Errorf("response object is not a page: %w", k6error.ErrFatal))
				}

				return syncMapPage(vu, p), nil
			}), nil
		},
		"pages": func() *sobek.Object {
			var (
				mpages []mapping
				pages  = bc.Pages()
			)
			for _, page := range pages {
				if page == nil {
					continue
				}
				m := syncMapPage(vu, page)
				mpages = append(mpages, m)
			}

			return rt.ToValue(mpages).ToObject(rt)
		},
		"newPage": func() (mapping, error) {
			page, err := bc.NewPage()
			if err != nil {
				return nil, err //nolint:wrapcheck
			}
			return syncMapPage(vu, page), nil
		},
	}
}
