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
	"regexp"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/sirupsen/logrus"
)

type Logger struct {
	ctx            context.Context
	logger         *logrus.Logger
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
	l := Logger{
		ctx:            ctx,
		logger:         logger,
		mu:             sync.Mutex{},
		debugOverride:  debugOverride,
		categoryFilter: categoryFilter,
		lastLogCall:    0,
	}
	return &l
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
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now().UnixNano() / 1000000
	elapsed := now - l.lastLogCall
	if now == elapsed {
		elapsed = 0
	}

	if l.categoryFilter == nil || l.categoryFilter.Match([]byte(category)) {
		magenta := color.New(color.FgMagenta).SprintFunc()
		if l.logger != nil {
			entry := l.logger.WithFields(logrus.Fields{
				"category":  magenta(category),
				"elapsed":   fmt.Sprintf("%d ms", elapsed),
				"goroutine": goRoutineID(),
			})
			if l.logger.GetLevel() < level && l.debugOverride {
				entry.Printf(msg, args...)
			} else {
				entry.Logf(level, msg, args...)
			}
		} else {
			fmt.Printf("%s [%d]: %s - %s ms\n", magenta(category), goRoutineID(), string(msg), magenta(elapsed))
		}
		l.lastLogCall = now
	}
}
