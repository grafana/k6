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

package k8s

import (
	"fmt"
	"github.com/dop251/goja"
	"github.com/loadimpact/k6/js/common"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestFail(t *testing.T) {
	rt := goja.New()
	rt.Set("k8s", common.Bind(rt, New(), nil))
	_, err := common.RunString(rt, `k8s.fail("k8s has loaded successfully")`)
	assert.Contains(t, err.Error(), "GoError: k8s has loaded successfully")
}

func TestList(t *testing.T) {
	rt := goja.New()
	rt.Set("k8s", common.Bind(rt, New(), nil))
	val, err := common.RunString(rt, `k8s.list("default")`)
	assert.Nil(t, err)
	fmt.Println(len(val.String()) > 0)
}
