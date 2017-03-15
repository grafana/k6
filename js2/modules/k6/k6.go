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

package k6

import (
	"context"

	log "github.com/Sirupsen/logrus"
	"github.com/dop251/goja"
)

type K6 struct{}

func (k6 *K6) Group(ctx context.Context, name string, fn goja.Callable) goja.Value {
	log.WithField("name", name).Info("running group")
	val, err := fn(goja.Undefined())
	if err != nil {
		panic(err)
	}
	return val
}

func (K6 K6) TestFn() {
	log.Info("aaaa")
}
