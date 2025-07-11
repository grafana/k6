package js

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/grafana/sobek"
	"github.com/sirupsen/logrus"
)

var consoleFormatSpecifiersRegex = regexp.MustCompile(`%(s|i|d|f)`)

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

func (c console) log(level logrus.Level, args ...sobek.Value) {
	var strs strings.Builder
	arr := make([]interface{}, 0)
	if len(args) > 0 { //nolint:nestif
		fmtStr := c.valueString(args[0])
		strs.WriteString(fmtStr)
		argsLen := len(args[1:])
		if argsLen > 0 {
			matches := consoleFormatSpecifiersRegex.FindAllString(fmtStr, -1)
			loopLen := len(matches)
			hasMatches := loopLen > 0
			if loopLen == 0 {
				loopLen = argsLen
			} else {
				arr = make([]interface{}, loopLen)
			}

			for i := 0; i < loopLen; i++ {
				fmtSpecifier := ""
				if hasMatches {
					fmtSpecifier = matches[i]
				}

				switch fmtSpecifier {
				case "%d":
					arr[i] = args[i+1].ToInteger()
				case "%f":
					arr[i] = args[i+1].ToFloat()
				case "%s":
					arr[i] = args[i+1].String()
				default:
					strs.WriteString(" ")
					strs.WriteString(c.valueString(args[i+1]))
				}
			}
		}
	}
	msg := fmt.Sprintf(strs.String(), arr...)

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

func (c console) Log(args ...sobek.Value) {
	c.Info(args...)
}

func (c console) Debug(args ...sobek.Value) {
	c.log(logrus.DebugLevel, args...)
}

func (c console) Info(args ...sobek.Value) {
	c.log(logrus.InfoLevel, args...)
}

func (c console) Warn(args ...sobek.Value) {
	c.log(logrus.WarnLevel, args...)
}

func (c console) Error(args ...sobek.Value) {
	c.log(logrus.ErrorLevel, args...)
}

func (c console) valueString(v sobek.Value) string {
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
