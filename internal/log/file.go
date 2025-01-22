package log

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/sirupsen/logrus"
	"go.k6.io/k6/internal/lib/strvals"
	"go.k6.io/k6/lib/fsext"
)

// fileHookBufferSize is a default size for the fileHook's loglines channel.
const fileHookBufferSize = 100

// fileHook is a hook to handle writing to local files.
type fileHook struct {
	fs             fsext.Fs
	fallbackLogger logrus.FieldLogger
	loglines       chan []byte
	path           string
	w              io.WriteCloser
	bw             *bufio.Writer
	levels         []logrus.Level
}

// FileHookFromConfigLine returns new fileHook hook.
func FileHookFromConfigLine(
	fs fsext.Fs, getCwd func() (string, error),
	fallbackLogger logrus.FieldLogger, line string,
) (AsyncHook, error) {
	hook := &fileHook{
		fs:             fs,
		fallbackLogger: fallbackLogger,
		levels:         logrus.AllLevels,
		loglines:       make(chan []byte, fileHookBufferSize),
	}

	logOutput, _, _ := strings.Cut(line, "=")
	if logOutput != "file" {
		return nil, fmt.Errorf("logfile configuration should be in the form `file=path-to-local-file` but is `%s`", line)
	}
	if err := hook.parseArgs(line); err != nil {
		return nil, err
	}
	if err := hook.openFile(getCwd); err != nil {
		return nil, err
	}
	return hook, nil
}

func (h *fileHook) parseArgs(line string) error {
	tokens, err := strvals.Parse(line)
	if err != nil {
		return fmt.Errorf("error while parsing logfile configuration %w", err)
	}

	for _, token := range tokens {
		switch token.Key {
		case "file":
			if token.Value == "" {
				return fmt.Errorf("filepath must not be empty")
			}
			h.path = token.Value
		case "level":
			h.levels, err = parseLevels(token.Value)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown logfile config key %s", token.Key)
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

	if _, err := h.fs.Stat(filepath.Dir(path)); errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("provided directory '%s' does not exist", filepath.Dir(path))
	}

	file, err := h.fs.OpenFile(path, syscall.O_WRONLY|syscall.O_APPEND|syscall.O_CREAT, 0o600)
	if err != nil {
		return fmt.Errorf("failed to open logfile %s: %w", path, err)
	}

	h.w = file
	h.bw = bufio.NewWriter(file)

	return nil
}

// Listen waits for log lines to flush.
func (h *fileHook) Listen(ctx context.Context) {
	for {
		select {
		case entry := <-h.loglines:
			if _, err := h.bw.Write(entry); err != nil {
				h.fallbackLogger.Errorf("failed to write a log message to a logfile: %w", err)
			}
		case <-ctx.Done():
			// This context is cancelled after the command finishes executing, so it is guaranteed that no more lines
			// will be sent to the channel. However, as it is buffered, it may still have items on it, so we drain any
			// pending log lines that may be there.
		drainloop:
			for {
				select {
				case entry := <-h.loglines:
					if _, err := h.bw.Write(entry); err != nil {
						h.fallbackLogger.Errorf("failed to write a log message to a logfile: %w", err)
					}
				default:
					break drainloop
				}
			}

			if err := h.bw.Flush(); err != nil {
				h.fallbackLogger.Errorf("failed to flush buffer: %w", err)
			}

			if err := h.w.Close(); err != nil {
				h.fallbackLogger.Errorf("failed to close logfile: %w", err)
			}

			return
		}
	}
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
