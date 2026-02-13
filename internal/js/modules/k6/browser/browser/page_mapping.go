package browser

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/grafana/sobek"

	"go.k6.io/k6/internal/js/modules/k6/browser/common"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext"
	k6common "go.k6.io/k6/js/common"
)

// mapPage to the JS module.
//
//nolint:funlen
func mapPage(vu moduleVU, p *common.Page) mapping { //nolint:gocognit,cyclop
	rt := vu.Runtime()
	maps := mapping{
		"bringToFront": func() *sobek.Promise {
			return promise(vu, func() (any, error) {
				return nil, p.BringToFront() //nolint:wrapcheck
			})
		},
		"check": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameCheckOptions(p.MainFrame().Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing new frame check options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return nil, p.Check(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"click": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parseFrameClickOptions(vu.Context(), opts, p.MainFrame().Timeout())
			if err != nil {
				return nil, err
			}

			return promise(vu, func() (any, error) {
				err := p.Click(selector, popts)
				return nil, err //nolint:wrapcheck
			}), nil
		},
		"close": func(opts sobek.Value) *sobek.Promise {
			// TODO when opts are implemented for this function, parse them here before calling promise()
			// in a goroutine off the event loop. As that will race with anything running on the event loop.
			return promise(vu, func() (any, error) {
				// It's safe to close the taskqueue for this targetID (if one
				// exists).
				vu.close(p.TargetID())

				return nil, p.Close() //nolint:wrapcheck
			})
		},
		"content": func() *sobek.Promise {
			return promise(vu, func() (any, error) {
				return p.Content() //nolint:wrapcheck
			})
		},
		"context": func() mapping {
			return mapBrowserContext(vu, p.Context())
		},
		"dblclick": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameDblClickOptions(p.MainFrame().Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing double click options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return nil, p.Dblclick(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"dispatchEvent": func(selector, typ string, eventInit, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameDispatchEventOptions(p.MainFrame().Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing page dispatch event options: %w", err)
			}
			earg := exportArg(eventInit)
			return promise(vu, func() (any, error) {
				return nil, p.DispatchEvent(selector, typ, earg, popts) //nolint:wrapcheck
			}), nil
		},
		"emulateMedia": func(opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parsePageEmulateMediaOptions(rt, opts, common.NewPageEmulateMediaOptions(p))
			if err != nil {
				return nil, fmt.Errorf("parsing emulateMedia options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return nil, p.EmulateMedia(popts) //nolint:wrapcheck
			}), nil
		},
		"emulateVisionDeficiency": func(typ string) *sobek.Promise {
			return promise(vu, func() (any, error) {
				return nil, p.EmulateVisionDeficiency(typ) //nolint:wrapcheck
			})
		},
		"evaluate": func(pageFunc sobek.Value, gargs ...sobek.Value) (*sobek.Promise, error) {
			if sobekEmptyString(pageFunc) {
				return nil, fmt.Errorf("evaluate requires a page function")
			}
			funcString := pageFunc.String()
			gopts := exportArgs(gargs)
			return promise(vu, func() (any, error) {
				return p.Evaluate(funcString, gopts...)
			}), nil
		},
		"evaluateHandle": func(pageFunc sobek.Value, gargs ...sobek.Value) (*sobek.Promise, error) {
			if sobekEmptyString(pageFunc) {
				return nil, fmt.Errorf("evaluateHandle requires a page function")
			}
			funcString := pageFunc.String()
			gopts := exportArgs(gargs)
			return promise(vu, func() (any, error) {
				jsh, err := p.EvaluateHandle(funcString, gopts...)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				return mapJSHandle(vu, jsh), nil
			}), nil
		},
		"fill": func(selector string, value string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameFillOptions(p.MainFrame().Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing fill options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return nil, p.Fill(selector, value, popts) //nolint:wrapcheck
			}), nil
		},
		"focus": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameBaseOptions(p.MainFrame().Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing focus options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return nil, p.Focus(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"frames": func() *sobek.Object {
			var (
				mfrs []mapping
				frs  = p.Frames()
			)
			for _, fr := range frs {
				mfrs = append(mfrs, mapFrame(vu, fr))
			}
			return rt.ToValue(mfrs).ToObject(rt)
		},
		"getAttribute": func(selector string, name string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameBaseOptions(p.MainFrame().Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing getAttribute options: %w", err)
			}
			return promise(vu, func() (any, error) {
				s, ok, err := p.GetAttribute(selector, name, popts)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				if !ok {
					return nil, nil //nolint:nilnil
				}
				return s, nil
			}), nil
		},
		"getByRole": func(role sobek.Value, opts sobek.Value) (*sobek.Object, error) {
			if k6common.IsNullish(role) {
				return nil, errors.New("missing required argument 'role'")
			}
			popts := parseGetByRoleOptions(vu.Context(), opts)

			ml := mapLocator(vu, p.GetByRole(role.String(), popts))
			return rt.ToValue(ml).ToObject(rt), nil
		},
		"getByAltText": func(alt sobek.Value, opts sobek.Value) (*sobek.Object, error) {
			if k6common.IsNullish(alt) {
				return nil, errors.New("missing required argument 'altText'")
			}
			palt, popts := parseGetByBaseOptions(vu.Context(), alt, false, opts)

			ml := mapLocator(vu, p.GetByAltText(palt, popts))
			return rt.ToValue(ml).ToObject(rt), nil
		},
		"getByLabel": func(label sobek.Value, opts sobek.Value) (*sobek.Object, error) {
			if k6common.IsNullish(label) {
				return nil, errors.New("missing required argument 'label'")
			}
			plabel, popts := parseGetByBaseOptions(vu.Context(), label, true, opts)

			ml := mapLocator(vu, p.GetByLabel(plabel, popts))
			return rt.ToValue(ml).ToObject(rt), nil
		},
		"getByPlaceholder": func(placeholder sobek.Value, opts sobek.Value) (*sobek.Object, error) {
			if k6common.IsNullish(placeholder) {
				return nil, errors.New("missing required argument 'placeholder'")
			}
			pplaceholder, popts := parseGetByBaseOptions(vu.Context(), placeholder, false, opts)

			ml := mapLocator(vu, p.GetByPlaceholder(pplaceholder, popts))
			return rt.ToValue(ml).ToObject(rt), nil
		},
		"getByTitle": func(title sobek.Value, opts sobek.Value) (*sobek.Object, error) {
			if k6common.IsNullish(title) {
				return nil, errors.New("missing required argument 'title'")
			}
			ptitle, popts := parseGetByBaseOptions(vu.Context(), title, false, opts)

			ml := mapLocator(vu, p.GetByTitle(ptitle, popts))
			return rt.ToValue(ml).ToObject(rt), nil
		},
		"getByTestId": func(testID sobek.Value) (*sobek.Object, error) {
			if k6common.IsNullish(testID) {
				return nil, errors.New("missing required argument 'testId'")
			}
			ptestID := parseStringOrRegex(testID, false)

			ml := mapLocator(vu, p.GetByTestID(ptestID))
			return rt.ToValue(ml).ToObject(rt), nil
		},
		"getByText": func(text sobek.Value, opts sobek.Value) (*sobek.Object, error) {
			if k6common.IsNullish(text) {
				return nil, errors.New("missing required argument 'text'")
			}
			ptext, popts := parseGetByBaseOptions(vu.Context(), text, true, opts)

			ml := mapLocator(vu, p.GetByText(ptext, popts))
			return rt.ToValue(ml).ToObject(rt), nil
		},
		"goto": func(url string, opts sobek.Value) (*sobek.Promise, error) {
			gopts := common.NewFrameGotoOptions(
				p.Referrer(),
				p.NavigationTimeout(),
			)
			if err := gopts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing page navigation options to %q: %w", url, err)
			}
			return promise(vu, func() (any, error) {
				resp, err := p.Goto(url, gopts)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}

				return mapResponse(vu, resp), nil
			}), nil
		},
		"hover": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameHoverOptions(p.MainFrame().Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing hover options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return nil, p.Hover(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"innerHTML": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameInnerHTMLOptions(p.MainFrame().Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing inner HTML options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return p.InnerHTML(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"innerText": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameInnerTextOptions(p.MainFrame().Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing inner text options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return p.InnerText(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"inputValue": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameInputValueOptions(p.MainFrame().Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing input value options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return p.InputValue(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"isChecked": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameIsCheckedOptions(p.MainFrame().Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parse isChecked options of selector %q: %w", selector, err)
			}
			return promise(vu, func() (any, error) {
				return p.IsChecked(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"isClosed": p.IsClosed,
		"isDisabled": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameIsDisabledOptions(p.MainFrame().Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parse isDisabled options of selector %q: %w", selector, err)
			}
			return promise(vu, func() (any, error) {
				return p.IsDisabled(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"isEditable": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameIsEditableOptions(p.MainFrame().Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parse isEditabled options of selector %q: %w", selector, err)
			}
			return promise(vu, func() (any, error) {
				return p.IsEditable(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"isEnabled": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameIsEnabledOptions(p.MainFrame().Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parse isEnabled options of selector %q: %w", selector, err)
			}
			return promise(vu, func() (any, error) {
				return p.IsEnabled(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"isHidden": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameIsHiddenOptions()
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parse isHidden options of selector %q: %w", selector, err)
			}
			return promise(vu, func() (any, error) {
				return p.IsHidden(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"isVisible": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameIsVisibleOptions()
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing isVisible options of selector %q: %w", selector, err)
			}
			return promise(vu, func() (any, error) {
				return p.IsVisible(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"keyboard": mapKeyboard(vu, p.GetKeyboard()),
		"locator": func(selector string, opts sobek.Value) *sobek.Object {
			ml := mapLocator(vu, p.Locator(selector, parseLocatorOptions(rt, opts)))
			return rt.ToValue(ml).ToObject(rt)
		},
		"frameLocator": func(selector string) *sobek.Object {
			mfl := mapFrameLocator(vu, p.FrameLocator(selector))
			return rt.ToValue(mfl).ToObject(rt)
		},
		"mainFrame": func() *sobek.Object {
			mf := mapFrame(vu, p.MainFrame())
			return rt.ToValue(mf).ToObject(rt)
		},
		"mouse": mapMouse(vu, p.GetMouse()),
		"on":    mapPageOn(vu, p),
		"opener": func() *sobek.Promise {
			return promise(vu, func() (any, error) {
				return p.Opener(), nil
			})
		},
		"press": func(selector string, key string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFramePressOptions(p.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing press options of selector %q: %w", selector, err)
			}
			return promise(vu, func() (any, error) {
				return nil, p.Press(selector, key, popts) //nolint:wrapcheck
			}), nil
		},
		"reload": func(opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewPageReloadOptions(common.LifecycleEventLoad, p.NavigationTimeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing reload options: %w", err)
			}
			return promise(vu, func() (any, error) {
				resp, err := p.Reload(popts)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				if resp == nil {
					return nil, nil //nolint:nilnil
				}
				return mapResponse(vu, resp), nil
			}), nil
		},
		"goBack": func(opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewPageGoBackForwardOptions(common.LifecycleEventLoad, p.NavigationTimeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing page go back options: %w", err)
			}
			return promise(vu, func() (any, error) {
				resp, err := p.GoBackForward(-1, popts)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				if resp == nil {
					return nil, nil //nolint:nilnil
				}
				return mapResponse(vu, resp), nil
			}), nil
		},
		"goForward": func(opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewPageGoBackForwardOptions(common.LifecycleEventLoad, p.NavigationTimeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing page go forward options: %w", err)
			}
			return promise(vu, func() (any, error) {
				resp, err := p.GoBackForward(+1, popts)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				if resp == nil {
					return nil, nil //nolint:nilnil
				}
				return mapResponse(vu, resp), nil
			}), nil
		},
		"route": mapPageRoute(vu, p),
		"screenshot": func(opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewPageScreenshotOptions()
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing page screenshot options: %w", err)
			}

			rt := vu.Runtime()
			promise, res, rej := rt.NewPromise()
			callback := vu.RegisterCallback()
			go func() {
				bb, err := p.Screenshot(popts, vu.filePersister)
				if err != nil {
					callback(func() error {
						return rej(err)
					})
					return
				}

				callback(func() error {
					return res(rt.NewArrayBuffer(bb))
				})
			}()

			return promise, nil
		},
		"selectOption": func(selector string, values sobek.Value, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameSelectOptionOptions(p.MainFrame().Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing select option options: %w", err)
			}

			convValues, err := ConvertSelectOptionValues(vu.Runtime(), values)
			if err != nil {
				return nil, fmt.Errorf("parsing select options values: %w", err)
			}
			return promise(vu, func() (any, error) {
				return p.SelectOption(selector, convValues, popts) //nolint:wrapcheck
			}), nil
		},
		"setChecked": func(selector string, checked bool, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameCheckOptions(p.MainFrame().Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing frame set check options: %w", err)
			}

			return promise(vu, func() (any, error) {
				return nil, p.SetChecked(selector, checked, popts) //nolint:wrapcheck
			}), nil
		},
		"setContent": func(html string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameSetContentOptions(p.MainFrame().NavigationTimeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing setContent options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return nil, p.SetContent(html, popts) //nolint:wrapcheck
			}), nil
		},
		"setDefaultNavigationTimeout": p.SetDefaultNavigationTimeout,
		"setDefaultTimeout":           p.SetDefaultTimeout,
		"setExtraHTTPHeaders": func(headers map[string]string) *sobek.Promise {
			return promise(vu, func() (any, error) {
				return nil, p.SetExtraHTTPHeaders(headers) //nolint:wrapcheck
			})
		},
		"setInputFiles": func(selector string, files sobek.Value, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameSetInputFilesOptions(p.MainFrame().Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing setInputFiles options: %w", err)
			}

			pfiles := new(common.Files)
			if err := pfiles.Parse(vu.Context(), files); err != nil {
				return nil, fmt.Errorf("parsing setInputFiles parameter: %w", err)
			}

			return promise(vu, func() (any, error) {
				return nil, p.SetInputFiles(selector, pfiles, popts) //nolint:wrapcheck
			}), nil
		},
		"setViewportSize": func(viewportSize sobek.Value) (*sobek.Promise, error) {
			s, err := parseSize(rt, viewportSize)
			if err != nil {
				return nil, fmt.Errorf("parsing viewport size: %w", err)
			}
			return promise(vu, func() (any, error) {
				return nil, p.SetViewportSize(s) //nolint:wrapcheck
			}), nil
		},
		"tap": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameTapOptions(p.Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing page tap options: %w", err)
			}
			return promise(vu, func() (any, error) {
				return nil, p.Tap(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"textContent": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameTextContentOptions(p.MainFrame().Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing text content options: %w", err)
			}

			return promise(vu, func() (any, error) {
				s, ok, err := p.TextContent(selector, popts)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				if !ok {
					return nil, nil //nolint:nilnil
				}
				return s, nil
			}), nil
		},
		"throttleCPU": func(cpuProfile common.CPUProfile) *sobek.Promise {
			return promise(vu, func() (any, error) {
				return nil, p.ThrottleCPU(cpuProfile) //nolint:wrapcheck
			})
		},
		"throttleNetwork": func(networkProfile common.NetworkProfile) *sobek.Promise {
			return promise(vu, func() (any, error) {
				return nil, p.ThrottleNetwork(networkProfile) //nolint:wrapcheck
			})
		},
		"title": func() *sobek.Promise {
			return promise(vu, func() (any, error) {
				return p.Title() //nolint:wrapcheck
			})
		},
		"touchscreen": mapTouchscreen(vu, p.GetTouchscreen()),
		"type": func(selector string, text string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameTypeOptions(p.MainFrame().Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing type options: %w", err)
			}

			return promise(vu, func() (any, error) {
				return nil, p.Type(selector, text, popts) //nolint:wrapcheck
			}), nil
		},
		"uncheck": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameUncheckOptions(p.MainFrame().Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing frame uncheck options %q: %w", selector, err)
			}

			return promise(vu, func() (any, error) {
				return nil, p.Uncheck(selector, popts) //nolint:wrapcheck
			}), nil
		},
		"unroute": func(url string) (*sobek.Promise, error) {
			return promise(vu, func() (any, error) {
				return nil, p.Unroute(url)
			}), nil
		},
		"unrouteAll": func() (*sobek.Promise, error) {
			return promise(vu, func() (any, error) {
				return nil, p.UnrouteAll()
			}), nil
		},
		"url":          p.URL,
		"viewportSize": p.ViewportSize,
		"waitForFunction": func(pageFunc, opts sobek.Value, args ...sobek.Value) (*sobek.Promise, error) {
			js, popts, pargs, err := parseWaitForFunctionArgs(
				vu.Context(), p.Timeout(), pageFunc, opts, args...,
			)
			if err != nil {
				return nil, fmt.Errorf("page waitForFunction: %w", err)
			}

			return promise(vu, func() (result any, reason error) {
				return p.WaitForFunction(js, popts, pargs...) //nolint:wrapcheck
			}), nil
		},
		"waitForLoadState": func(state string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameWaitForLoadStateOptions(p.MainFrame().Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing waitForLoadState %q options: %w", state, err)
			}

			return promise(vu, func() (any, error) {
				return nil, p.WaitForLoadState(state, popts) //nolint:wrapcheck
			}), nil
		},
		"waitForNavigation": func(opts sobek.Value) (*sobek.Promise, error) {
			return mapWaitForNavigation(vu, p, opts)
		},
		"waitForSelector": func(selector string, opts sobek.Value) (*sobek.Promise, error) {
			popts := common.NewFrameWaitForSelectorOptions(p.MainFrame().Timeout())
			if err := popts.Parse(vu.Context(), opts); err != nil {
				return nil, fmt.Errorf("parsing wait for selector %q options: %w", selector, err)
			}

			return promise(vu, func() (any, error) {
				eh, err := p.WaitForSelector(selector, popts)
				if err != nil {
					return nil, err //nolint:wrapcheck
				}
				return mapElementHandle(vu, eh), nil
			}), nil
		},
		"waitForTimeout": func(timeout int64) *sobek.Promise {
			return promise(vu, func() (any, error) {
				p.WaitForTimeout(timeout)
				return nil, nil
			})
		},
		"waitForURL": func(url sobek.Value, opts sobek.Value) (*sobek.Promise, error) {
			return mapWaitForURL(vu, p, url, opts)
		},
		"waitForResponse": func(url sobek.Value, opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parsePageWaitForResponseOptions(vu.Context(), opts, p.Timeout())
			if err != nil {
				return nil, fmt.Errorf("parsing waitForResponse options: %w", err)
			}

			var val string
			switch url.ExportType() {
			case reflect.TypeOf(string("")):
				val = "'" + url.String() + "'" // Strings require quotes
			default: // JS Regex, CSS, numbers or booleans
				val = url.String() // No quotes
			}

			tq, ctx, stop := newTaskQueue(vu)

			return promise(vu, func() (result any, reason error) {
				defer stop()
				return p.WaitForResponse(val, popts, newRegExMatcher(ctx, vu, tq))
			}), nil
		},
		"waitForRequest": func(url sobek.Value, opts sobek.Value) (*sobek.Promise, error) {
			popts, err := parsePageWaitForRequestOptions(vu.Context(), opts, p.Timeout())
			if err != nil {
				return nil, fmt.Errorf("parsing waitForRequest options: %w", err)
			}

			var val string
			switch url.ExportType() {
			case reflect.TypeOf(string("")):
				val = "'" + url.String() + "'" // Strings require quotes
			default: // JS Regex, CSS, numbers or booleans
				val = url.String() // No quotes
			}

			tq, ctx, stop := newTaskQueue(vu)

			return promise(vu, func() (result any, reason error) {
				defer stop()
				return p.WaitForRequest(val, popts, newRegExMatcher(ctx, vu, tq))
			}), nil
		},
		"waitForEvent": func(event common.PageEventName, opts sobek.Value) (*sobek.Promise, error) {
			// AVOID using a default case to force handling new event types explicitly
			// so that the linter can catch unhandled event types as non-exhaustive switch.
			// Otherwise, we might miss mapping new [PageEvent] types added in the future.
			// This is for keeping events in sync between waitForEvent and page.on.
			mapPageEvent := func(vu moduleVU, pe common.PageEvent) (mapping, error) {
				switch event {
				case common.PageEventConsole:
					return mapConsoleMessage(vu, pe), nil
				case common.PageEventRequest:
					return mapRequestEvent(vu, pe), nil
				case common.PageEventResponse:
					return mapResponseEvent(vu, pe), nil
				case common.PageEventRequestFinished:
					return mapRequestEvent(vu, pe), nil
				case common.PageEventRequestFailed:
					return mapRequestEvent(vu, pe), nil
				case common.PageEventMetric:
					// intentionally left blank
				}
				return nil, fmt.Errorf("waitForEvent does not support mapping for event: %q", event)
			}

			popts, fn, err := parsePageWaitForEventOptions(vu.Context(), opts, p.Timeout())
			if err != nil {
				return nil, fmt.Errorf("parsing waitForEvent options: %w", err)
			}

			ctx := vu.Context()
			tq := vu.get(ctx, p.TargetID())

			return promise(vu, func() (any, error) {
				rpe, err := p.WaitForEvent(event, popts, func(pe common.PageEvent) (bool, error) {
					if fn == nil {
						return true, nil
					}
					return queueTask(ctx, tq, func() (bool, error) {
						m, err := mapPageEvent(vu, pe)
						if err != nil {
							return false, err
						}
						v, err := fn(sobek.Undefined(), vu.Runtime().ToValue(m))
						if err != nil {
							return false, fmt.Errorf("executing waitForEvent predicate: %w", err)
						}
						return v.ToBoolean(), nil
					})()
				})
				if err != nil {
					return nil, fmt.Errorf("waiting for page event %q: %w", event, err)
				}
				return mapPageEvent(vu, rpe)
			}), nil
		},
		"workers": func() *sobek.Object {
			var mws []mapping
			for _, w := range p.Workers() {
				mw := mapWorker(vu, w)
				mws = append(mws, mw)
			}
			return rt.ToValue(mws).ToObject(rt)
		},
	}
	maps["$"] = func(selector string) *sobek.Promise {
		return promise(vu, func() (any, error) {
			eh, err := p.Query(selector)
			if err != nil {
				return nil, err //nolint:wrapcheck
			}
			// ElementHandle can be null when the selector does not match any elements.
			// We do not want to map nil elementHandles since the expectation is a
			// null result in the test script for this case.
			if eh == nil {
				return nil, nil
			}
			ehm := mapElementHandle(vu, eh)

			return ehm, nil
		})
	}
	maps["$$"] = func(selector string) *sobek.Promise {
		return promise(vu, func() (any, error) {
			ehs, err := p.QueryAll(selector)
			if err != nil {
				return nil, err //nolint:wrapcheck
			}
			var mehs []mapping
			for _, eh := range ehs {
				ehm := mapElementHandle(vu, eh)
				mehs = append(mehs, ehm)
			}
			return mehs, nil
		})
	}

	return maps
}

// mapPageOn enables using various page.on event handlers with the page.on method.
// It provides a generic way to map different event types to their respective handler functions.
func mapPageOn(vu moduleVU, p *common.Page) func(common.PageEventName, sobek.Callable) error {
	return func(eventName common.PageEventName, handle sobek.Callable) error {
		if handle == nil {
			panic(vu.Runtime().NewTypeError(`The "listener" argument must be a function`))
		}

		pageEvents := map[common.PageEventName]struct {
			mapp func(vu moduleVU, event common.PageEvent) mapping
			wait bool // Whether to wait for the handler to complete.
		}{
			common.PageEventConsole:         {mapp: mapConsoleMessage},
			common.PageEventMetric:          {mapp: mapMetricEvent, wait: true},
			common.PageEventRequest:         {mapp: mapRequestEvent},
			common.PageEventResponse:        {mapp: mapResponseEvent},
			common.PageEventRequestFinished: {mapp: mapRequestEvent},
			common.PageEventRequestFailed:   {mapp: mapRequestEvent},
		}
		pageEvent, ok := pageEvents[eventName]
		if !ok {
			return fmt.Errorf("unknown page on event: %q", eventName)
		}

		ctx := vu.Context()
		tq := vu.get(ctx, p.TargetID())

		return p.On(eventName, func(event common.PageEvent) error {
			wait := queueTask(ctx, tq, func() (sobek.Value, error) {
				_, err := handle(sobek.Undefined(), vu.Runtime().ToValue(pageEvent.mapp(vu, event)))
				if err != nil {
					return nil, fmt.Errorf("executing page.on('%s') handler: %w", eventName, err)
				}
				return nil, nil
			})
			if pageEvent.wait {
				if _, err := wait(); errors.Is(err, context.Canceled) {
					return errors.New("iteration ended before page.on handler completed executing")
				}
			}
			return nil
		})
	}
}

func parseWaitForFunctionArgs(
	ctx context.Context, timeout time.Duration, pageFunc, opts sobek.Value, gargs ...sobek.Value,
) (string, *common.FrameWaitForFunctionOptions, []any, error) {
	popts := common.NewFrameWaitForFunctionOptions(timeout)
	err := popts.Parse(ctx, opts)
	if err != nil {
		return "", nil, nil, fmt.Errorf("parsing waitForFunction options: %w", err)
	}

	js := pageFunc.ToString().String()
	_, isCallable := sobek.AssertFunction(pageFunc)
	if !isCallable {
		js = fmt.Sprintf("() => (%s)", js)
	}

	return js, popts, exportArgs(gargs), nil
}

// parseStringOrRegex parses a sobek.Value to return either a quoted string if it was a string,
// or a raw string if it was a JS RegExp object or another type.
//
// Some getBy* APIs work with single quotes and some work with double quotes.
// This inconsistency seems to stem from the injected code copied from
// Playwright itself.
//
// I would prefer not to change the copied injected script code from Playwright
// so that it is easier to copy over updates/fixes from Playwright when we need
// to.
func parseStringOrRegex(v sobek.Value, doubleQuote bool) string {
	const stringType = string("")

	var a string
	switch v.ExportType() {
	case reflect.TypeOf(stringType): // text values require quotes
		if doubleQuote {
			a = `"` + strings.ReplaceAll(v.String(), `"`, `\"`) + `"`
		} else {
			a = `'` + strings.ReplaceAll(v.String(), `'`, `\'`) + `'`
		}
	case reflect.TypeOf(map[string]interface{}(nil)): // JS RegExp
		a = v.String() // No quotes
	default: // CSS, numbers or booleans
		a = v.String() // No quotes
	}
	return a
}

// parseGetByRoleOptions parses the GetByRole options from the Sobek.Value.
func parseGetByRoleOptions(ctx context.Context, opts sobek.Value) *common.GetByRoleOptions {
	if k6common.IsNullish(opts) {
		return nil
	}

	o := &common.GetByRoleOptions{}

	rt := k6ext.Runtime(ctx)

	obj := opts.ToObject(rt)
	for _, k := range obj.Keys() {
		switch k {
		case "checked":
			val := obj.Get(k).ToBoolean()
			o.Checked = &val
		case "disabled":
			val := obj.Get(k).ToBoolean()
			o.Disabled = &val
		case "exact":
			val := obj.Get(k).ToBoolean()
			o.Exact = &val
		case "expanded":
			val := obj.Get(k).ToBoolean()
			o.Expanded = &val
		case "includeHidden":
			val := obj.Get(k).ToBoolean()
			o.IncludeHidden = &val
		case "level":
			val := obj.Get(k).ToInteger()
			o.Level = &val
		case "name":
			val := parseStringOrRegex(obj.Get(k), false)
			o.Name = &val
		case "pressed":
			val := obj.Get(k).ToBoolean()
			o.Pressed = &val
		case "selected":
			val := obj.Get(k).ToBoolean()
			o.Selected = &val
		}
	}

	return o
}

// parseGetByBaseOptions parses the options for the GetBy* APIs and the input
// text/regex.
func parseGetByBaseOptions(
	ctx context.Context,
	input sobek.Value,
	doubleQuote bool,
	opts sobek.Value,
) (string, *common.GetByBaseOptions) {
	a := parseStringOrRegex(input, doubleQuote)

	if k6common.IsNullish(opts) {
		return a, nil
	}

	o := &common.GetByBaseOptions{}

	rt := k6ext.Runtime(ctx)

	obj := opts.ToObject(rt)
	for _, k := range obj.Keys() {
		if k == "exact" {
			val := obj.Get(k).ToBoolean()
			o.Exact = &val
		}
	}

	return a, o
}

// mapPageRoute maps the requested page.route event to the Sobek runtime.
func mapPageRoute(vu moduleVU, p *common.Page) func(sobek.Value, sobek.Callable) (*sobek.Promise, error) {
	return func(path sobek.Value, cb sobek.Callable) (*sobek.Promise, error) {
		ctx := vu.Context()

		ppath := parseStringOrRegex(path, false)
		tq := vu.get(ctx, p.TargetID())

		route := func(r *common.Route) error {
			_, err := queueTask(ctx, tq, func() (any, error) {
				return cb(sobek.Undefined(), vu.Runtime().ToValue(mapRoute(vu, r)))
			})()
			if errors.Is(err, context.Canceled) {
				return fmt.Errorf("page.route('%s'): iteration ended before route completed", path)
			}
			if err != nil {
				return fmt.Errorf("page.route('%s'): %w", path, err)
			}
			return nil
		}

		return promise(vu, func() (any, error) {
			return nil, p.Route(ppath, route, newRegExMatcher(ctx, vu, tq))
		}), nil
	}
}

func mapWaitForURL(vu moduleVU, target interface {
	Timeout() time.Duration
	WaitForURL(urlPattern string, opts *common.FrameWaitForURLOptions, rm common.RegExMatcher) error
}, url sobek.Value, opts sobek.Value,
) (*sobek.Promise, error) {
	if k6common.IsNullish(url) {
		return nil, errors.New("missing required argument 'url'")
	}
	popts := common.NewFrameWaitForURLOptions(target.Timeout())
	if err := popts.Parse(vu.Context(), opts); err != nil {
		return nil, fmt.Errorf("parsing waitForURL options: %w", err)
	}

	purl := parseStringOrRegex(url, false)
	tq, ctx, stop := newTaskQueue(vu)

	return promise(vu, func() (result any, reason error) {
		defer stop()
		return nil, target.WaitForURL(purl, popts, newRegExMatcher(ctx, vu, tq))
	}), nil
}

func mapWaitForNavigation(vu moduleVU, target interface {
	Timeout() time.Duration
	WaitForNavigation(*common.FrameWaitForNavigationOptions, common.RegExMatcher) (*common.Response, error)
}, opts sobek.Value,
) (*sobek.Promise, error) {
	popts := common.NewFrameWaitForNavigationOptions(target.Timeout())
	if err := popts.Parse(vu.Context(), opts); err != nil {
		return nil, fmt.Errorf("parsing frame wait for navigation options: %w", err)
	}

	// Only use task queue and RegExMatcher if a URL is specified.
	rm, stop := func() (common.RegExMatcher, func()) {
		if popts.URL == "" {
			return nil, func() {}
		}
		tq, ctx, stop := newTaskQueue(vu)
		return newRegExMatcher(ctx, vu, tq), stop
	}()

	return promise(vu, func() (any, error) {
		defer stop()

		resp, err := target.WaitForNavigation(popts, rm)
		if err != nil {
			return nil, err //nolint:wrapcheck
		}
		return mapResponse(vu, resp), nil
	}), nil
}

func parsePageWaitForResponseOptions(
	ctx context.Context, opts sobek.Value, defaultTimeout time.Duration,
) (*common.PageWaitForResponseOptions, error) {
	ropts := common.NewPageWaitForResponseOptions(defaultTimeout)
	if k6common.IsNullish(opts) {
		return ropts, nil
	}

	rt := k6ext.Runtime(ctx)
	obj := opts.ToObject(rt)
	for _, k := range obj.Keys() {
		switch k {
		case "timeout":
			ropts.Timeout = time.Duration(obj.Get(k).ToInteger()) * time.Millisecond
		default:
			return ropts, fmt.Errorf("unsupported waitForResponse option: '%s'", k)
		}
	}

	return ropts, nil
}

func parsePageWaitForRequestOptions(
	ctx context.Context, opts sobek.Value, defaultTimeout time.Duration,
) (*common.PageWaitForRequestOptions, error) {
	ropts := common.PageWaitForRequestOptions{
		Timeout: defaultTimeout,
	}

	if k6common.IsNullish(opts) {
		return &ropts, nil
	}

	obj := opts.ToObject(k6ext.Runtime(ctx))
	for _, k := range obj.Keys() {
		switch k {
		case "timeout":
			ropts.Timeout = time.Duration(obj.Get(k).ToInteger()) * time.Millisecond
		default:
			return &ropts, fmt.Errorf("unsupported waitForRequest option: '%s'", k)
		}
	}

	return &ropts, nil
}

func parsePageWaitForEventOptions(
	ctx context.Context, opts sobek.Value, defaultTimeout time.Duration,
) (*common.PageWaitForEventOptions, sobek.Callable, error) {
	ropts := &common.PageWaitForEventOptions{
		Timeout: defaultTimeout,
	}

	if k6common.IsNullish(opts) {
		return ropts, nil, nil
	}

	if fn, ok := sobek.AssertFunction(opts); ok {
		return ropts, fn, nil
	}

	obj := opts.ToObject(k6ext.Runtime(ctx))

	var pred sobek.Callable
	for _, k := range obj.Keys() {
		switch k {
		case "timeout":
			ropts.Timeout = time.Duration(obj.Get(k).ToInteger()) * time.Millisecond
		case "predicate":
			fn, ok := sobek.AssertFunction(obj.Get(k))
			if !ok {
				return nil, nil, fmt.Errorf("predicate must be a function")
			}
			pred = fn
		default:
			return nil, nil, fmt.Errorf("waitForEvent does not support option: '%s'", k)
		}
	}

	return ropts, pred, nil
}

// parseSize parses the size options from a Sobek value.
func parseSize(rt *sobek.Runtime, opts sobek.Value) (*common.Size, error) {
	size := &common.Size{}
	if k6common.IsNullish(opts) {
		return size, nil
	}

	obj := opts.ToObject(rt)
	for _, k := range obj.Keys() {
		v := obj.Get(k)
		if k6common.IsNullish(v) {
			continue
		}
		switch k {
		case "width":
			switch v.ExportType().Kind() {
			case reflect.Int64, reflect.Float64:
				size.Width = v.ToFloat()
			default:
				return nil, fmt.Errorf("width must be a number, got %s", v.ExportType().Kind())
			}
		case "height":
			switch v.ExportType().Kind() {
			case reflect.Int64, reflect.Float64:
				size.Height = v.ToFloat()
			default:
				return nil, fmt.Errorf("height must be a number, got %s", v.ExportType().Kind())
			}
		}
	}

	return size, nil
}

// parsePageEmulateMediaOptions parses the page emulate media options from a Sobek value.
//
//nolint:unparam
func parsePageEmulateMediaOptions(
	rt *sobek.Runtime, opts sobek.Value, defaults *common.PageEmulateMediaOptions,
) (*common.PageEmulateMediaOptions, error) {
	if k6common.IsNullish(opts) {
		return defaults, nil
	}

	obj := opts.ToObject(rt)
	for _, k := range obj.Keys() {
		switch k {
		case "colorScheme":
			defaults.ColorScheme = common.ColorScheme(obj.Get(k).String())
		case "media":
			defaults.Media = common.MediaType(obj.Get(k).String())
		case "reducedMotion":
			defaults.ReducedMotion = common.ReducedMotion(obj.Get(k).String())
		}
	}

	return defaults, nil
}
