package js

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/dop251/goja"
	"github.com/sirupsen/logrus"
)

// console represents a JS console implemented as a logrus.FieldLogger.
type console struct {
	logger logrus.FieldLogger
}

// Creates a console with the standard logrus logger.
func newConsole(logger logrus.FieldLogger) *console {
	return &console{logger.WithField("source", "console")}
}

// Creates a console logger with its output set to the file at the provided `filepath`.
func newFileConsole(filepath string, formatter logrus.Formatter, level logrus.Level) (*console, error) {
	//nolint:gosec,forbidigo // see https://github.com/grafana/k6/issues/2565
	f, err := os.OpenFile(filepath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return nil, err
	}

	l := logrus.New()
	l.SetLevel(level)
	l.SetOutput(f)
	l.SetFormatter(formatter)

	return &console{l}, nil
}

func (c console) log(level logrus.Level, args ...goja.Value) {
	var strs strings.Builder
	for i := 0; i < len(args); i++ {
		if i > 0 {
			strs.WriteString(" ")
		}
		strs.WriteString(c.valueString(args[i]))
	}
	msg := strs.String()

	switch level { //nolint:exhaustive
	case logrus.DebugLevel:
		c.logger.Debug(msg)
	case logrus.InfoLevel:
		c.logger.Info(msg)
	case logrus.WarnLevel:
		c.logger.Warn(msg)
	case logrus.ErrorLevel:
		c.logger.Error(msg)
	}
}

func (c console) Log(args ...goja.Value) {
	c.Info(args...)
}

func (c console) Debug(args ...goja.Value) {
	c.log(logrus.DebugLevel, args...)
}

func (c console) Info(args ...goja.Value) {
	c.log(logrus.InfoLevel, args...)
}

func (c console) Warn(args ...goja.Value) {
	c.log(logrus.WarnLevel, args...)
}

func (c console) Error(args ...goja.Value) {
	c.log(logrus.ErrorLevel, args...)
}

func (c console) valueString(v goja.Value) string {
	mv, ok := v.(json.Marshaler)
	if !ok {
		return v.String()
	}

	b, err := json.Marshal(mv)
	if err != nil {
		return v.String()
	}
	return string(b)
}
