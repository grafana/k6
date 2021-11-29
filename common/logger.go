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

package common

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/sirupsen/logrus"
)

type Logger struct {
	ctx            context.Context
	log            *logrus.Logger
	mu             sync.Mutex
	lastLogCall    int64
	debugOverride  bool
	categoryFilter *regexp.Regexp
}

func NullLogger() *logrus.Logger {
	log := logrus.New()
	log.SetOutput(ioutil.Discard)
	return log
}

func NewLogger(ctx context.Context, logger *logrus.Logger, debugOverride bool, categoryFilter *regexp.Regexp) *Logger {
	return &Logger{
		ctx:            ctx,
		log:            logger,
		debugOverride:  debugOverride,
		categoryFilter: categoryFilter,
	}
}

func (l *Logger) Tracef(category string, msg string, args ...interface{}) {
	l.Logf(logrus.TraceLevel, category, msg, args...)
}

func (l *Logger) Debugf(category string, msg string, args ...interface{}) {
	l.Logf(logrus.DebugLevel, category, msg, args...)
}

func (l *Logger) Errorf(category string, msg string, args ...interface{}) {
	l.Logf(logrus.ErrorLevel, category, msg, args...)
}

func (l *Logger) Infof(category string, msg string, args ...interface{}) {
	l.Logf(logrus.InfoLevel, category, msg, args...)
}

func (l *Logger) Warnf(category string, msg string, args ...interface{}) {
	l.Logf(logrus.WarnLevel, category, msg, args...)
}

func (l *Logger) Logf(level logrus.Level, category string, msg string, args ...interface{}) {
	// don't log if the current log level isn't in the required level.
	if l.log.GetLevel() < level {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now().UnixNano() / 1000000
	elapsed := now - l.lastLogCall
	if now == elapsed {
		elapsed = 0
	}
	defer func() {
		l.lastLogCall = now
	}()

	if l.categoryFilter != nil && !l.categoryFilter.Match([]byte(category)) {
		return
	}
	if l.log == nil {
		magenta := color.New(color.FgMagenta).SprintFunc()
		fmt.Printf("%s [%d]: %s - %s ms\n", magenta(category), goRoutineID(), string(msg), magenta(elapsed))
		return
	}
	entry := l.log.WithFields(logrus.Fields{
		"category":  category,
		"elapsed":   fmt.Sprintf("%d ms", elapsed),
		"goroutine": goRoutineID(),
	})
	if l.log.GetLevel() < level && l.debugOverride {
		entry.Printf(msg, args...)
		return
	}
	entry.Logf(level, msg, args...)
}

// SetLogLevel sets the logger level.
func (l *Logger) SetLevel(level logrus.Level) {
	l.log.SetLevel(level)
}

// SetLevelEnv sets the logger level from an environment variable if the
// variable exists.
func (l *Logger) SetLevelEnv(level string) error {
	el, ok := os.LookupEnv(level)
	if !ok {
		// don't change the level if the env var doesn't exist
		return nil
	}
	pl, err := logrus.ParseLevel(el)
	if err != nil {
		return err
	}
	l.log.SetLevel(pl)
	return nil
}

// DebugMode returns true if the logger level is set to Debug or higher.
func (l *Logger) DebugMode() bool {
	return l.log.GetLevel() >= logrus.DebugLevel
}
