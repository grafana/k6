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
	"strconv"

	log "github.com/sirupsen/logrus"
	"github.com/dop251/goja"
)

type Console struct {
	Logger *log.Logger
}

func NewConsole() *Console {
	return &Console{log.StandardLogger()}
}

func (c Console) log(level log.Level, msgobj goja.Value, args ...goja.Value) {
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

func (c Console) Log(msg goja.Value, args ...goja.Value) {
	c.Info(msg, args...)
}

func (c Console) Debug(msg goja.Value, args ...goja.Value) {
	c.log(log.DebugLevel, msg, args...)
}

func (c Console) Info(msg goja.Value, args ...goja.Value) {
	c.log(log.InfoLevel, msg, args...)
}

func (c Console) Warn(msg goja.Value, args ...goja.Value) {
	c.log(log.WarnLevel, msg, args...)
}

func (c Console) Error(msg goja.Value, args ...goja.Value) {
	c.log(log.ErrorLevel, msg, args...)
}
