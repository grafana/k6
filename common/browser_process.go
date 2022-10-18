package common

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/grafana/xk6-browser/log"
	"github.com/grafana/xk6-browser/storage"
)

type BrowserProcess struct {
	ctx    context.Context
	cancel context.CancelFunc

	// The process of the browser, if running locally.
	process *os.Process

	// Channels for managing termination.
	lostConnection             chan struct{}
	processIsGracefullyClosing chan struct{}

	// Browser's WebSocket URL to speak CDP
	wsURL string

	// The directory where user data for the browser is stored.
	userDataDir *storage.Dir

	logger *log.Logger
}

func NewBrowserProcess(
	ctx context.Context, path string, args, env []string, dataDir *storage.Dir,
	ctxCancel context.CancelFunc, logger *log.Logger,
) (*BrowserProcess, error) {
	procCtx, procCtxCancel := context.WithCancel(ctx)
	cmd, stdout, err := execute(
		procCtx, procCtxCancel, path, args, env, dataDir, logger)
	if err != nil {
		procCtxCancel()
		return nil, err
	}

	wsURL, err := parseDevToolsURL(procCtx, stdout)
	if err != nil {
		procCtxCancel()
		return nil, err
	}

	p := BrowserProcess{
		ctx:                        procCtx,
		cancel:                     ctxCancel,
		process:                    cmd.Process,
		lostConnection:             make(chan struct{}),
		processIsGracefullyClosing: make(chan struct{}),
		wsURL:                      wsURL,
		userDataDir:                dataDir,
	}

	go func() {
		// If we lose connection to the browser and we're not in-progress with clean
		// browser-initiated termination then cancel the context to clean up.
		select {
		case <-p.lostConnection:
		case <-procCtx.Done():
		}

		select {
		case <-p.processIsGracefullyClosing:
		default:
			p.cancel()
		}
	}()

	return &p, nil
}

func (p *BrowserProcess) didLoseConnection() {
	close(p.lostConnection)
}

func (p *BrowserProcess) isConnected() bool {
	var ok bool
	select {
	case _, ok = <-p.lostConnection:
	default:
		ok = true
	}
	return ok
}

// GracefulClose triggers a graceful closing of the browser process.
func (p *BrowserProcess) GracefulClose() {
	p.logger.Debugf("Browser:GracefulClose", "")
	close(p.processIsGracefullyClosing)
}

// Terminate triggers the termination of the browser process.
func (p *BrowserProcess) Terminate() {
	p.logger.Debugf("Browser:Close", "browserProc terminate")
	p.cancel()
}

// WsURL returns the Websocket URL that the browser is listening on for CDP clients.
func (p *BrowserProcess) WsURL() string {
	return p.wsURL
}

// Pid returns the browser process ID.
func (p *BrowserProcess) Pid() int {
	return p.process.Pid
}

// AttachLogger attaches a logger to the browser process.
func (p *BrowserProcess) AttachLogger(logger *log.Logger) {
	p.logger = logger
}

func execute(
	ctx context.Context, ctxCancel func(), path string, args, env []string,
	dataDir *storage.Dir, logger *log.Logger,
) (*exec.Cmd, io.Reader, error) {
	cmd := exec.CommandContext(ctx, path, args...)
	killAfterParent(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("%w", err)
	}
	cmd.Stderr = cmd.Stdout

	// Set up environment variable for process
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}

	// We must start the cmd before calling cmd.Wait, as otherwise the two
	// can run into a data race.
	err = cmd.Start()
	if os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("file does not exist: %s", path)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("%w", err)
	}
	if ctx.Err() != nil {
		return nil, nil, fmt.Errorf("%w", ctx.Err())
	}

	go func() {
		// TODO: How to handle these errors?
		defer func() {
			if err := dataDir.Cleanup(); err != nil {
				logger.Errorf("browser", "cleaning up the user data directory: %v", err)
			}
			ctxCancel()
		}()

		if err := cmd.Wait(); err != nil {
			logger.Errorf("browser",
				"process with PID %d unexpectedly ended: %v",
				cmd.Process.Pid, err)
		}
	}()

	return cmd, stdout, nil
}

// parseDevToolsURL grabs the websocket address from chrome's output and returns it.
func parseDevToolsURL(ctx context.Context, rc io.Reader) (wsURL string, _ error) {
	type result struct {
		devToolsURL string
		err         error
	}
	c := make(chan result, 1)
	go func() {
		const prefix = "DevTools listening on "

		scanner := bufio.NewScanner(rc)
		for scanner.Scan() {
			if s := scanner.Text(); strings.HasPrefix(s, prefix) {
				c <- result{
					strings.TrimPrefix(strings.TrimSpace(s), prefix),
					nil,
				}
				return
			}
		}
		if err := scanner.Err(); err != nil {
			c <- result{"", err}
		}
	}()
	select {
	case r := <-c:
		return r.devToolsURL, r.err
	case <-ctx.Done():
		return "", fmt.Errorf("%w", ctx.Err())
	}
}
