// Package log implements various logrus hooks.
package log

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
)

// fileHookBufferSize is a default size for the fileHook's loglines channel.
const fileHookBufferSize = 100

// fileHook is a hook to handle writing to local files.
type fileHook struct {
	fs             afero.Fs
	fallbackLogger logrus.FieldLogger
	loglines       chan []byte
	path           string
	w              io.WriteCloser
	bw             *bufio.Writer
	levels         []logrus.Level
	done           chan struct{}
}

// FileHookFromConfigLine returns new fileHook hook.
func FileHookFromConfigLine(
	ctx context.Context, fs afero.Fs, getCwd func() (string, error),
	fallbackLogger logrus.FieldLogger, line string, done chan struct{},
) (logrus.Hook, error) {
	hook := &fileHook{
		fs:             fs,
		fallbackLogger: fallbackLogger,
		levels:         logrus.AllLevels,
		done:           done,
	}

	parts := strings.SplitN(line, "=", 2)
	if parts[0] != "file" {
		return nil, fmt.Errorf("logfile configuration should be in the form `file=path-to-local-file` but is `%s`", line)
	}

	if err := hook.parseArgs(line); err != nil {
		return nil, err
	}

	if err := hook.openFile(getCwd); err != nil {
		return nil, err
	}

	hook.loglines = hook.loop(ctx)

	return hook, nil
}

func (h *fileHook) parseArgs(line string) error {
	tokens, err := tokenize(line)
	if err != nil {
		return fmt.Errorf("error while parsing logfile configuration %w", err)
	}

	for _, token := range tokens {
		switch token.key {
		case "file":
			if token.value == "" {
				return fmt.Errorf("filepath must not be empty")
			}
			h.path = token.value
		case "level":
			h.levels, err = parseLevels(token.value)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown logfile config key %s", token.key)
		}
	}

	return nil
}

// openFile opens logfile and initializes writers.
func (h *fileHook) openFile(getCwd func() (string, error)) error {
	path := h.path
	if !filepath.IsAbs(path) {
		cwd, err := getCwd()
		if err != nil {
			return fmt.Errorf("'%s' is a relative path but could not determine CWD: %w", path, err)
		}
		path = filepath.Join(cwd, path)
	}

	if _, err := h.fs.Stat(filepath.Dir(path)); os.IsNotExist(err) {
		return fmt.Errorf("provided directory '%s' does not exist", filepath.Dir(path))
	}

	file, err := h.fs.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("failed to open logfile %s: %w", path, err)
	}

	h.w = file
	h.bw = bufio.NewWriter(file)

	return nil
}

func (h *fileHook) loop(ctx context.Context) chan []byte {
	loglines := make(chan []byte, fileHookBufferSize)

	go func() {
		defer close(h.done)
		for {
			select {
			case entry := <-loglines:
				if _, err := h.bw.Write(entry); err != nil {
					h.fallbackLogger.Errorf("failed to write a log message to a logfile: %w", err)
				}
			case <-ctx.Done():
				if err := h.bw.Flush(); err != nil {
					h.fallbackLogger.Errorf("failed to flush buffer: %w", err)
				}

				if err := h.w.Close(); err != nil {
					h.fallbackLogger.Errorf("failed to close logfile: %w", err)
				}

				return
			}
		}
	}()

	return loglines
}

// Fire writes the log file to defined path.
func (h *fileHook) Fire(entry *logrus.Entry) error {
	message, err := entry.Bytes()
	if err != nil {
		return fmt.Errorf("failed to get a log entry bytes: %w", err)
	}

	h.loglines <- message
	return nil
}

// Levels returns configured log levels.
func (h *fileHook) Levels() []logrus.Level {
	return h.levels
}
