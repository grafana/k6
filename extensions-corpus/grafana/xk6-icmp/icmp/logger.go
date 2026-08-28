package icmp

import (
	"fmt"
	"log/slog"
)

// extensionLogger keeps the extension's existing contextual logging style
// while depending only on the standalone slog capability.
type extensionLogger struct{ logger *slog.Logger }

func newLogger(logger *slog.Logger) extensionLogger { return extensionLogger{logger: logger} }

func (l extensionLogger) WithError(err error) extensionLogger {
	return newLogger(l.logger.With("error", err))
}

func (l extensionLogger) WithField(key string, value any) extensionLogger {
	return newLogger(l.logger.With(key, value))
}

func (l extensionLogger) Debug(message string) { l.logger.Debug(message) }
func (l extensionLogger) Warn(message string)  { l.logger.Warn(message) }
func (l extensionLogger) Error(message string) { l.logger.Error(message) }

func (l extensionLogger) Warnf(format string, args ...any) {
	l.logger.Warn(fmt.Sprintf(format, args...))
}
