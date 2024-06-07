package browser

import (
	"fmt"
	"reflect"

	"github.com/grafana/sobek"

	"github.com/grafana/xk6-browser/common"
	"github.com/grafana/xk6-browser/k6error"
	"github.com/grafana/xk6-browser/k6ext"
)

// mapBrowserContext to the JS module.
func mapBrowserContext(vu moduleVU, bc *common.BrowserContext) mapping { //nolint:funlen,gocognit,cyclop
	rt := vu.Runtime()
	return mapping{
		"addCookies": func(cookies []*common.Cookie) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, bc.AddCookies(cookies) //nolint:wrapcheck
			})
		},
		"addInitScript": func(script sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				if !sobekValueExists(script) {
					return nil, nil
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

				return nil, bc.AddInitScript(source) //nolint:wrapcheck
			})
		},
		"browser": func() mapping {
			// the browser is grabbed from VU.
			return mapBrowser(vu)
		},
		"clearCookies": func() *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, bc.ClearCookies() //nolint:wrapcheck
			})
		},
		"clearPermissions": func() *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, bc.ClearPermissions() //nolint:wrapcheck
			})
		},
		"close": func() *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, bc.Close() //nolint:wrapcheck
			})
		},
		"cookies": func(urls ...string) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return bc.Cookies(urls...) //nolint:wrapcheck
			})
		},
		"grantPermissions": func(permissions []string, opts sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				popts := common.NewGrantPermissionsOptions()
				popts.Parse(vu.Context(), opts)

				return nil, bc.GrantPermissions(permissions, popts) //nolint:wrapcheck
			})
		},
		"setDefaultNavigationTimeout": bc.SetDefaultNavigationTimeout,
		"setDefaultTimeout":           bc.SetDefaultTimeout,
		"setGeolocation": func(geolocation sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, bc.SetGeolocation(geolocation) //nolint:wrapcheck
			})
		},
		"setHTTPCredentials": func(httpCredentials sobek.Value) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, bc.SetHTTPCredentials(httpCredentials) //nolint:staticcheck,wrapcheck
			})
		},
		"setOffline": func(offline bool) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, bc.SetOffline(offline) //nolint:wrapcheck
			})
		},
		"waitForEvent": func(event string, optsOrPredicate sobek.Value) (*sobek.Promise, error) {
			ctx := vu.Context()
			popts := common.NewWaitForEventOptions(
				bc.Timeout(),
			)
			if err := popts.Parse(ctx, optsOrPredicate); err != nil {
				return nil, fmt.Errorf("parsing waitForEvent options: %w", err)
			}

			return k6ext.Promise(ctx, func() (result any, reason error) {
				var runInTaskQueue func(p *common.Page) (bool, error)
				if popts.PredicateFn != nil {
					runInTaskQueue = func(p *common.Page) (bool, error) {
						tq := vu.taskQueueRegistry.get(p.TargetID())

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

				return mapPage(vu, p), nil
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
				m := mapPage(vu, page)
				mpages = append(mpages, m)
			}

			return rt.ToValue(mpages).ToObject(rt)
		},
		"newPage": func() *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				page, err := bc.NewPage()
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				return mapPage(vu, page), nil
			})
		},
	}
}
