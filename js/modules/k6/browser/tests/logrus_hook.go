package tests

import (
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"
)

// logCache implements the logrus.Hook interface and could be used to check
// if log messages were outputted.
type logCache struct {
	HookedLevels []logrus.Level

	mu      sync.RWMutex
	entries []logrus.Entry
}

// Levels just returns whatever was stored in the HookedLevels slice.
func (lc *logCache) Levels() []logrus.Level {
	return lc.HookedLevels
}

// Fire saves whatever message the logrus library passed in the cache.
func (lc *logCache) Fire(e *logrus.Entry) error {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	lc.entries = append(lc.entries, *e)
	return nil
}

// assertContains checks if msg is contained in any of the cached logged events
// and fails the test if it's not. It also prints the cached log messages to
// help debugging.
func (lc *logCache) assertContains(tb testing.TB, msg string) {
	tb.Helper()

	if lc.contains(msg) {
		return
	}
	tb.Errorf("expected log cache to contain %q, but it didn't.", msg)
	lc.dump(tb)
}

// dump prints all the cached log messages to the testing.TB.
func (lc *logCache) dump(tb testing.TB) {
	tb.Helper()

	lc.mu.RLock()
	defer lc.mu.RUnlock()

	tb.Log(strings.Repeat("-", 80))
	for _, e := range lc.entries {
		tb.Log(e.Message)
	}
}

// contains returns true if msg is contained in any of the cached logged events
// or false otherwise.
func (lc *logCache) contains(msg string) bool {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	for _, evt := range lc.entries {
		if strings.Contains(evt.Message, msg) {
			return true
		}
	}
	return false
}

// attachLogCache sets logger to DebugLevel, attaches a LogCache hook and
// returns it.
func attachLogCache(tb testing.TB, fl logrus.FieldLogger) *logCache {
	tb.Helper()

	var ok bool
	var logger *logrus.Logger                  //nolint:forbidigo
	if logger, ok = fl.(*logrus.Logger); !ok { //nolint:forbidigo
		// TODO: Fix this to always work with logrus.FieldLoger.
		// See: https://go.k6.io/k6/js/modules/k6/browser/issues/818
		tb.Fatalf("logCache: unexpected logger type: %T", fl)
	}

	lc := &logCache{HookedLevels: []logrus.Level{logrus.DebugLevel, logrus.WarnLevel}}
	logger.SetLevel(logrus.DebugLevel)
	logger.AddHook(lc)
	logger.SetOutput(io.Discard)

	return lc
}
