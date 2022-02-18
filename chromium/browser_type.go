package chromium

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/dop251/goja"
	k6common "go.k6.io/k6/js/common"
	k6lib "go.k6.io/k6/lib"

	"github.com/grafana/xk6-browser/api"
	"github.com/grafana/xk6-browser/common"
)

// Ensure BrowserType implements the api.BrowserType interface.
var _ api.BrowserType = &BrowserType{}

type BrowserType struct {
	Ctx             context.Context
	CancelFn        context.CancelFunc
	hooks           *common.Hooks
	fieldNameMapper *common.FieldNameMapper
}

func NewBrowserType(ctx context.Context) api.BrowserType {
	rt := k6common.GetRuntime(ctx)
	hooks := common.NewHooks()

	// Create extension master context. If this context is cancelled we'll
	// initiate an extension wide cancellation and shutdown.
	extensionCtx, extensionCancelFn := context.WithCancel(ctx)
	extensionCtx = common.WithHooks(extensionCtx, hooks)

	b := BrowserType{
		Ctx:             extensionCtx,
		CancelFn:        extensionCancelFn,
		hooks:           hooks,
		fieldNameMapper: common.NewFieldNameMapper(),
	}
	rt.SetFieldNameMapper(b.fieldNameMapper)
	return &b
}

func (b *BrowserType) Connect(opts goja.Value) {
	rt := k6common.GetRuntime(b.Ctx)
	k6common.Throw(rt, errors.New("BrowserType.connect() has not been implemented yet"))
}

func (b *BrowserType) ExecutablePath() string {
	return "chromium"
}

// Launch creates a new client to remote control a Chrome browser.
func (b *BrowserType) Launch(opts goja.Value) api.Browser {
	var (
		rt         = k6common.GetRuntime(b.Ctx)
		state      = k6lib.GetState(b.Ctx)
		launchOpts = common.NewLaunchOptions()
	)
	if err := launchOpts.Parse(b.Ctx, opts); err != nil {
		k6common.Throw(rt, fmt.Errorf("cannot parse launch options: %v", err))
	}
	b.Ctx = common.WithLaunchOptions(b.Ctx, launchOpts)

	envs := make([]string, 0, len(launchOpts.Env))
	for k, v := range launchOpts.Env {
		envs = append(envs, fmt.Sprintf("%s=%s", k, v))
	}

	allocator := NewAllocator(b.flags(launchOpts, &state.Options), envs)
	browserProc, err := allocator.Allocate(b.Ctx, launchOpts)
	if browserProc == nil {
		k6common.Throw(rt, fmt.Errorf("cannot allocate browser: %w", err))
	}

	logger, err := makeLogger(b.Ctx, launchOpts)
	if err != nil {
		k6common.Throw(rt, fmt.Errorf("cannot make logger: %w", err))
	}
	browserProc.AttachLogger(logger)

	// attach the browser process ID to the context
	// so that we can kill it afterward if it lingers
	// see: k6Throw function
	b.Ctx = common.WithProcessID(b.Ctx, browserProc.Pid())
	browser, err := common.NewBrowser(b.Ctx, b.CancelFn, browserProc, launchOpts, logger)
	if err != nil {
		k6common.Throw(rt, err)
	}
	return browser
}

func (b *BrowserType) LaunchPersistentContext(userDataDir string, opts goja.Value) api.Browser {
	rt := k6common.GetRuntime(b.Ctx)
	k6common.Throw(rt, errors.New("BrowserType.LaunchPersistentContext(userDataDir, opts) has not been implemented yet"))
	return nil
}

func (b *BrowserType) Name() string {
	return "chromium"
}

func (b *BrowserType) flags(lopts *common.LaunchOptions, k6opts *k6lib.Options) map[string]interface{} {
	// After Puppeteer's and Playwright's default behavior.
	f := map[string]interface{}{
		"disable-background-networking":                      true,
		"enable-features":                                    "NetworkService,NetworkServiceInProcess",
		"disable-background-timer-throttling":                true,
		"disable-backgrounding-occluded-windows":             true,
		"disable-breakpad":                                   true,
		"disable-client-side-phishing-detection":             true,
		"disable-component-extensions-with-background-pages": true,
		"disable-default-apps":                               true,
		"disable-dev-shm-usage":                              true,
		"disable-extensions":                                 true,
		"disable-features":                                   "ImprovedCookieControls,LazyFrameLoading,GlobalMediaControls,DestroyProfileOnBrowserClose,MediaRouter,AcceptCHFrame",
		"disable-hang-monitor":                               true,
		"disable-ipc-flooding-protection":                    true,
		"disable-popup-blocking":                             true,
		"disable-prompt-on-repost":                           true,
		"disable-renderer-backgrounding":                     true,
		"disable-sync":                                       true,
		"force-color-profile":                                "srgb",
		"metrics-recording-only":                             true,
		"no-first-run":                                       true,
		"safebrowsing-disable-auto-update":                   true,
		"enable-automation":                                  true,
		"password-store":                                     "basic",
		"use-mock-keychain":                                  true,
		"no-service-autorun":                                 true,

		"no-startup-window":           true,
		"no-default-browser-check":    true,
		"no-sandbox":                  true,
		"headless":                    lopts.Headless,
		"auto-open-devtools-for-tabs": lopts.Devtools,
		"window-size":                 fmt.Sprintf("%d,%d", 800, 600),
	}
	if runtime.GOOS == "darwin" {
		f["enable-use-zoom-for-dsf"] = false
	}
	if lopts.Headless {
		f["hide-scrollbars"] = true
		f["mute-audio"] = true
		f["blink-settings"] = "primaryHoverType=2,availableHoverTypes=2,primaryPointerType=4,availablePointerTypes=4"
	}

	setFlagsFromArgs(f, lopts.Args)
	setFlagsFromK6Options(f, k6opts)

	return f
}

// makes an extension wide logger.
func makeLogger(ctx context.Context, launchOpts *common.LaunchOptions) (*common.Logger, error) {
	var (
		k6Logger            = k6lib.GetState(ctx).Logger
		reCategoryFilter, _ = regexp.Compile(launchOpts.LogCategoryFilter)
		logger              = common.NewLogger(ctx, k6Logger, launchOpts.Debug, reCategoryFilter)
	)

	// set the log level from the launch options (usually from a script's options).
	if launchOpts.Debug {
		_ = logger.SetLevel("debug")
	}
	if el, ok := os.LookupEnv("XK6_BROWSER_LOG"); ok {
		if err := logger.SetLevel(el); err != nil {
			return nil, err
		}
	}
	if _, ok := os.LookupEnv("XK6_BROWSER_CALLER"); ok {
		logger.ReportCaller()
	}
	return logger, nil
}

func setFlagsFromArgs(flags map[string]interface{}, args []string) {
	var argname, argval string
	for _, arg := range args {
		pair := strings.SplitN(arg, "=", 2)
		argname, argval = strings.TrimSpace(pair[0]), ""
		if len(pair) > 1 {
			argval = common.TrimQuotes(strings.TrimSpace(pair[1]))
		}
		flags[argname] = argval
	}
}

func setFlagsFromK6Options(flags map[string]interface{}, k6opts *k6lib.Options) {
	if k6opts == nil {
		return
	}

	hostResolver := []string{}
	if currHostResolver, ok := flags["host-resolver-rules"]; ok {
		hostResolver = append(hostResolver, fmt.Sprintf("%s", currHostResolver))
	}

	for k, v := range k6opts.Hosts {
		hostResolver = append(hostResolver, fmt.Sprintf("MAP %s %s", k, v))
	}

	if len(hostResolver) > 0 {
		sort.Strings(hostResolver)
		flags["host-resolver-rules"] = strings.Join(hostResolver, ",")
	}
}
