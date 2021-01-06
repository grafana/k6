/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2017 Load Impact
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

//go:generate rice embed-go

package lib

import (
	"sync"

	rice "github.com/GeertJohan/go.rice"
	"github.com/dop251/goja"
	"github.com/dop251/goja/parser"
)

//nolint:gochecknoglobals
var (
	once   sync.Once
	coreJs *goja.Program
)

func GetCoreJS() *goja.Program {
	once.Do(func() {
		program, err := goja.Parse(
			"core-js/shim.min.js",
			rice.MustFindBox("core-js").MustString("shim.min.js"),
			parser.WithDisableSourceMaps)
		if err != nil {
			panic(err)
		}
		coreJs, err = goja.CompileAST(program, false)
		if err != nil {
			panic(err)
		}
	})

	return coreJs
}
