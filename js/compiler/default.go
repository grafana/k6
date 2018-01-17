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

package compiler

import (
	"github.com/dop251/goja"
)

func init() {
	c, err := New()
	if err != nil {
		panic(err)
	}
	DefaultCompiler = c
}

var DefaultCompiler *Compiler

func Transform(src, filename string) (code string, srcmap SourceMap, err error) {
	return DefaultCompiler.Transform(src, filename)
}

func Compile(src, filename string, pre, post string, strict bool) (*goja.Program, string, error) {
	return DefaultCompiler.Compile(src, filename, pre, post, strict)
}
