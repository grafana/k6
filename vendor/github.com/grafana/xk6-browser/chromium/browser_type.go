package chromium

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/grafana/xk6-browser/api"
	"github.com/grafana/xk6-browser/common"
	"github.com/grafana/xk6-browser/env"
	"github.com/grafana/xk6-browser/k6ext"
	"github.com/grafana/xk6-browser/log"
	"github.com/grafana/xk6-browser/storage"

	k6common "go.k6.io/k6/js/common"
	k6modules "go.k6.io/k6/js/modules"
	k6lib "go.k6.io/k6/lib"

	"github.com/dop251/goja"
)

// BrowserType provides methods to launch a Chrome browser instance or connect to an existing one.
// It's the entry point for interacting with the browser.
type BrowserType struct {
	// FIXME: This is only exported because testBrowser needs it. Contexts
	// shouldn't be stored on structs if we can avoid it.
	Ctx          context.Context
	vu           k6modules.VU
	hooks        *common.Hooks
	k6Metrics    *k6ext.CustomMetrics
	execPath     string // path to the Chromium executable
	randSrc      *rand.Rand
	envLookupper env.LookupFunc
}

// NewBrowserType registers our custom k6 metrics, creates method mappings on
// the goja runtime, and returns a new Chrome browser type.
func NewBrowserType(vu k6modules.VU) *BrowserType {
	// NOTE: vu.InitEnv() *must* be called from the script init scope,
	// otherwise it will return nil.
	env := vu.InitEnv()

	return &BrowserType{
		vu:           vu,
		hooks:        common.NewHooks(),
		k6Metrics:    k6ext.RegisterCustomMetrics(env.Registry),
		randSrc:      rand.New(rand.NewSource(time.Now().UnixNano())), //nolint: gosec
		envLookupper: env.LookupEnv,
	}
}

func (b *BrowserType) init(
	ctx context.Context, isRemoteBrowser bool,
) (context.Context, *common.BrowserOptions, *log.Logger, error) {
	ctx = b.initContext(ctx)

	logger, err := makeLogger(ctx, b.envLookupper)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error setting up logger: %w", err)
	}

	var browserOpts *common.BrowserOptions
	if isRemoteBrowser {
		browserOpts = common.NewRemoteBrowserOptions()
	} else {
		browserOpts = common.NewLocalBrowserOptions()
	}

	opts := k6ext.GetScenarioOpts(b.vu.Context(), b.vu)
	if err = browserOpts.Parse(ctx, logger, opts, b.envLookupper); err != nil {
		return nil, nil, nil, fmt.Errorf("error parsing browser options: %w", err)
	}
	ctx = common.WithBrowserOptions(ctx, browserOpts)

	if err := logger.SetCategoryFilter(browserOpts.LogCategoryFilter); err != nil {
		return nil, nil, nil, fmt.Errorf("error setting category filter: %w", err)
	}
	if browserOpts.Debug {
		_ = logger.SetLevel("debug")
	}

	return ctx, browserOpts, logger, nil
}

func (b *BrowserType) initContext(ctx context.Context) context.Context {
	ctx = k6ext.WithVU(ctx, b.vu)
	ctx = k6ext.WithCustomMetrics(ctx, b.k6Metrics)
	ctx = common.WithHooks(ctx, b.hooks)
	ctx = common.WithIterationID(ctx, fmt.Sprintf("%x", b.randSrc.Uint64()))
	return ctx
}

// Connect attaches k6 browser to an existing browser instance.
func (b *BrowserType) Connect(ctx context.Context, wsEndpoint string) (api.Browser, error) {
	ctx, browserOpts, logger, err := b.init(ctx, true)
	if err != nil {
		return nil, fmt.Errorf("initializing browser type: %w", err)
	}

	bp, err := b.connect(ctx, wsEndpoint, browserOpts, logger)
	if err != nil {
		err = &k6ext.UserFriendlyError{
			Err:     err,
			Timeout: browserOpts.Timeout,
		}
		return nil, fmt.Errorf("%w", err)
	}

	return bp, nil
}

func (b *BrowserType) connect(
	ctx context.Context, wsURL string, opts *common.BrowserOptions, logger *log.Logger,
) (*common.Browser, error) {
	browserProc, err := b.link(ctx, wsURL, logger)
	if browserProc == nil {
		return nil, fmt.Errorf("connecting to browser: %w", err)
	}

	// If this context is cancelled we'll initiate an extension wide
	// cancellation and shutdown.
	browserCtx, browserCtxCancel := context.WithCancel(ctx)
	b.Ctx = browserCtx
	browser, err := common.NewBrowser(
		browserCtx, browserCtxCancel, browserProc, opts, logger,
	)
	if err != nil {
		return nil, fmt.Errorf("connecting to browser: %w", err)
	}

	return browser, nil
}

func (b *BrowserType) link(
	ctx context.Context, wsURL string, logger *log.Logger,
) (*common.BrowserProcess, error) {
	bProcCtx, bProcCtxCancel := context.WithCancel(ctx)
	p, err := common.NewRemoteBrowserProcess(bProcCtx, wsURL, bProcCtxCancel, logger)
	if err != nil {
		bProcCtxCancel()
		return nil, err //nolint:wrapcheck
	}

	return p, nil
}

// Launch allocates a new Chrome browser process and returns a new api.Browser value,
// which can be used for controlling the Chrome browser.
func (b *BrowserType) Launch(ctx context.Context) (_ api.Browser, browserProcessID int, _ error) {
	ctx, browserOpts, logger, err := b.init(ctx, false)
	if err != nil {
		return nil, 0, fmt.Errorf("initializing browser type: %w", err)
	}

	bp, pid, err := b.launch(ctx, browserOpts, logger)
	if err != nil {
		err = &k6ext.UserFriendlyError{
			Err:     err,
			Timeout: browserOpts.Timeout,
		}
		return nil, 0, fmt.Errorf("%w", err)
	}

	return bp, pid, nil
}

func (b *BrowserType) launch(
	ctx context.Context, opts *common.BrowserOptions, logger *log.Logger,
) (_ *common.Browser, pid int, _ error) {
	flags, err := prepareFlags(opts, &(b.vu.State()).Options)
	if err != nil {
		return nil, 0, fmt.Errorf("%w", err)
	}

	dataDir := &storage.Dir{}
	if err := dataDir.Make(b.tmpdir(), flags["user-data-dir"]); err != nil {
		return nil, 0, fmt.Errorf("%w", err)
	}
	flags["user-data-dir"] = dataDir.Dir

	browserProc, err := b.allocate(ctx, opts, flags, dataDir, logger)
	if browserProc == nil {
		return nil, 0, fmt.Errorf("launching browser: %w", err)
	}

	// If this context is cancelled we'll initiate an extension wide
	// cancellation and shutdown.
	browserCtx, browserCtxCancel := context.WithCancel(ctx)
	b.Ctx = browserCtx
	browser, err := common.NewBrowser(browserCtx, browserCtxCancel,
		browserProc, opts, logger)
	if err != nil {
		return nil, 0, fmt.Errorf("launching browser: %w", err)
	}

	return browser, browserProc.Pid(), nil
}

// tmpdir returns the temporary directory to use for the browser.
// It returns the value of the TMPDIR environment variable if set,
// otherwise it returns an empty string.
func (b *BrowserType) tmpdir() string {
	dir, _ := b.envLookupper("TMPDIR")
	return dir
}

// LaunchPersistentContext launches the browser with persistent storage.
func (b *BrowserType) LaunchPersistentContext(userDataDir string, opts goja.Value) api.Browser {
	rt := b.vu.Runtime()
	k6common.Throw(rt, errors.New("BrowserType.LaunchPersistentContext(userDataDir, opts) has not been implemented yet"))
	return nil
}

// Name returns the name of this browser type.
func (b *BrowserType) Name() string {
	return "chromium"
}

// allocate starts a new Chromium browser process and returns it.
func (b *BrowserType) allocate(
	ctx context.Context, opts *common.BrowserOptions,
	flags map[string]any, dataDir *storage.Dir,
	logger *log.Logger,
) (_ *common.BrowserProcess, rerr error) {
	bProcCtx, bProcCtxCancel := context.WithCancel(ctx)
	defer func() {
		if rerr != nil {
			bProcCtxCancel()
		}
	}()

	args, err := parseArgs(flags)
	if err != nil {
		return nil, err
	}

	path := opts.ExecutablePath
	if path == "" {
		path = b.ExecutablePath()
	}

	return common.NewLocalBrowserProcess(bProcCtx, path, args, dataDir, bProcCtxCancel, logger) //nolint: wrapcheck
}

// ExecutablePath returns the path where the extension expects to find the browser executable.
func (b *BrowserType) ExecutablePath() (execPath string) {
	if b.execPath != "" {
		return b.execPath
	}
	defer func() {
		b.execPath = execPath
	}()

	paths := []string{
		// Unix-like
		"headless_shell",
		"headless-shell",
		"chromium",
		"chromium-browser",
		"google-chrome",
		"google-chrome-stable",
		"google-chrome-beta",
		"google-chrome-unstable",
		"/usr/bin/google-chrome",
		// Windows
		"chrome",
		"chrome.exe", // in case PATHEXT is misconfigured
		`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files\Google\Chrome\Application\chrome.exe`,
		// Mac (from https://commondatastorage.googleapis.com/chromium-browser-snapshots/index.html?prefix=Mac/857950/)
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
	}
	if userProfile, ok := b.envLookupper("USERPROFILE"); ok {
		paths = append(paths, filepath.Join(userProfile, `AppData\Local\Google\Chrome\Application\chrome.exe`))
	}
	for _, path := range paths {
		if _, err := exec.LookPath(path); err == nil {
			return path
		}
	}

	return ""
}

// parseArgs parses command-line arguments and returns them.
func parseArgs(flags map[string]any) ([]string, error) {
	// Build command line args list
	var args []string
	for name, value := range flags {
		switch value := value.(type) {
		case string:
			args = append(args, fmt.Sprintf("--%s=%s", name, value))
		case bool:
			if value {
				args = append(args, fmt.Sprintf("--%s", name))
			}
		default:
			return nil, fmt.Errorf(`invalid browser command line flag: "%s=%v"`, name, value)
		}
	}
	if _, ok := flags["remote-debugging-port"]; !ok {
		args = append(args, "--remote-debugging-port=0")
	}

	// Force the first page to be blank, instead of the welcome page;
	// --no-first-run doesn't enforce that.
	// args = append(args, common.BlankPage)
	// args = append(args, "--no-startup-window")
	return args, nil
}

func prepareFlags(lopts *common.BrowserOptions, k6opts *k6lib.Options) (map[string]any, error) {
	// After Puppeteer's and Playwright's default behavior.
	f := map[string]any{
		"disable-background-networking":                      true,
		"enable-features":                                    "NetworkService,NetworkServiceInProcess",
		"disable-background-timer-throttling":                true,
		"disable-backgrounding-occluded-windows":             true,
		"disable-breakpad":                                   true,
		"disable-component-extensions-with-background-pages": true,
		"disable-default-apps":                               true,
		"disable-dev-shm-usage":                              true,
		"disable-extensions":                                 true,
		//nolint:lll
		"disable-features":                "ImprovedCookieControls,LazyFrameLoading,GlobalMediaControls,DestroyProfileOnBrowserClose,MediaRouter,AcceptCHFrame",
		"disable-hang-monitor":            true,
		"disable-ipc-flooding-protection": true,
		"disable-popup-blocking":          true,
		"disable-prompt-on-repost":        true,
		"disable-renderer-backgrounding":  true,
		"force-color-profile":             "srgb",
		"metrics-recording-only":          true,
		"no-first-run":                    true,
		"enable-automation":               true,
		"password-store":                  "basic",
		"use-mock-keychain":               true,
		"no-service-autorun":              true,

		"no-startup-window":        true,
		"no-default-browser-check": true,
		"headless":                 lopts.Headless,
		"window-size":              fmt.Sprintf("%d,%d", 800, 600),
	}
	if lopts.Headless {
		f["hide-scrollbars"] = true
		f["mute-audio"] = true
		f["blink-settings"] = "primaryHoverType=2,availableHoverTypes=2,primaryPointerType=4,availablePointerTypes=4"
	}
	ignoreDefaultArgsFlags(f, lopts.IgnoreDefaultArgs)

	setFlagsFromArgs(f, lopts.Args)
	if err := setFlagsFromK6Options(f, k6opts); err != nil {
		return nil, err
	}

	return f, nil
}

// ignoreDefaultArgsFlags ignores any flags in the provided slice.
func ignoreDefaultArgsFlags(flags map[string]any, toIgnore []string) {
	for _, name := range toIgnore {
		delete(flags, strings.TrimPrefix(name, "--"))
	}
}

// setFlagsFromArgs fills flags by parsing the args slice.
// This is used for passing the "arg=value" arguments along with other launch options
// when launching a new Chrome browser.
func setFlagsFromArgs(flags map[string]any, args []string) {
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

// setFlagsFromK6Options adds additional data to flags considering the k6 options.
// Such as: "host-resolver-rules" for blocking requests.
func setFlagsFromK6Options(flags map[string]any, k6opts *k6lib.Options) error {
	if k6opts == nil {
		return nil
	}

	hostResolver := []string{}
	if currHostResolver, ok := flags["host-resolver-rules"]; ok {
		hostResolver = append(hostResolver, fmt.Sprintf("%s", currHostResolver))
	}

	// Add the host resolver rules.
	//
	// This is done by marshaling the k6 hosts option to JSON and then
	// unmarshaling it to a map[string]string. This is done because the
	// k6 v0.42 changed Hosts from a map to types.NullHosts and doesn't
	// expose the map anymore.
	//
	// TODO: A better way to do this would be to handle the resolver
	// rules by communicating with Chromium (and then using Hosts's
	// Match method) instead of passing the rules via the command line
	// to Chromium.
	var rules map[string]string
	b, err := json.Marshal(k6opts.Hosts)
	if err != nil {
		return fmt.Errorf("marshaling hosts option: %w", err)
	}
	if err := json.Unmarshal(b, &rules); err != nil {
		return fmt.Errorf("unmarshaling hosts option: %w", err)
	}
	for k, v := range rules {
		hostResolver = append(hostResolver, fmt.Sprintf("MAP %s %s", k, v))
	}
	if len(hostResolver) > 0 {
		sort.Strings(hostResolver)
		flags["host-resolver-rules"] = strings.Join(hostResolver, ",")
	}

	return nil
}

// makeLogger makes and returns an extension wide logger.
func makeLogger(ctx context.Context, envLookup env.LookupFunc) (*log.Logger, error) {
	var (
		k6Logger = k6ext.GetVU(ctx).State().Logger
		logger   = log.New(k6Logger, common.GetIterationID(ctx))
	)
	if el, ok := envLookup(env.LogLevel); ok {
		if logger.SetLevel(el) != nil {
			return nil, fmt.Errorf(
				"invalid log level %q, should be one of: panic, fatal, error, warn, warning, info, debug, trace",
				el,
			)
		}
	}
	if _, ok := envLookup(env.LogCaller); ok {
		logger.ReportCaller()
	}

	return logger, nil
}
