/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
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

package js

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"

	"github.com/dop251/goja"
	"github.com/sirupsen/logrus"
)

// console represents a JS console implemented as a logrus.Logger.
type console struct {
	logger logrus.FieldLogger
}

// Creates a console with the standard logrus logger.
func newConsole(logger logrus.FieldLogger) *console {
	return &console{logger.WithField("source", "console")}
}

// Creates a console logger with its output set to the file at the provided `filepath`.
func newFileConsole(filepath string, formatter logrus.Formatter) (*console, error) {
	f, err := os.OpenFile(filepath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644) //nolint:gosec
	if err != nil {
		return nil, err
	}

	l := logrus.New()
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
	exptype := v.ExportType()
	if exptype == nil {
		return v.String()
	}

	kind := exptype.Kind()
	if kind != reflect.Map && kind != reflect.Slice {
		return v.String()
	}

	b, err := json.Marshal(v)
	if err != nil {
		return v.String()
	}
	return string(b)
}
