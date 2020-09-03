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
	"context"
	"fmt"
	"strings"

	"github.com/dop251/goja"
	"github.com/golang/protobuf/proto"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	"google.golang.org/grpc"

	"github.com/loadimpact/k6/js/common"
)

// GRPC represents the gRPC protocol module for k6
type GRPC struct {
	fds  []*desc.FileDescriptor
	conn *grpc.ClientConn
}

// New creates a new gRPC module
func New() *GRPC {
	return &GRPC{}
}

// Load will parse the given proto files and add their File Descriptors to the available list
func (g *GRPC) Load(importPaths []string, filenames ...string) error {
	var err error
	if filenames, err = protoparse.ResolveFilenames(importPaths, filenames...); err != nil {
		return err
	}
	parser := protoparse.Parser{
		ImportPaths:      importPaths,
		InferImportPaths: len(importPaths) == 0,
	}
	if g.fds, err = parser.ParseFiles(filenames...); err != nil {
		return err
	}

	return nil
}

func (g *GRPC) Connect(addr string) error {
	// TODO: Create a true block dial so we can report on any errors
	var err error
	if g.conn, err = grpc.Dial(addr, grpc.WithInsecure()); err != nil {
		return err
	}
	return nil
}

func (g *GRPC) InvokeRPC(ctx context.Context, method string, req goja.Value) (proto.Message, error) {
	// TODO: [RC] deal with base cases
	parts := strings.Split(method, "/")

	var md *desc.MethodDescriptor
	for _, fd := range g.fds {
		s := fd.FindService(parts[0])
		if s == nil {
			continue
		}
		md = s.FindMethodByName(parts[1])
		if md != nil {
			break
		}
	}

	if md == nil {
		return nil, fmt.Errorf("Method %q not found in file descriptors", method)
	}

	dm := dynamic.NewMessage(md.GetInputType())
	s := grpcdynamic.NewStub(g.conn)

	rt := common.GetRuntime(ctx)
	b, _ := req.ToObject(rt).MarshalJSON()
	dm.UnmarshalJSON(b)

	resp, err := s.InvokeRpc(ctx, md, dm)

	return resp, err
}
