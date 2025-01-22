package testutils

import (
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

// SimpleLogrusHook implements the logrus.Hook interface and could be used to check
// if log messages were outputted
type SimpleLogrusHook struct {
	HookedLevels []logrus.Level
	mutex        sync.Mutex
	messageCache []logrus.Entry
}

// Levels just returns whatever was stored in the HookedLevels slice
func (smh *SimpleLogrusHook) Levels() []logrus.Level {
	return smh.HookedLevels
}

// Fire saves whatever message the logrus library passed in the cache
func (smh *SimpleLogrusHook) Fire(e *logrus.Entry) error {
	smh.mutex.Lock()
	defer smh.mutex.Unlock()
	smh.messageCache = append(smh.messageCache, *e)
	return nil
}

// Drain returns the currently stored messages and deletes them from the cache
func (smh *SimpleLogrusHook) Drain() []logrus.Entry {
	smh.mutex.Lock()
	defer smh.mutex.Unlock()
	res := smh.messageCache
	smh.messageCache = []logrus.Entry{}
	return res
}

// Lines returns the logged lines.
func (smh *SimpleLogrusHook) Lines() []string {
	entries := smh.Drain()
	lines := make([]string, len(entries))
	for i, entry := range entries {
		lines[i] = entry.Message
	}
	return lines
}

// Reset clears the internal entry buffer.
func (smh *SimpleLogrusHook) Reset() {
	smh.mutex.Lock()
	defer smh.mutex.Unlock()
	smh.messageCache = []logrus.Entry{}
}

// LastEntry returns the last entry that was logged (or nil, if no messages were
// logged or remain).
func (smh *SimpleLogrusHook) LastEntry() *logrus.Entry {
	smh.mutex.Lock()
	defer smh.mutex.Unlock()
	i := len(smh.messageCache) - 1
	if i < 0 {
		return nil
	}
	return &smh.messageCache[i]
}

var _ logrus.Hook = &SimpleLogrusHook{}

// NewLogHook creates a new SimpleLogrusHook with the given levels and returns
// it. If no levels are specified, then logrus.AllLevels will be used.
func NewLogHook(levels ...logrus.Level) *SimpleLogrusHook {
	if len(levels) == 0 {
		levels = logrus.AllLevels
	}
	return &SimpleLogrusHook{HookedLevels: levels}
}

// LogContains is a helper function that checks the provided list of log entries
// for a message matching the provided level and contents.
func LogContains(logEntries []logrus.Entry, expLevel logrus.Level, expContents string) bool {
	for _, entry := range logEntries {
		if entry.Level == expLevel && strings.Contains(entry.Message, expContents) {
			return true
		}
	}
	return false
}

// FilterEntries is a helper function that checks the provided list of log entries
// for a messages matching the provided level and contents and returns an array with only them.
func FilterEntries(logEntries []logrus.Entry, expLevel logrus.Level, expContents string) []logrus.Entry {
	filtered := make([]logrus.Entry, 0)
	for _, entry := range logEntries {
		if entry.Level == expLevel && strings.Contains(entry.Message, expContents) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}
