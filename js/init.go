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
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/loadimpact/k6/stats"
	"github.com/robertkrimen/otto"
	"github.com/spf13/afero"
)

type InitAPI struct {
	r  *Runtime
	fs afero.Fs

	fileCache map[string]string
}

func (i *InitAPI) NewMetric(it int, name string, isTime bool) *stats.Metric {
	t := stats.MetricType(it)
	vt := stats.Default
	if isTime {
		vt = stats.Time
	}

	if m, ok := i.r.Metrics[name]; ok {
		if m.Type != t {
			throw(i.r.VM, errors.New(fmt.Sprintf("attempted to redeclare %s with a different type (%s != %s)", name, m.Type, t)))
			return nil
		}
		if m.Contains != vt {
			throw(i.r.VM, errors.New(fmt.Sprintf("attempted to redeclare %s with a different kind of value (%s != %s)", name, m.Contains, vt)))
		}
		return m
	}

	m := stats.New(name, t, vt)
	i.r.Metrics[name] = m
	return m
}

func (i *InitAPI) Require(name string) otto.Value {
	if !strings.HasPrefix(name, ".") {
		exports, err := i.r.loadLib(name + ".js")
		if err != nil {
			throw(i.r.VM, err)
		}
		return exports
	}

	exports, err := i.r.loadFile(name+".js", i.fs)
	if err != nil {
		throw(i.r.VM, err)
	}
	return exports
}

func (i *InitAPI) Open(name string) string {
	if i.fileCache == nil {
		i.fileCache = make(map[string]string)
	}

	path, err := filepath.Abs(name)
	if err != nil {
		throw(i.r.VM, err)
	}

	if data, ok := i.fileCache[path]; ok {
		return data
	}

	data, err := afero.ReadFile(i.fs, path)
	if err != nil {
		throw(i.r.VM, err)
	}

	s := string(data)
	i.fileCache[path] = s
	return s
}
