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

package protocompile

import (
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/bufbuild/protocompile/internal/editions"
)

// IsEditionSupported returns true if this module can compile sources for
// the given edition. This returns true for the special EDITION_PROTO2 and
// EDITION_PROTO3 as well as all actual editions supported.
func IsEditionSupported(edition descriptorpb.Edition) bool {
	return edition == descriptorpb.Edition_EDITION_PROTO2 ||
		edition == descriptorpb.Edition_EDITION_PROTO3 ||
		(edition >= editions.MinSupportedEdition && edition <= editions.MaxSupportedEdition)
}
