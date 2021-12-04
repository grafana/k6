/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2020 Load Impact
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
)

// fileHookBufferSize is a default size for the fileHook's loglines channel.
const fileHookBufferSize = 100

// fileHook is a hook to handle writing to local files.
type fileHook struct {
	fallbackLogger logrus.FieldLogger
	loglines       chan []byte
	path           string
	w              io.WriteCloser
	bw             *bufio.Writer
	levels         []logrus.Level
}

// FileHookFromConfigLine returns new fileHook hook.
func FileHookFromConfigLine(
	ctx context.Context, fallbackLogger logrus.FieldLogger, line string,
) (logrus.Hook, error) {
	hook := &fileHook{
		fallbackLogger: fallbackLogger,
		levels:         logrus.AllLevels,
	}

	parts := strings.SplitN(line, "=", 2)
	if parts[0] != "file" {
		return nil, fmt.Errorf("logfile configuration should be in the form `file=path-to-local-file` but is `%s`", line)
	}

	if err := hook.parseArgs(line); err != nil {
		return nil, err
	}

	if err := hook.openFile(); err != nil {
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
func (h *fileHook) openFile() error {
	if _, err := os.Stat(filepath.Dir(h.path)); os.IsNotExist(err) {
		return fmt.Errorf("provided directory '%s' does not exist", filepath.Dir(h.path))
	}

	file, err := os.OpenFile(h.path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("failed to open logfile %s: %w", h.path, err)
	}

	h.w = file
	h.bw = bufio.NewWriter(file)

	return nil
}

func (h *fileHook) loop(ctx context.Context) chan []byte {
	loglines := make(chan []byte, fileHookBufferSize)

	go func() {
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
