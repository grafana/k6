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
	"testing"

	"github.com/robertkrimen/otto"
)

func BenchmarkOttoRun(b *testing.B) {
	vm := otto.New()
	src := `1 + 1`

	b.Run("string", func(b *testing.B) {
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, err := vm.Run(src)
			if err != nil {
				b.Error(err)
				return
			}
		}
	})

	b.Run("*Script", func(b *testing.B) {
		script, err := vm.Compile("__snippet__", src)
		if err != nil {
			b.Error(err)
			return
		}
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			_, err := vm.Run(script)
			if err != nil {
				b.Error(err)
				return
			}
		}
	})
}
