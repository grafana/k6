// Copyright 2020-2024 Buf Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package parser contains the logic for parsing protobuf source code into an
// AST (abstract syntax tree) and also for converting an AST into a descriptor
// proto.
//
// A FileDescriptorProto is very similar to an AST, but the AST this package
// uses is more useful because it contains more information about the source
// code, including details about whitespace and comments, that cannot be
// represented by a descriptor proto. This makes it ideal for things like
// code formatters, which may want to preserve things like whitespace and
// comment format.
package parser
