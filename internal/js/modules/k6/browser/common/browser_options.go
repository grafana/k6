package common

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.k6.io/k6/internal/js/modules/k6/browser/env"
	"go.k6.io/k6/internal/js/modules/k6/browser/log"

	"go.k6.io/k6/lib/types"
)

// Script variables.
const optType = "type"

// BrowserOptions stores browser options.
type BrowserOptions struct {
	Args              []string
	Debug             bool
	ExecutablePath    string
	Headless          bool
	IgnoreDefaultArgs []string
	LogCategoryFilter string
	// TODO: Do not expose slowMo option by now.
	// See https://go.k6.io/k6/js/modules/k6/browser/issues/857.
	SlowMo  time.Duration
	Timeout time.Duration

	isRemoteBrowser bool // some options will be ignored if browser is in a remote machine
}

// NewLocalBrowserOptions returns a new BrowserOptions
// for a browser launched in the local machine.
func NewLocalBrowserOptions() *BrowserOptions {
	return &BrowserOptions{
		Headless:          true,
		LogCategoryFilter: ".*",
		Timeout:           DefaultTimeout,
	}
}

// NewRemoteBrowserOptions returns a new BrowserOptions
// for a browser running in a remote machine.
func NewRemoteBrowserOptions() *BrowserOptions {
	return &BrowserOptions{
		Headless:          true,
		LogCategoryFilter: ".*",
		Timeout:           DefaultTimeout,
		isRemoteBrowser:   true,
	}
}

// Parse parses browser options from a JS object.
func (bo *BrowserOptions) Parse(
	ctx context.Context, logger *log.Logger, opts map[string]any, envLookup env.LookupFunc,
) error {
	// Parse opts
	bt, ok := opts[optType]
	// Only 'chromium' is supported by now, so return error
	// if type option is not set, or if it's set and its value
	// is different than 'chromium'
	if !ok {
		return errors.New("browser type option must be set")
	}
	if bt != "chromium" {
		return fmt.Errorf("unsupported browser type: %s", bt)
	}

	// Parse env
	envOpts := [...]string{
		env.BrowserArguments,
		env.BrowserEnableDebugging,
		env.BrowserExecutablePath,
		env.BrowserHeadless,
		env.BrowserIgnoreDefaultArgs,
		env.LogCategoryFilter,
		env.BrowserGlobalTimeout,
	}

	for _, e := range envOpts {
		ev, ok := envLookup(e)
		if !ok || ev == "" {
			continue
		}
		if bo.shouldIgnoreIfBrowserIsRemote(e) {
			logger.Warnf("BrowserOptions", "setting %s option is disallowed when browser is remote", e)
			continue
		}
		var err error
		switch e {
		case env.BrowserArguments:
			bo.Args = parseListOpt(ev)
		case env.BrowserEnableDebugging:
			bo.Debug, err = parseBoolOpt(e, ev)
		case env.BrowserExecutablePath:
			bo.ExecutablePath = ev
		case env.BrowserHeadless:
			bo.Headless, err = parseBoolOpt(e, ev)
		case env.BrowserIgnoreDefaultArgs:
			bo.IgnoreDefaultArgs = parseListOpt(ev)
		case env.LogCategoryFilter:
			bo.LogCategoryFilter = ev
		case env.BrowserGlobalTimeout:
			bo.Timeout, err = parseTimeOpt(e, ev)
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func (bo *BrowserOptions) shouldIgnoreIfBrowserIsRemote(opt string) bool {
	if !bo.isRemoteBrowser {
		return false
	}

	shouldIgnoreIfBrowserIsRemote := map[string]struct{}{
		env.BrowserArguments:         {},
		env.BrowserExecutablePath:    {},
		env.BrowserHeadless:          {},
		env.BrowserIgnoreDefaultArgs: {},
	}
	_, ignore := shouldIgnoreIfBrowserIsRemote[opt]

	return ignore
}

func parseBoolOpt(k, v string) (bool, error) {
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s should be a boolean", k)
	}

	return b, nil
}

func parseTimeOpt(k, v string) (time.Duration, error) {
	t, err := types.GetDurationValue(v)
	if err != nil {
		return time.Duration(0), fmt.Errorf("%s should be a time duration value: %w", k, err)
	}

	return t, nil
}

func parseListOpt(v string) []string {
	elems := strings.Split(v, ",")
	// If last element is a void string,
	// because value contained an ending comma,
	// remove it
	if elems[len(elems)-1] == "" {
		elems = elems[:len(elems)-1]
	}

	return elems
}
