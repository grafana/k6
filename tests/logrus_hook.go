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

package tests

import (
	"io/ioutil"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

// LogCache implements the logrus.Hook interface and could be used to check
// if log messages were outputted
type LogCache struct {
	HookedLevels []logrus.Level
	mutex        sync.RWMutex
	messageCache []logrus.Entry
}

// Levels just returns whatever was stored in the HookedLevels slice
func (lc *LogCache) Levels() []logrus.Level {
	return lc.HookedLevels
}

// Fire saves whatever message the logrus library passed in the cache
func (lc *LogCache) Fire(e *logrus.Entry) error {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()
	lc.messageCache = append(lc.messageCache, *e)
	return nil
}

// Drain returns the currently stored messages and deletes them from the cache
func (lc *LogCache) Drain() []logrus.Entry {
	lc.mutex.Lock()
	defer lc.mutex.Unlock()
	res := lc.messageCache
	lc.messageCache = []logrus.Entry{}
	return res
}

// Contains returns true if msg is contained in any of the cached logged events
// or false otherwise.
func (lc *LogCache) Contains(msg string) bool {
	lc.mutex.RLock()
	defer lc.mutex.RUnlock()
	for _, evt := range lc.messageCache {
		if strings.Contains(evt.Message, msg) {
			return true
		}
	}
	return false
}

var _ logrus.Hook = &LogCache{}

// AttachLogCache sets logger to DebugLevel, attaches a LogCache hook and
// returns it.
func AttachLogCache(logger *logrus.Logger) *LogCache {
	lc := &LogCache{HookedLevels: []logrus.Level{logrus.DebugLevel, logrus.WarnLevel}}
	logger.SetLevel(logrus.DebugLevel)
	logger.AddHook(lc)
	logger.SetOutput(ioutil.Discard)
	return lc
}
