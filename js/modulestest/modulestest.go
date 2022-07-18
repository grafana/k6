/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2021 Load Impact
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

package modulestest

import (
	"context"

	"github.com/dop251/goja"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/lib"
)

var _ modules.VU = &VU{}

// VU is a modules.VU implementation meant to be used within tests
type VU struct {
	CtxField              context.Context
	InitEnvField          *common.InitEnvironment
	StateField            *lib.State
	RuntimeField          *goja.Runtime
	RegisterCallbackField func() func(f func() error)
}

// Context returns internally set field to conform to modules.VU interface
func (m *VU) Context() context.Context {
	return m.CtxField
}

// InitEnv returns internally set field to conform to modules.VU interface
func (m *VU) InitEnv() *common.InitEnvironment {
	m.checkIntegrity()
	return m.InitEnvField
}

// State returns internally set field to conform to modules.VU interface
func (m *VU) State() *lib.State {
	m.checkIntegrity()
	return m.StateField
}

// Runtime returns internally set field to conform to modules.VU interface
func (m *VU) Runtime() *goja.Runtime {
	return m.RuntimeField
}

// RegisterCallback is not really implemented
func (m *VU) RegisterCallback() func(f func() error) {
	return m.RegisterCallbackField()
}

func (m *VU) checkIntegrity() {
	if m.InitEnvField != nil && m.StateField != nil {
		panic("there is a bug in the test: InitEnvField and StateField are not allowed at the same time")
	}
}
