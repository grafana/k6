package modulestest

// IMPORTANT: This file is a subset of the `js/console.go` file, as the original file doesn't expose
// the `console` struct implementation, which is needed for the `console.log` function to work in tests.

import (
	"encoding/json"
	"strings"

	"github.com/dop251/goja"
	"github.com/sirupsen/logrus"
)

type console struct {
	logger logrus.FieldLogger
}

// Creates a console with the standard logrus logger.
func newConsole(logger logrus.FieldLogger) *console {
	return &console{logger.WithField("source", "console")}
}

func (c console) Log(args ...goja.Value) {
	c.Info(args...)
}

func (c console) Info(args ...goja.Value) {
	c.log(logrus.InfoLevel, args...)
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
