// Copyright 2020-2023 Buf Technologies, Inc.
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

import (
	"bytes"
	"unicode"
	"unicode/utf8"
)

// JSONName returns the default JSON name for a field with the given name.
// This mirrors the algorithm in protoc:
//
//	https://github.com/protocolbuffers/protobuf/blob/v21.3/src/google/protobuf/descriptor.cc#L95
func JSONName(name string) string {
	var js []rune
	nextUpper := false
	for _, r := range name {
		if r == '_' {
			nextUpper = true
			continue
		}
		if nextUpper {
			nextUpper = false
			js = append(js, unicode.ToUpper(r))
		} else {
			js = append(js, r)
		}
	}
	return string(js)
}

// InitCap returns the given field name, but with the first letter capitalized.
func InitCap(name string) string {
	r, sz := utf8.DecodeRuneInString(name)
	return string(unicode.ToUpper(r)) + name[sz:]
}

// CreatePrefixList returns a list of package prefixes to search when resolving
// a symbol name. If the given package is blank, it returns only the empty
// string. If the given package contains only one token, e.g. "foo", it returns
// that token and the empty string, e.g. ["foo", ""]. Otherwise, it returns
// successively shorter prefixes of the package and then the empty string. For
// example, for a package named "foo.bar.baz" it will return the following list:
//
//	["foo.bar.baz", "foo.bar", "foo", ""]
func CreatePrefixList(pkg string) []string {
	if pkg == "" {
		return []string{""}
	}

	numDots := 0
	// one pass to pre-allocate the returned slice
	for i := 0; i < len(pkg); i++ {
		if pkg[i] == '.' {
			numDots++
		}
	}
	if numDots == 0 {
		return []string{pkg, ""}
	}

	prefixes := make([]string, numDots+2)
	// second pass to fill in returned slice
	for i := 0; i < len(pkg); i++ {
		if pkg[i] == '.' {
			prefixes[numDots] = pkg[:i]
			numDots--
		}
	}
	prefixes[0] = pkg

	return prefixes
}

func WriteEscapedBytes(buf *bytes.Buffer, b []byte) {
	// This uses the same algorithm as the protoc C++ code for escaping strings.
	// The protoc C++ code in turn uses the abseil C++ library's CEscape function:
	//  https://github.com/abseil/abseil-cpp/blob/934f613818ffcb26c942dff4a80be9a4031c662c/absl/strings/escaping.cc#L406
	for _, c := range b {
		switch c {
		case '\n':
			buf.WriteString("\\n")
		case '\r':
			buf.WriteString("\\r")
		case '\t':
			buf.WriteString("\\t")
		case '"':
			buf.WriteString("\\\"")
		case '\'':
			buf.WriteString("\\'")
		case '\\':
			buf.WriteString("\\\\")
		default:
			if c >= 0x20 && c < 0x7f {
				// simple printable characters
				buf.WriteByte(c)
			} else {
				// use octal escape for all other values
				buf.WriteRune('\\')
				buf.WriteByte('0' + ((c >> 6) & 0x7))
				buf.WriteByte('0' + ((c >> 3) & 0x7))
				buf.WriteByte('0' + (c & 0x7))
			}
		}
	}
}
