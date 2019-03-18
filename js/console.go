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
	"context"
	"os"
	"strconv"

	"github.com/dop251/goja"
	log "github.com/sirupsen/logrus"
)

// console represents a JS console implemented as a logrus.Logger.
type console struct {
	Logger *log.Logger
}

// Creates a console with the standard logrus logger.
func newConsole() *console {
	return &console{log.StandardLogger()}
}

// Creates a console logger with its output set to the file at the provided `filepath`.
func newFileConsole(filepath string) (*console, error) {
	f, err := os.OpenFile(filepath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		return nil, err
	}

	l := log.New()
	l.SetOutput(f)

	//TODO: refactor to not rely on global variables, albeit external ones
	l.SetFormatter(log.StandardLogger().Formatter)

	return &console{l}, nil
}

func (c console) log(ctx *context.Context, level log.Level, msgobj goja.Value, args ...goja.Value) {
	if ctx != nil && *ctx != nil {
		select {
		case <-(*ctx).Done():
			return
		default:
		}
	}

	fields := make(log.Fields)
	for i, arg := range args {
		fields[strconv.Itoa(i)] = arg.String()
	}
	msg := msgobj.ToString()
	e := c.Logger.WithFields(fields)
	switch level {
	case log.DebugLevel:
		e.Debug(msg)
	case log.InfoLevel:
		e.Info(msg)
	case log.WarnLevel:
		e.Warn(msg)
	case log.ErrorLevel:
		e.Error(msg)
	}
}

func (c console) Log(ctx *context.Context, msg goja.Value, args ...goja.Value) {
	c.Info(ctx, msg, args...)
}

func (c console) Debug(ctx *context.Context, msg goja.Value, args ...goja.Value) {
	c.log(ctx, log.DebugLevel, msg, args...)
}

func (c console) Info(ctx *context.Context, msg goja.Value, args ...goja.Value) {
	c.log(ctx, log.InfoLevel, msg, args...)
}

func (c console) Warn(ctx *context.Context, msg goja.Value, args ...goja.Value) {
	c.log(ctx, log.WarnLevel, msg, args...)
}

func (c console) Error(ctx *context.Context, msg goja.Value, args ...goja.Value) {
	c.log(ctx, log.ErrorLevel, msg, args...)
}
