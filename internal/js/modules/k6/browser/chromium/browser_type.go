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

	"go.k6.io/k6/internal/js/modules/k6/browser/common"
	"go.k6.io/k6/internal/js/modules/k6/browser/env"
	"go.k6.io/k6/internal/js/modules/k6/browser/k6ext"
	"go.k6.io/k6/internal/js/modules/k6/browser/log"
	"go.k6.io/k6/internal/js/modules/k6/browser/storage"

	k6modules "go.k6.io/k6/js/modules"
	k6lib "go.k6.io/k6/lib"
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
	randSrc      *rand.Rand
	envLookupper env.LookupFunc
}

// NewBrowserType registers our custom k6 metrics, creates method mappings on
// the sobek runtime, and returns a new Chrome browser type.
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
//
// vuCtx is the context coming from the VU itself. The k6 vu/iteration controls
// its lifecycle.
//
// context.background() is used when connecting to an instance of chromium. The
// connection lifecycle should be handled by the k6 event system.
//
// The separation is important to allow for the iteration to end when k6 requires
// the iteration to end (e.g. during a SIGTERM) and unblocks k6 to then fire off
// the events which allows the connection to close.
func (b *BrowserType) Connect(ctx, vuCtx context.Context, wsEndpoint string) (*common.Browser, error) {
	vuCtx, browserOpts, logger, err := b.init(vuCtx, true)
	if err != nil {
		return nil, fmt.Errorf("initializing browser type: %w", err)
	}

	bp, err := b.connect(ctx, vuCtx, wsEndpoint, browserOpts, logger)
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
	ctx, vuCtx context.Context, wsURL string, opts *common.BrowserOptions, logger *log.Logger,
) (*common.Browser, error) {
	browserProc, err := b.link(ctx, wsURL, logger)
	if browserProc == nil {
		return nil, fmt.Errorf("connecting to browser: %w", err)
	}

	// If this context is cancelled we'll initiate an extension wide
	// cancellation and shutdown.
	browserCtx, browserCtxCancel := context.WithCancel(vuCtx)
	b.Ctx = browserCtx
	browser, err := common.NewBrowser(
		ctx, browserCtx, browserCtxCancel, browserProc, opts, logger,
	)
	if err != nil {
		return nil, fmt.Errorf("connecting to browser: %w", err)
	}

	return browser, nil
}

func (b *BrowserType) link(
	ctx context.Context,
	wsURL string, logger *log.Logger,
) (*common.BrowserProcess, error) {
	bProcCtx, bProcCtxCancel := context.WithCancel(ctx)
	p, err := common.NewRemoteBrowserProcess(bProcCtx, wsURL, bProcCtxCancel, logger)
	if err != nil {
		bProcCtxCancel()
		return nil, err //nolint:wrapcheck
	}

	return p, nil
}

// Launch allocates a new Chrome browser process and returns a new Browser value,
// which can be used for controlling the Chrome browser.
//
// vuCtx is the context coming from the VU itself. The k6 vu/iteration controls
// its lifecycle.
//
// context.background() is used when launching an instance of chromium. The
// chromium lifecycle should be handled by the k6 event system.
//
// The separation is important to allow for the iteration to end when k6 requires
// the iteration to end (e.g. during a SIGTERM) and unblocks k6 to then fire off
// the events which allows the chromium subprocess to shutdown.
func (b *BrowserType) Launch(ctx, vuCtx context.Context) (_ *common.Browser, browserProcessID int, _ error) {
	vuCtx, browserOpts, logger, err := b.init(vuCtx, false)
	if err != nil {
		return nil, 0, fmt.Errorf("initializing browser type: %w", err)
	}

	bp, pid, err := b.launch(ctx, vuCtx, browserOpts, logger)
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
	ctx, vuCtx context.Context, opts *common.BrowserOptions, logger *log.Logger,
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

	path, err := executablePath(opts.ExecutablePath, b.envLookupper, exec.LookPath)
	if err != nil {
		return nil, 0, fmt.Errorf("finding browser executable: %w", err)
	}

	browserProc, err := b.allocate(ctx, path, flags, dataDir, logger)
	if browserProc == nil {
		return nil, 0, fmt.Errorf("launching browser: %w", err)
	}

	// If this context is cancelled we'll initiate an extension wide
	// cancellation and shutdown.
	browserCtx, browserCtxCancel := context.WithCancel(vuCtx)
	b.Ctx = browserCtx
	browser, err := common.NewBrowser(ctx, browserCtx, browserCtxCancel,
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

// Name returns the name of this browser type.
func (b *BrowserType) Name() string {
	return "chromium"
}

// allocate starts a new Chromium browser process and returns it.
func (b *BrowserType) allocate(
	ctx context.Context,
	path string,
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

	return common.NewLocalBrowserProcess(bProcCtx, path, args, dataDir, bProcCtxCancel, logger) //nolint: wrapcheck
}

var (
	// ErrChromeNotInstalled is returned when the Chrome executable is not found.
	ErrChromeNotInstalled = errors.New(
		"k6 couldn't detect google chrome or a chromium-supported browser on this system",
	)

	// ErrChromeNotFoundAtPath is returned when the Chrome executable is not found at the given path.
	ErrChromeNotFoundAtPath = errors.New(
		"k6 couldn't detect google chrome or a chromium-supported browser on the given path",
	)
)

// executablePath returns the path where the extension expects to find the browser executable.
func executablePath(
	path string,
	env env.LookupFunc,
	lookPath func(file string) (string, error), // os.LookPath
) (string, error) {
	// find the browser executable in the user provided path
	if path := strings.TrimSpace(path); path != "" {
		if _, err := lookPath(path); err == nil {
			return path, nil
		}
		return "", fmt.Errorf("%w: %s", ErrChromeNotFoundAtPath, path)
	}

	// find the browser executable in the default paths below
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
	// find the browser executable in the user profile
	if userProfile, ok := env("USERPROFILE"); ok {
		paths = append(paths, filepath.Join(userProfile, `AppData\Local\Google\Chrome\Application\chrome.exe`))
	}
	for _, path := range paths {
		if _, err := lookPath(path); err == nil {
			return path, nil
		}
	}

	return "", ErrChromeNotInstalled
}

// parseArgs parses command-line arguments and returns them.
func parseArgs(flags map[string]any) ([]string, error) {
	// Build command line args list
	var args []string
	for name, value := range flags {
		switch value := value.(type) {
		case string:
			args = append(args, parseStringArg(name, value))
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

func parseStringArg(flag string, value string) string {
	if strings.TrimSpace(value) == "" {
		// If the value is empty, we don't include it in the args list.
		// Otherwise, it will produce "--name=" which is invalid.
		return fmt.Sprintf("--%s", flag)
	}
	return fmt.Sprintf("--%s=%s", flag, value)
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
