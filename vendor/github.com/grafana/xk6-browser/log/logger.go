package log

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type Logger struct {
	*logrus.Logger
	mu             sync.Mutex
	lastLogCall    int64
	iterID         string
	categoryFilter *regexp.Regexp
}

// NewNullLogger will create a logger where log lines will
// be discarded and not logged anywhere.
func NewNullLogger() *Logger {
	log := logrus.New()
	log.SetOutput(ioutil.Discard)
	return New(log, "")
}

// New creates a new logger.
func New(logger logrus.FieldLogger, iterID string) *Logger {
	ll := &Logger{
		Logger: logrus.New(),
		iterID: iterID,
	}

	if logger == nil {
		ll.Warnf("Logger", "no logger supplied, using default")
	} else if l, ok := logger.(*logrus.Logger); !ok {
		ll.Warnf("Logger", "invalid logger type %T, using default", logger)
	} else {
		ll.Logger = l
	}

	return ll
}

// Tracef logs a trace message.
func (l *Logger) Tracef(category string, msg string, args ...any) {
	l.Logf(logrus.TraceLevel, category, msg, args...)
}

// Debugf logs a debug message.
func (l *Logger) Debugf(category string, msg string, args ...any) {
	l.Logf(logrus.DebugLevel, category, msg, args...)
}

// Errorf logs an error message.
func (l *Logger) Errorf(category string, msg string, args ...any) {
	l.Logf(logrus.ErrorLevel, category, msg, args...)
}

// Infof logs an info message.
func (l *Logger) Infof(category string, msg string, args ...any) {
	l.Logf(logrus.InfoLevel, category, msg, args...)
}

// Warnf logs an warning message.
func (l *Logger) Warnf(category string, msg string, args ...any) {
	l.Logf(logrus.WarnLevel, category, msg, args...)
}

// Logf logs a message.
func (l *Logger) Logf(level logrus.Level, category string, msg string, args ...any) {
	if l == nil {
		return
	}
	// don't log if the current log level isn't in the required level.
	if l.GetLevel() < level {
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

	if l.categoryFilter != nil && !l.categoryFilter.MatchString(category) {
		return
	}
	fields := logrus.Fields{
		"source":   "browser",
		"category": category,
		"elapsed":  fmt.Sprintf("%d ms", elapsed),
	}
	if l.iterID != "" && l.GetLevel() > logrus.InfoLevel {
		fields["iteration_id"] = l.iterID
	}
	entry := l.WithFields(fields)
	if l.GetLevel() < level {
		entry.Printf(msg, args...)
		return
	}
	entry.Logf(level, msg, args...)
}

// SetLevel sets the logger level from a level string.
// Accepted values:
//   - "panic"
//   - "fatal"
//   - "error"
//   - "warn"
//   - "warning"
//   - "info"
//   - "debug"
//   - "trace"
func (l *Logger) SetLevel(level string) error {
	pl, err := logrus.ParseLevel(level)
	if err != nil {
		return err
	}
	l.Logger.SetLevel(pl)
	return nil
}

// DebugMode returns true if the logger level is set to Debug or higher.
func (l *Logger) DebugMode() bool {
	return l.GetLevel() >= logrus.DebugLevel
}

// ReportCaller adds source file and function names to the log entries.
func (l *Logger) ReportCaller() {
	caller := func() func(*runtime.Frame) (string, string) {
		return func(f *runtime.Frame) (function string, file string) {
			return f.Func.Name(), fmt.Sprintf("%s:%d", f.File, f.Line)
		}
	}
	l.SetFormatter(&logrus.TextFormatter{
		CallerPrettyfier: caller(),
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyFile: "caller",
		},
	})
	l.SetReportCaller(true)
}

// ConsoleLogFormatterSerializer creates a new logger that will
// correctly serialize RemoteObject instances.
func (l *Logger) ConsoleLogFormatterSerializer() *Logger {
	return &Logger{
		Logger: &logrus.Logger{
			Out:       l.Out,
			Level:     l.Level,
			Formatter: &consoleLogFormatter{l.Formatter},
			Hooks:     l.Hooks,
		},
	}
}

// SetCategoryFilter enables filtering logs by the filter regex.
func (l *Logger) SetCategoryFilter(filter string) (err error) {
	if filter == "" {
		return nil
	}
	if l.categoryFilter, err = regexp.Compile(filter); err != nil {
		return fmt.Errorf("invalid category filter %q: %w", filter, err)
	}
	return nil
}

type consoleLogFormatter struct {
	logrus.Formatter
}

// Format assembles a message from marshalling elements in the "objects" field
// to JSON separated by space, and deletes the field when done.
func (f *consoleLogFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	if objects, ok := entry.Data["objects"].([]any); ok {
		var msg []string
		for _, obj := range objects {
			// TODO: Log error?
			if o, err := json.Marshal(obj); err == nil {
				msg = append(msg, string(o))
			}
		}
		entry.Message = strings.Join(msg, " ")
		delete(entry.Data, "objects")
	}
	return f.Formatter.Format(entry)
}
