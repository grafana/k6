package browser

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/grafana/sobek"

	"go.k6.io/k6/internal/js/modules/k6/browser/common"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6error"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext"
)

// mapBrowserContext to the JS module.
func mapBrowserContext(vu moduleVU, bc *common.BrowserContext) mapping { //nolint:funlen,gocognit
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
		"grantPermissions": func(permissions []string, opts sobek.Value) (*sobek.Promise, error) {
			popts, err := exportTo[common.GrantPermissionsOptions](vu.Runtime(), opts)
			if err != nil {
				return nil, fmt.Errorf("parsing grant permission options: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, bc.GrantPermissions(permissions, popts)
			}), nil
		},
		"setDefaultNavigationTimeout": bc.SetDefaultNavigationTimeout,
		"setDefaultTimeout":           bc.SetDefaultTimeout,
		"setGeolocation": func(geolocation sobek.Value) (*sobek.Promise, error) {
			gl, err := exportTo[common.Geolocation](vu.Runtime(), geolocation)
			if err != nil {
				return nil, fmt.Errorf("parsing geo location: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, bc.SetGeolocation(&gl)
			}), nil
		},
		"setHTTPCredentials": func(httpCredentials sobek.Value) (*sobek.Promise, error) {
			creds, err := exportTo[common.Credentials](rt, httpCredentials)
			if err != nil {
				return nil, fmt.Errorf("parsing HTTP credentials: %w", err)
			}
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, bc.SetHTTPCredentials(creds) //nolint:staticcheck
			}), nil
		},
		"setOffline": func(offline bool) *sobek.Promise {
			return k6ext.Promise(vu.Context(), func() (any, error) {
				return nil, bc.SetOffline(offline) //nolint:wrapcheck
			})
		},
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
						tq := vu.taskQueueRegistry.get(ctx, p.TargetID())

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

						select {
						case <-c:
						case <-ctx.Done():
							err = errors.New("iteration ended before waitForEvent completed")
						}

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

// waitForEventOptions are the options used by the browserContext.waitForEvent API.
type waitForEventOptions struct {
	Timeout     time.Duration
	PredicateFn sobek.Callable
}

// parseWaitForEventOptions parses optsOrPredicate into a WaitForEventOptions.
// It returns a WaitForEventOptions with the default timeout if optsOrPredicate is nil,
// or not a callable predicate function.
// It can parse only a callable predicate function or an object which contains a
// callable predicate function and a timeout.
func parseWaitForEventOptions(
	rt *sobek.Runtime, optsOrPredicate sobek.Value, defaultTime time.Duration,
) (*waitForEventOptions, error) {
	w := &waitForEventOptions{
		Timeout: defaultTime,
	}

	if !sobekValueExists(optsOrPredicate) {
		return w, nil
	}
	var isCallable bool
	w.PredicateFn, isCallable = sobek.AssertFunction(optsOrPredicate)
	if isCallable {
		return w, nil
	}

	opts := optsOrPredicate.ToObject(rt)
	for _, k := range opts.Keys() {
		switch k {
		case "predicate":
			w.PredicateFn, isCallable = sobek.AssertFunction(opts.Get(k))
			if !isCallable {
				return nil, errors.New("predicate function is not callable")
			}
		case "timeout":
			w.Timeout = time.Duration(opts.Get(k).ToInteger()) * time.Millisecond
		default:
			return nil, fmt.Errorf("unknown option: %s", k)
		}
	}

	return w, nil
}
