package chromium

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/grafana/xk6-browser/common"
)

// Allocator provides facilities for finding, running, and interacting with a Chromium browser.
type Allocator struct {
	execPath  string                 // path to the Chromium executable
	initFlags map[string]interface{} // CLI flags to pass to the Chromium executable
	initEnv   []string               // environment variables to pass to the Chromium executable
	tempDir   string                 // path for storing the extension and user specific data

	wg sync.WaitGroup

	combinedOutputWriter io.Writer
}

// NewAllocator returns a new Allocator with a path to a Chromium executable.
func NewAllocator(flags map[string]interface{}, env []string) *Allocator {
	a := Allocator{
		execPath:  "google-chrome",
		initFlags: flags,
		initEnv:   env,
		tempDir:   "",
	}
	a.findExecPath()
	return &a
}

// parseArgs parses command-line arguments and returns them.
func (a *Allocator) parseArgs() ([]string, error) {
	// Build command line args list
	var args []string
	for name, value := range a.initFlags {
		switch value := value.(type) {
		case string:
			args = append(args, fmt.Sprintf("--%s=%s", name, value))
		case bool:
			if value {
				args = append(args, fmt.Sprintf("--%s", name))
			}
		default:
			return nil, errors.New("invalid browser command line flag")
		}
	}
	if _, ok := a.initFlags["no-sandbox"]; !ok && os.Getuid() == 0 {
		// Running as root, for example in a Linux container. Chromium
		// needs --no-sandbox when running as root, so make that the
		// default, unless the user set "no-sandbox": false.
		args = append(args, "--no-sandbox")
	}
	if _, ok := a.initFlags["remote-debugging-port"]; !ok {
		args = append(args, "--remote-debugging-port=0")
	}

	// Force the first page to be blank, instead of the welcome page;
	// --no-first-run doesn't enforce that.
	// args = append(args, "about:blank")
	// args = append(args, "--no-startup-window")
	return args, nil
}

// fincExecPath finds the path to the Chromium executable.
func (a *Allocator) findExecPath() {
	for _, path := range [...]string{
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
		filepath.Join(os.Getenv("USERPROFILE"), `AppData\Local\Google\Chrome\Application\chrome.exe`),

		// Mac (from https://commondatastorage.googleapis.com/chromium-browser-snapshots/index.html?prefix=Mac/857950/)
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
	} {
		_, err := exec.LookPath(path)
		if err == nil {
			a.execPath = path
			break
		}
	}
}

// readOutput grabs the websocket address from chrome's output and returns it.
// All read output is forwarded to forward, if non-nil.
// done is used to signal that the asynchronous io.Copy is done, if any.
func (a *Allocator) readOutput(rc io.ReadCloser, forward io.Writer, done func()) (wsURL string, _ error) {
	prefix := []byte("DevTools listening on")
	var accumulated bytes.Buffer
	bufr := bufio.NewReader(rc)
readLoop:
	for {
		line, err := bufr.ReadBytes('\n')
		if err != nil {
			return "", fmt.Errorf("chrome failed to start:\n%s", accumulated.Bytes())
		}
		if forward != nil {
			if _, err := forward.Write(line); err != nil {
				return "", err
			}
		}

		if bytes.HasPrefix(line, prefix) {
			wsURL = string(bytes.TrimSpace(line[len(prefix):]))
			break readLoop
		}
		accumulated.Write(line)
	}
	if forward == nil {
		// We don't need the process's output anymore.
		rc.Close()
	} else {
		// Copy the rest of the output in a separate goroutine, as we
		// need to return with the websocket URL.
		go func() {
			_, _ = io.Copy(forward, bufr)
			done()
		}()
	}
	return wsURL, nil
}

// Allocate starts a new Chromium browser process.
func (a *Allocator) Allocate(
	ctx context.Context, launchOpts *common.LaunchOptions,
) (_ *common.BrowserProcess, rerr error) {
	// Create cancelable context for the browser process
	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		if rerr != nil {
			cancel()
		}
	}()

	// use the provided directory or create a temporary one.
	usrDir, removeUsrDir, err := makeUserDataDir(a.tempDir, a.initFlags["user-data-dir"])
	if err != nil {
		return nil, fmt.Errorf("cannot make user temp directory: %w", err)
	}
	// add dir to flags so that parseArgs can parse it.
	a.initFlags["user-data-dir"] = usrDir

	args, err := a.parseArgs()
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, a.execPath, args...)
	defer func() {
		if cmd.Process == nil {
			removeUsrDir()
		}
	}()
	KillAfterParent(cmd)

	// Pipe stderr to stdout
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = cmd.Stdout

	// Set up environment variable for process
	if len(a.initEnv) > 0 {
		cmd.Env = append(os.Environ(), a.initEnv...)
	}

	// We must start the cmd before calling cmd.Wait, as otherwise the two
	// can run into a data race.
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	a.wg.Add(1) // for the entire allocator
	if a.combinedOutputWriter != nil {
		a.wg.Add(1) // for the io.Copy in a separate goroutine
	}
	go func() {
		_ = cmd.Wait()
		removeUsrDir()
		a.wg.Done()
	}()

	var wsURL string
	wsURLChan := make(chan struct{}, 1)
	go func() {
		wsURL, err = a.readOutput(stdout, a.combinedOutputWriter, a.wg.Done)
		wsURLChan <- struct{}{}
	}()
	select {
	case <-wsURLChan:
	case <-time.After(launchOpts.Timeout):
		err = errors.New("websocket url timeout reached")
	}
	if err != nil {
		if a.combinedOutputWriter != nil {
			// There's no io.Copy goroutine to call the done func.
			// TODO: a cleaner way to deal with this edge case?
			a.wg.Done()
		}
		return nil, err
	}
	return common.NewBrowserProcess(ctx, cancel, cmd.Process, wsURL, usrDir), nil
}

const k6BrowserTempDirPattern = "xk6-browser-user-data-*"

// makeUserDataDir creates a new temporary directory for user specific data,
// returns its path, and a remove func for removing it.
// When usrDir is not empty, no directory will be created and the remove
// function will be no-op.
func makeUserDataDir(tmpDir string, usrDir interface{}) (dir string, remove func(), _ error) {
	// used for when there is no directory to remove.
	noop := func() {}

	// use the provided dir.
	if d, ok := usrDir.(string); ok && d != "" {
		return d, noop, nil
	}

	// create a temporary dir because the provided dir is empty.
	var err error
	dir, err = os.MkdirTemp(tmpDir, k6BrowserTempDirPattern)
	if err != nil {
		return "", noop, fmt.Errorf("mkdir: %w", err)
	}

	return dir, func() { _ = os.RemoveAll(dir) }, nil
}
