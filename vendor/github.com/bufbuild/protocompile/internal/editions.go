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

package internal

import "google.golang.org/protobuf/types/descriptorpb"

// AllowEditions is set to true in tests to enable editions syntax for testing.
// This will be removed and editions will be allowed by non-test code once the
// implementation is complete.
var AllowEditions = false

// SupportedEditions is the exhaustive set of editions that protocompile
// can support. We don't allow it to compile future/unknown editions, to
// make sure we don't generate incorrect descriptors, in the event that
// a future edition introduces a change or new feature that requires
// new logic in the compiler.
var SupportedEditions = map[string]descriptorpb.Edition{
	"2023": descriptorpb.Edition_EDITION_2023,
}
