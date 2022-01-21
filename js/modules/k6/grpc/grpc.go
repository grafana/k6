/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2020 Load Impact
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

package grpc

import (
	"github.com/dop251/goja"
	"go.k6.io/k6/js/modules"
	"google.golang.org/grpc/codes"
)

type (
	// RootModule is the global module instance that will create module
	// instances for each VU.
	RootModule struct{}

	// ModuleInstance represents an instance of the GRPC module for every VU.
	ModuleInstance struct {
		vu      modules.VU
		exports map[string]interface{}
	}
)

var (
	_ modules.Module   = &RootModule{}
	_ modules.Instance = &ModuleInstance{}
)

// New returns a pointer to a new RootModule instance.
func New() *RootModule {
	return &RootModule{}
}

// NewModuleInstance implements the modules.Module interface to return
// a new instance for each VU.
func (*RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	mi := &ModuleInstance{
		vu:      vu,
		exports: make(map[string]interface{}),
	}

	mi.exports["Client"] = mi.NewClient
	mi.defineConstants()
	return mi
}

// NewClient is the JS constructor for the grpc Client.
func (mi *ModuleInstance) NewClient(call goja.ConstructorCall) *goja.Object {
	rt := mi.vu.Runtime()
	return rt.ToValue(&Client{vu: mi.vu}).ToObject(rt)
}

// defineConstants defines the constant variables of the module.
func (mi *ModuleInstance) defineConstants() {
	rt := mi.vu.Runtime()
	mustAddCode := func(name string, code codes.Code) {
		mi.exports[name] = rt.ToValue(code)
	}

	mustAddCode("StatusOK", codes.OK)
	mustAddCode("StatusCanceled", codes.Canceled)
	mustAddCode("StatusUnknown", codes.Unknown)
	mustAddCode("StatusInvalidArgument", codes.InvalidArgument)
	mustAddCode("StatusDeadlineExceeded", codes.DeadlineExceeded)
	mustAddCode("StatusNotFound", codes.NotFound)
	mustAddCode("StatusAlreadyExists", codes.AlreadyExists)
	mustAddCode("StatusPermissionDenied", codes.PermissionDenied)
	mustAddCode("StatusResourceExhausted", codes.ResourceExhausted)
	mustAddCode("StatusFailedPrecondition", codes.FailedPrecondition)
	mustAddCode("StatusAborted", codes.Aborted)
	mustAddCode("StatusOutOfRange", codes.OutOfRange)
	mustAddCode("StatusUnimplemented", codes.Unimplemented)
	mustAddCode("StatusInternal", codes.Internal)
	mustAddCode("StatusUnavailable", codes.Unavailable)
	mustAddCode("StatusDataLoss", codes.DataLoss)
	mustAddCode("StatusUnauthenticated", codes.Unauthenticated)
}

// Exports returns the exports of the grpc module.
func (mi *ModuleInstance) Exports() modules.Exports {
	return modules.Exports{
		Named: mi.exports,
	}
}
