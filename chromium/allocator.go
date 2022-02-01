/*
 *
 * xk6-browser - a browser automation extension for k6
 * Copyright (C) 2021 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package chromium

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/grafana/xk6-browser/allocation"
	"github.com/grafana/xk6-browser/common"
)

// Ensure Allocator implements the browser.Allocator interface.
var _ allocation.Allocator = &Allocator{}

type Allocator struct {
	execPath  string
	initFlags map[string]interface{}
	initEnv   []string
	tempDir   string

	wg sync.WaitGroup

	combinedOutputWriter io.Writer
}

func NewAllocator(flags map[string]interface{}, env []string) allocation.Allocator {
	a := Allocator{
		execPath:  "google-chrome",
		initFlags: flags,
		initEnv:   env,
		tempDir:   "",
	}
	a.findExecPath()
	return &a
}

func (a *Allocator) buildCmdArgs(userDataDir *string, removeDir *bool) ([]string, error) {
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
	// TODO: Refactor the creation of a temp dir out of here. This method should
	// only build command line arguments and not have side effects.
	*removeDir = false
	var ok bool
	*userDataDir, ok = a.initFlags["user-data-dir"].(string)
	if !ok {
		tempDir, err := ioutil.TempDir(a.tempDir, "xk6-browser-user-data-*")
		if err != nil {
			return nil, err
		}
		args = append(args, "--user-data-dir="+tempDir)
		*userDataDir = tempDir
		*removeDir = true
	}
	if _, ok := a.initFlags["no-sandbox"]; !ok && os.Getuid() == 0 {
		// Running as root, for example in a Linux container. Chrome
		// needs --no-sandbox when running as root, so make that the
		// default, unless the user set "no-sandbox": false.
		args = append(args, "--no-sandbox")
	}
	if _, ok := a.initFlags["remote-debugging-port"]; !ok {
		args = append(args, "--remote-debugging-port=0")
	}

	// Force the first page to be blank, instead of the welcome page;
	// --no-first-run doesn't enforce that.
	//args = append(args, "about:blank")
	//args = append(args, "--no-startup-window")
	return args, nil
}

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
		//"/Users/robin/Downloads/chrome-mac 2/Chromium.app/Contents/MacOS/Chromium",
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

// readOutput grabs the websocket address from chrome's output, returning as
// soon as it is found. All read output is forwarded to forward, if non-nil.
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

// Allocate starts a new local browser process
func (a *Allocator) Allocate(ctx context.Context, launchOpts *common.LaunchOptions) (_ *common.BrowserProcess, err error) {
	// Create cancelable context for the browser process
	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	var (
		userDataDir string
		removeDir   bool
	)
	args, err := a.buildCmdArgs(&userDataDir, &removeDir)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, a.execPath, args...)
	defer func() {
		if removeDir && cmd.Process == nil {
			// We couldn't start the process, so we didn't get to
			// the goroutine that handles RemoveAll below. Remove it
			// to not leave an empty directory.
			os.RemoveAll(userDataDir)
		}
	}()
	allocation.AllocateCmdOptions(cmd)

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

		// Delete the temporary user data directory, if needed.
		if removeDir {
			os.RemoveAll(userDataDir)
		}
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
	return common.NewBrowserProcess(ctx, cancel, cmd.Process, wsURL, userDataDir), nil
}
