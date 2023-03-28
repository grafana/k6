package testutils

import (
	"io"
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

var _ logrus.Hook = &SimpleLogrusHook{}

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

// NewMemLogger creates a Logrus logger mocked by SimpleLogrusHook.
func NewMemLogger(levels ...logrus.Level) (*logrus.Logger, *SimpleLogrusHook) {
	if len(levels) == 0 {
		levels = logrus.AllLevels
	}
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.Out = io.Discard
	hook := &SimpleLogrusHook{
		HookedLevels: levels,
	}
	logger.AddHook(hook)
	return logger, hook
}
