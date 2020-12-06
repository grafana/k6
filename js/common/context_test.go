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

package common

import (
	"context"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
)

func TestContextRuntime(t *testing.T) {
	rt := goja.New()
	assert.Equal(t, rt, GetRuntime(WithRuntime(context.Background(), rt)))
}

func TestContextRuntimeNil(t *testing.T) {
	assert.Nil(t, GetRuntime(context.Background()))
}

func TestContextInitEnv(t *testing.T) {
	ie := &InitEnvironment{}
	assert.Nil(t, GetInitEnv(context.Background()))
	assert.Equal(t, ie, GetInitEnv(WithInitEnv(context.Background(), ie)))
}
